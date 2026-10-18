// Package events 装配本域引用事件与平台持久中转。
package events

import (
	"context"
	"nice-wiki/libs/platform"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

type OperationContext = platform.OperationContext
type PublishOutboxInput struct {
	Limit    int    `json:"limit"`
	WorkerId string `json:"workerId"`
}
type PublishOutboxResult struct {
	Published int64 `json:"published"`
	Pending   int64 `json:"pending"`
}

func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error) {
	if op.CallerService != "localization" || op.Actor.Kind != "service" || op.Actor.SubjectID != "localization" {
		return PublishOutboxResult{}, platform.NewError(platform.CodeForbidden, false, "本地化outbox只供受信后台调用")
	}
	result, err := localizationruntime.PublishBatch(ctx, input.Limit)
	if err != nil {
		return PublishOutboxResult{}, err
	}
	return PublishOutboxResult{Published: result.Relayed, Pending: result.Pending}, nil
}
