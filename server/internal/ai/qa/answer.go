package qa

import (
	"context"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/llm/protocol"
)

func Answer(ctx context.Context, client *llm.Client, question, material, memory string) (protocol.Response, error) {
	system := "只能根据给定材料回答；每项事实用 [块ID] 标注引用。\n" + memory + "\n材料：\n" + material
	return client.Complete(ctx, "qa-answer", []protocol.Message{{Role: "system", Content: system}, {Role: "user", Content: question}})
}
