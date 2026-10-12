package ingest

import (
	"context"
	"errors"
)

type DocumentSource interface {
	PublishedDocuments(context.Context) ([]string, error)
	Document(context.Context, string) (any, error)
}

func Reindex(ctx context.Context, source DocumentSource, run func(context.Context, string) error, progress func(int)) error {
	ids, err := source.PublishedDocuments(ctx)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		progress(100)
		return nil
	}
	for index, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := run(ctx, id); err != nil {
			return errors.Join(errors.New("重建索引失败"), err)
		}
		progress((index + 1) * 100 / len(ids))
	}
	return nil
}
