package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
)

// 转发循环参数边界与退避上限。
const (
	maxBatchLimit  = 100
	minPollMillis  = 100
	maxPollMillis  = 10000
	rowLease       = 30 * time.Second
	backoffCap     = 60 * time.Second
	quarantineNote = "同eventId异hash或信封非法"
)

// RunRelay 是outbox转发循环的通用实现：
// 按提交顺序以30秒行租约领取pending行，逐条调用jobs:publishEvent，
// 确认后标relayed；失败按1/2/4秒退避（上限60秒）；同eventId异hash
// 或信封非法标quarantined；屏障draining时继续转发直到空；
// 恢复换入后丢弃旧dataEpoch行。ctx取消即退出。
func RunRelay(ctx context.Context, store platform.OutboxStore, client platform.EventPublisher, config platform.RelayConfig) error {
	if config.BatchLimit < 1 || config.BatchLimit > maxBatchLimit || config.PollMillis < minPollMillis || config.PollMillis > maxPollMillis || config.Runtime == nil || config.WriteWindow == nil {
		return platform.NewError(platform.CodeValidation, false, "outbox转发参数或写窗口未装配")
	}
	poll := time.Duration(config.PollMillis) * time.Millisecond
	var lastEpoch platform.Identifier
	for {
		if ctx.Err() != nil {
			return nil
		}
		work, view, cancel, err := relayWriteContext(ctx, config)
		if err != nil {
			if !sleepContext(ctx, poll) {
				return nil
			}
			continue
		}
		if lastEpoch != view.DataEpoch {
			if _, err = store.DiscardOtherEpochs(work, view.DataEpoch); err == nil {
				lastEpoch = view.DataEpoch
			}
		}
		var rows []platform.OutboxRow
		if err == nil {
			rows, err = store.ClaimBatch(work, config.BatchLimit, rowLease)
		}
		cancel()
		if err != nil || len(rows) == 0 {
			if !sleepContext(ctx, poll) {
				return nil
			}
			continue
		}
		if err := relayBatch(ctx, store, client, rows, config); err != nil {
			if !sleepContext(ctx, poll) {
				return nil
			}
		}
	}
}

// relayWriteContext 每次IO前读取最新运行权威和control窗口；未知不得改active。
// 后台控制请求设置本次IO截止，保留父ctx的更早取消及原费用根。
func relayWriteContext(ctx context.Context, config platform.RelayConfig) (context.Context, platform.RuntimeView, context.CancelFunc, error) {
	work, cancel := context.WithTimeout(ctx, 30*time.Second)
	if op, ok := platform.OperationContextFrom(work); ok {
		deadline, _ := work.Deadline()
		op.Deadline = platform.Timestamp(deadline.UTC().Format(time.RFC3339Nano))
		work = platform.WithOperationContext(work, op)
	}
	view, err := config.Runtime(work)
	if err != nil {
		cancel()
		return nil, view, nil, err
	}
	deadline, err := config.WriteWindow(work)
	if err != nil {
		cancel()
		return nil, view, nil, err
	}
	if view.DataEpoch == "" {
		cancel()
		return nil, view, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "outbox运行权威数据代次未知")
	}
	if view.Phase != platform.PhaseRunning && view.Phase != platform.PhaseBackupBarrier && view.Phase != platform.PhaseRestoreStaging {
		cancel()
		return nil, view, nil, platform.NewError(platform.CodeLocked, true, "当前恢复阶段不接受outbox写入")
	}
	if (view.Maintenance && deadline.IsZero()) || (!deadline.IsZero() && !deadline.After(time.Now())) {
		cancel()
		return nil, view, nil, platform.NewError(platform.CodeLocked, true, "outbox维护安全窗口已关闭")
	}
	if !deadline.IsZero() {
		bounded, release := context.WithDeadline(work, deadline)
		if op, ok := platform.OperationContextFrom(bounded); ok {
			effective, _ := bounded.Deadline()
			op.Deadline = platform.Timestamp(effective.UTC().Format(time.RFC3339Nano))
			bounded = platform.WithOperationContext(bounded, op)
		}
		return bounded, view, func() { release(); cancel() }, nil
	}
	return work, view, cancel, nil
}

// PublishClient 与生成的原始Jobs客户端同形，不要求虚假的租约方法。
type PublishClient interface {
	PublishEvent(context.Context, any, string) (json.RawMessage, error)
}

type eventPublisher struct{ client PublishClient }

// NewEventPublisher 只适配版本化publishEvent，使用eventId作为请求幂等键。
func NewEventPublisher(client PublishClient) platform.EventPublisher {
	return eventPublisher{client: client}
}

func (p eventPublisher) PublishEvent(ctx context.Context, event platform.EventEnvelope) error {
	op, ok := platform.OperationContextFrom(ctx)
	if !ok {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "事件转发缺少可信操作上下文")
	}
	if op.RootOperationID == "" || op.RootOperationID == op.OperationID {
		op.RootOperationID = event.EventID
	}
	op.OperationID = event.EventID
	op.DataEpoch = event.DataEpoch
	ctx = platform.WithOperationContext(ctx, op)
	raw, err := p.client.PublishEvent(ctx, struct {
		Event platform.EventEnvelope `json:"event"`
	}{event}, event.EventID)
	if err != nil {
		return err
	}
	var result struct {
		Accepted bool   `json:"accepted"`
		EventID  string `json:"eventId"`
	}
	if json.Unmarshal(raw, &result) != nil || !result.Accepted || result.EventID != event.EventID {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "事件接收回执与请求不一致")
	}
	return nil
}

// relayBatch 转发一批已领取行；返回值只表达批次级中断。
func relayBatch(ctx context.Context, store platform.OutboxStore, client platform.EventPublisher, rows []platform.OutboxRow, config platform.RelayConfig) error {
	for _, row := range rows {
		work, view, cancel, err := relayWriteContext(ctx, config)
		if err != nil {
			return err
		}
		if row.DataEpoch != view.DataEpoch {
			err = store.Discard(work, row.EventID, view.DataEpoch)
			cancel()
			if err != nil {
				return err
			}
			continue
		}
		publishErr := client.PublishEvent(work, envelopeOf(row))
		cancel()
		// 网络调用返回后重新核对窗口，不能在safe截止后写确认/重试状态。
		work, view, cancel, err = relayWriteContext(ctx, config)
		if err != nil {
			return err
		}
		if row.DataEpoch != view.DataEpoch {
			err = store.Discard(work, row.EventID, view.DataEpoch)
		} else {
			switch {
			case publishErr == nil:
				err = store.MarkRelayed(work, []platform.Identifier{row.EventID})
			case isQuarantineable(publishErr):
				err = store.Quarantine(work, row.EventID, quarantineNote)
			default:
				err = store.ReleaseForRetry(work, row.EventID, row.Attempts+1, backoff(row.Attempts+1))
			}
		}
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

// envelopeOf 从outbox行还原事件信封。
func envelopeOf(row platform.OutboxRow) platform.EventEnvelope {
	return platform.EventEnvelope{
		EventID:      row.EventID,
		EventType:    row.EventType,
		EventVersion: row.EventVersion,
		Producer:     row.Producer,
		SubjectID:    row.SubjectID,
		Revision:     row.Revision,
		OccurredAt:   row.OccurredAt,
		DataEpoch:    row.DataEpoch,
		Payload:      row.Payload,
	}
}

// isQuarantineable 同eventId异hash（CONFLICT）与信封非法（VALIDATION）
// 属于永久性问题，隔离不重试。
func isQuarantineable(err error) bool {
	var libErr *platform.Error
	if errors.As(err, &libErr) {
		return libErr.Code == platform.CodeConflict || libErr.Code == platform.CodeValidation
	}
	return false
}

// backoff 计算1/2/4秒…指数退避，上限60秒。
func backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	shift := attempts - 1
	if shift > 6 {
		shift = 6
	}
	delay := time.Second << uint(shift)
	if delay > backoffCap {
		delay = backoffCap
	}
	return delay
}

// sleepContext 等待d或ctx取消；返回false表示已取消。
func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
