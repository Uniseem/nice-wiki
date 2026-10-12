package ingest

import (
	"context"
	"nice-wiki/internal/ai/retrieve"
)

type KeywordWriter interface {
	ReplaceKeywords(context.Context, string, string) error
}

func WriteKeywords(ctx context.Context, writer KeywordWriter, units []retrieve.Unit) error {
	for _, unit := range units {
		if err := writer.ReplaceKeywords(ctx, unit.ID, unit.Text); err != nil {
			return err
		}
	}
	return nil
}
