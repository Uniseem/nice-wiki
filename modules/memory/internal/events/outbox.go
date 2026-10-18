// Package events 只装配本域事件，不复制平台 outbox/inbox 机制。
package events

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/memory/internal/runtime"
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
	batch, err := runtime.RelayBatch(ctx, input.Limit)
	if err != nil {
		return PublishOutboxResult{}, err
	}
	return PublishOutboxResult{Published: batch.Relayed, Pending: batch.Pending}, nil
}
