package qa

import (
	"context"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
	"nice-wiki/internal/prompts"
)

func Answer(ctx context.Context, client *llm.Client, question, material, memory string) (protocol.Response, error) {
	messages, err := answerMessages(question, material, memory)
	if err != nil {
		return protocol.Response{}, err
	}
	return client.Complete(ctx, "qa-answer", messages)
}
func AnswerStream(ctx context.Context, client *llm.Client, question, material, memory string, emit func(string) error) (protocol.Response, error) {
	messages, err := answerMessages(question, material, memory)
	if err != nil {
		return protocol.Response{}, err
	}
	return client.Stream(ctx, "qa-answer", messages, func(chunk protocol.Chunk) error {
		if chunk.Text != "" {
			return emit(chunk.Text)
		}
		return nil
	})
}
func answerMessages(question, material, memory string) ([]protocol.Message, error) {
	prompt, err := prompts.Get("answer")
	if err != nil {
		return nil, err
	}
	system := memory + "\n" + prompt + "\n材料：\n" + material
	return []protocol.Message{{Role: "system", Content: system}, {Role: "user", Content: question}}, nil
}
