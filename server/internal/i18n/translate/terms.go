package translate

import (
	"context"
	"nice-wiki/internal/textutil"
)

type TermExtractor interface {
	ExtractTerms(context.Context, string, string) ([]Term, error)
}

func ExtractTerms(ctx context.Context, model TermExtractor, glossary *Glossary, document, language, text string) error {
	values, err := model.ExtractTerms(ctx, language, text)
	if err != nil {
		return err
	}
	for _, value := range values {
		if value.ID == "" {
			value.ID = textutil.Hash([]byte(language + "\x00" + value.SourceText))[:32]
		}
		value.Source = TermAutomatic
		value.DocumentID = document
		value.Language = language
		if err := glossary.Save(value); err != nil {
			return err
		}
	}
	return nil
}
