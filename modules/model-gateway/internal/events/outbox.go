package events

import (
	"context"
	"nice-wiki/libs/platform"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

type OperationContext = platform.OperationContext
type PublishOutboxInput struct {
	Limit    int    `json:"limit"`
	WorkerID string `json:"workerId"`
}
type PublishOutboxResult struct {
	Published int `json:"published"`
	Pending   int `json:"pending"`
}

func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error) {
	if op.CallerService != "model-gateway" || op.Actor.Kind != "service" || op.Actor.SubjectID != "model-gateway" {
		return PublishOutboxResult{}, platform.NewError(platform.CodeForbidden, false, "模型outbox只能由本域后台转发")
	}
	result, err := gatewayruntime.RelayOutbox(ctx, op, input.Limit)
	if err != nil {
		return PublishOutboxResult{}, err
	}
	return PublishOutboxResult{Published: int(result.Relayed), Pending: int(result.Pending)}, nil
}
