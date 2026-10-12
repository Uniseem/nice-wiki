package ingest

import (
	"context"
	"nice-wiki/internal/ai/retrieve"
	"nice-wiki/internal/llm/protocol"
)

type VectorWriter interface {
	ReplaceVectors(context.Context, string, [][]float32) error
}

func Embed(ctx context.Context, model protocol.Embedder, writer VectorWriter, modelID string, units []retrieve.Unit) error {
	texts := make([]string, len(units))
	for i := range units {
		texts[i] = units[i].Text
	}
	vectors, _, err := model.Embed(ctx, modelID, texts)
	if err != nil {
		return err
	}
	for i := range units {
		if err := writer.ReplaceVectors(ctx, units[i].ID, [][]float32{vectors[i]}); err != nil {
			return err
		}
	}
	return nil
}
