package content

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/content/internal/gen"
)

type RelayOutboxInput struct {
	BatchLimit int `json:"batchLimit"`
}
type RelayOutboxResult = platform.RelayBatchResult

// RelayRuntime仅由本域后台入口装配；不接受HTTP请求触发。
type RelayRuntime struct {
	Store       *Store
	Jobs        *gen.JobsClient
	WriteWindow platform.WriteWindowConfig
}
type relayRuntimeKey struct{}

func WithRelayRuntime(ctx context.Context, runtime RelayRuntime) context.Context {
	return context.WithValue(ctx, relayRuntimeKey{}, runtime)
}

// RelayOutbox只装配真实本域数据库和版本化jobs客户端，机制统一由平台执行。
func RelayOutbox(ctx context.Context, op OperationContext, input RelayOutboxInput) (RelayOutboxResult, error) {
	if op.CallerService != "content" || op.Actor.Kind != "service" || op.Actor.SubjectID != "content" {
		return RelayOutboxResult{}, platform.NewError(platform.CodeForbidden, false, "事件转发仅允许本域后台服务")
	}
	runtime, ok := ctx.Value(relayRuntimeKey{}).(RelayRuntime)
	if !ok || runtime.Store == nil || runtime.Jobs == nil {
		return RelayOutboxResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "文档事件转发依赖未装配")
	}
	if err := runtime.Store.checkRevisionScope(ctx, op); err != nil {
		return RelayOutboxResult{}, err
	}
	return outbox.RunRelayBatch(platform.WithOperationContext(ctx, op), outbox.NewStore(runtime.Store.db), outbox.NewEventPublisher(runtime.Jobs), platform.RelayConfig{BatchLimit: input.BatchLimit, PollMillis: 1000, Runtime: runtime.WriteWindow.Runtime, WriteWindow: runtime.WriteWindow.WriteWindow})
}
