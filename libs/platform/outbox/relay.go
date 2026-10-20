package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
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
	if config.PollMillis < minPollMillis || config.PollMillis > maxPollMillis {
		return platform.NewError(platform.CodeValidation, false, "outbox轮询间隔不合法")
	}
	poll := time.Duration(config.PollMillis) * time.Millisecond
	for {
		if ctx.Err() != nil {
			return nil
		}
		_, err := RunRelayBatch(ctx, store, client, config)
		var failure *platform.Error
		if errors.As(err, &failure) && failure.Code == platform.CodeValidation {
			return err
		}
		if !sleepContext(ctx, poll) {
			return nil
		}
	}
}

// relayWriteContext 每次IO前读取最新运行权威和control窗口；未知不得改active。
// 后台控制请求设置本次IO截止，保留父ctx的更早取消及原费用根。
func relayWriteContext(ctx context.Context, config platform.RelayConfig) (context.Context, platform.RuntimeView, context.CancelFunc, error) {
	return lifecycle.NewWriteContext(ctx, platform.WriteWindowConfig{Runtime: config.Runtime, WriteWindow: config.WriteWindow})
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
	if op.RootOperationID != event.EventID {
		return platform.NewError(platform.CodeIntegrityError, false, "事件转发必须使用已绑定的event控制根")
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

// RunRelayBatch领取并处理一批已提交事件；每次写入都重新取得安全窗口。
func RunRelayBatch(ctx context.Context, store platform.OutboxStore, client platform.EventPublisher, config platform.RelayConfig) (platform.RelayBatchResult, error) {
	// 未接入快照屏障的服务可以不提供准入与逐事件作用域，此时直接放行。
	if config.AdmitBatch == nil {
		config.AdmitBatch = func(ctx context.Context) (context.Context, func(), error) { return ctx, func() {}, nil }
	}
	if config.BindEventScope == nil {
		// 每个事件以自己的 eventId 作操作根，jobs 才能按事件幂等接收。
		config.BindEventScope = func(ctx context.Context, row platform.OutboxRow) (context.Context, func(), error) {
			if op, ok := platform.OperationContextFrom(ctx); ok {
				op.RootOperationID = row.EventID
				op.OperationID = row.EventID
				ctx = platform.WithOperationContext(ctx, op)
			}
			return ctx, func() {}, nil
		}
	}
	if config.BatchLimit < 1 || config.BatchLimit > maxBatchLimit || config.Runtime == nil || config.WriteWindow == nil {
		return platform.RelayBatchResult{}, platform.NewError(platform.CodeValidation, false, "outbox转发参数或写窗口未装配")
	}
	scoped, releaseScope, scopeErr := config.AdmitBatch(ctx)
	if scopeErr != nil {
		return platform.RelayBatchResult{}, scopeErr
	}
	defer releaseScope()
	ctx = scoped
	work, view, cancel, err := relayWriteContext(ctx, config)
	if err != nil {
		return platform.RelayBatchResult{}, err
	}
	_, err = store.DiscardOtherEpochs(work, view.DataEpoch)
	cancel()
	if err != nil {
		return platform.RelayBatchResult{}, err
	}
	work, _, cancel, err = relayWriteContext(ctx, config)
	if err != nil {
		return platform.RelayBatchResult{}, err
	}
	rows, err := store.ClaimBatch(work, config.BatchLimit, rowLease)
	cancel()
	if err != nil {
		return platform.RelayBatchResult{}, err
	}
	result := platform.RelayBatchResult{Pending: int64(len(rows))}
	for _, row := range rows {
		if err := func() error {
			eventContext, releaseEvent, eventErr := config.BindEventScope(ctx, row)
			if eventErr != nil {
				return eventErr
			}
			defer releaseEvent()
			ctx := eventContext

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
				result.Pending--
				return nil
			}
			publishErr := client.PublishEvent(work, envelopeOf(row))
			if publishErr != nil && os.Getenv("NW_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "事件转发失败 type=%s：%v\n", row.EventType, publishErr)
			}
			cancel()
			// 网络调用后重新核对窗口，不能在安全截止后写确认或重试状态。
			work, view, cancel, err = relayWriteContext(ctx, config)
			if err != nil {
				return err
			}
			if row.DataEpoch != view.DataEpoch {
				err = store.Discard(work, row.EventID, view.DataEpoch)
				result.Pending--
			} else {
				switch {
				case publishErr == nil:
					err = store.MarkRelayed(work, []platform.Identifier{row.EventID})
					result.Relayed++
					result.Pending--
				case isQuarantineable(publishErr):
					err = store.Quarantine(work, row.EventID, quarantineNote)
					result.Quarantined++
					result.Pending--
				default:
					err = store.ReleaseForRetry(work, row.EventID, row.Attempts+1, backoff(row.Attempts+1))
				}
			}
			cancel()
			if err != nil {
				return err
			}
			return nil
		}(); err != nil {
			return platform.RelayBatchResult{}, err
		}
	}
	return result, nil
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
