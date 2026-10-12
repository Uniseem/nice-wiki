package ingest

import (
	"context"
	"errors"
)

type DescriptionStore interface {
	GetDescription(context.Context, string) (string, bool, error)
	PutDescription(context.Context, string, string) error
}
type DescriptionModel interface {
	Describe(context.Context, string, string) (string, error)
}

func Describe(ctx context.Context, store DescriptionStore, model DescriptionModel, hash, kind string) (string, error) {
	if value, ok, err := store.GetDescription(ctx, hash); err != nil {
		return "", err
	} else if ok {
		return value, nil
	}
	if model == nil {
		return "", errors.New("视觉描述模型尚未配置")
	}
	value, err := model.Describe(ctx, hash, kind)
	if err != nil {
		return "", err
	}
	if err := store.PutDescription(ctx, hash, value); err != nil {
		return "", err
	}
	return value, nil
}
