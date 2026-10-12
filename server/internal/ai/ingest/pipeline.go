package ingest

import (
	"context"
	"nice-wiki/internal/doc"
)

type Pipeline struct {
	Context  ContextModel
	Keywords KeywordWriter
}

func (p Pipeline) Run(ctx context.Context, documentID string, value doc.Document) error {
	units, err := Units(documentID, value)
	if err != nil {
		return err
	}
	if p.Context != nil {
		units, err = Contextualize(ctx, p.Context, units)
		if err != nil {
			return err
		}
	}
	if p.Keywords != nil {
		return WriteKeywords(ctx, p.Keywords, units)
	}
	return nil
}
