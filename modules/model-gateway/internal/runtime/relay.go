package runtime

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
)

// RelayOutbox复用平台私有存储及jobs发布机制，载荷只能来自本域业务事务outbox。
func RelayOutbox(ctx context.Context, op OperationContext, limit int) (platform.RelayBatchResult, error) {
	store := CurrentStore()
	if store == nil {
		return platform.RelayBatchResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "模型事件私有存储未装配")
	}
	options, err := ExecutionDependencies()
	if err != nil {
		return platform.RelayBatchResult{}, err
	}
	if options.Jobs == nil {
		return platform.RelayBatchResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "jobs事件发布未装配")
	}
	service := op
	service.CallerService = "model-gateway"
	service.Actor = platform.AuthContext{Kind: "service", SubjectID: "model-gateway", SiteID: op.Actor.SiteID, Grants: []string{}, ExpiresAt: op.Deadline}
	return outbox.RunRelayBatch(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service), outbox.NewStore(store.db), outbox.NewEventPublisher(options.Jobs), platform.RelayConfig{BatchLimit: limit, PollMillis: 1000, Runtime: options.WriteWindow.Runtime, WriteWindow: options.WriteWindow.WriteWindow})
}
