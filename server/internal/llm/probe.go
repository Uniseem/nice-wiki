package llm

import (
	"context"
	"nice-wiki/internal/llm/protocol"
)

func Probe(ctx context.Context, chat protocol.Chat, model string) error {
	_, err := chat.Complete(ctx, protocol.Request{Model: model, Messages: []protocol.Message{{Role: "user", Content: "Reply with OK."}}, MaxTokens: 8})
	return err
}
