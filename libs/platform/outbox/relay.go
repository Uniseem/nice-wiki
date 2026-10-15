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
	if config.BatchLimit < 1 || config.BatchLimit > maxBatchLimit {
		return platform.NewError(platform.CodeValidation, false, "batchLimit必须在1到100之间")
	}
	if config.PollMillis < minPollMillis || config.PollMillis > maxPollMillis {
		return platform.NewError(platform.CodeValidation, false, "pollMillis超出允许范围")
	}
	poll := time.Duration(config.PollMillis) * time.Millisecond
	var lastEpoch platform.Identifier
	for {
		if ctx.Err() != nil {
			return nil
		}
		runtime, ok := readRuntime(ctx, config.Runtime)
		if ok && runtime.DataEpoch != "" {
			if lastEpoch != "" && runtime.DataEpoch != lastEpoch {
				// 恢复换入后丢弃旧代pending行，防止旧事件写入新数据；
				// 失败则不推进lastEpoch，下个周期重试。
				if _, err := store.DiscardOtherEpochs(ctx, runtime.DataEpoch); err == nil {
					lastEpoch = runtime.DataEpoch
				}
			} else if lastEpoch == "" {
				lastEpoch = runtime.DataEpoch
			}
		}
		rows, err := store.ClaimBatch(ctx, config.BatchLimit, rowLease)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if !sleepContext(ctx, poll) {
				return nil
			}
			continue
		}
		if len(rows) == 0 {
			if !sleepContext(ctx, poll) {
				return nil
			}
			continue
		}
		if err := relayBatch(ctx, store, client, rows, runtime); err != nil && ctx.Err() != nil {
			return nil
		}
	}
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
func relayBatch(ctx context.Context, store platform.OutboxStore, client platform.EventPublisher, rows []platform.OutboxRow, runtime platform.RuntimeView) error {
	var relayed []platform.Identifier
	for _, row := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if runtime.DataEpoch != "" && row.DataEpoch != runtime.DataEpoch {
			_ = store.Discard(ctx, row.EventID, runtime.DataEpoch)
			continue
		}
		err := client.PublishEvent(ctx, envelopeOf(row))
		switch {
		case err == nil:
			relayed = append(relayed, row.EventID)
		case isQuarantineable(err):
			_ = store.Quarantine(ctx, row.EventID, quarantineNote)
		default:
			// jobs不可用或暂时失败：按1/2/4秒退避（上限60秒），不丢outbox。
			_ = store.ReleaseForRetry(ctx, row.EventID, row.Attempts+1, backoff(row.Attempts+1))
		}
	}
	return store.MarkRelayed(ctx, relayed)
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

// readRuntime 读取运行状态；未提供或读取失败按running处理。
func readRuntime(ctx context.Context, provider func(context.Context) (platform.RuntimeView, error)) (platform.RuntimeView, bool) {
	if provider == nil {
		return platform.RuntimeView{Phase: platform.PhaseRunning}, false
	}
	view, err := provider(ctx)
	if err != nil {
		return platform.RuntimeView{Phase: platform.PhaseRunning}, false
	}
	return view, true
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
