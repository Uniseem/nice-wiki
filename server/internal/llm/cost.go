package llm

import (
	"context"
	"nice-wiki/internal/llm/protocol"
)

type spendingKey struct{}
type Spending func(float64) (func(float64), error)

func WithSpending(ctx context.Context, spend Spending) context.Context {
	return context.WithValue(ctx, spendingKey{}, spend)
}
func Reserve(ctx context.Context, amount float64) (func(float64), error) {
	if spend, ok := ctx.Value(spendingKey{}).(Spending); ok {
		return spend(amount)
	}
	return func(float64) {}, nil
}

type outputKey struct{}

func WithOutputLimit(ctx context.Context, limit int) context.Context {
	return context.WithValue(ctx, outputKey{}, limit)
}

func Cost(model Model, usage protocol.Usage) float64 {
	return float64(usage.InputTokens)/1_000_000*model.InputPrice + float64(usage.OutputTokens)/1_000_000*model.OutputPrice
}
