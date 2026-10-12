package ingest

import (
	"context"
	"nice-wiki/internal/ai/retrieve"
)

type ContextModel interface {
	Contextualize(context.Context, retrieve.Unit) (string, error)
}

func Contextualize(ctx context.Context, model ContextModel, values []retrieve.Unit) ([]retrieve.Unit, error) {
	result := append([]retrieve.Unit{}, values...)
	for index := range result {
		prefix, err := model.Contextualize(ctx, result[index])
		if err != nil {
			return nil, err
		}
		if prefix != "" {
			result[index].Text = prefix + "\n" + result[index].Text
		}
	}
	return result, nil
}
