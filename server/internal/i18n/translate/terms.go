package translate

import "context"

type TermExtractor interface {
	ExtractTerms(context.Context, string, string) ([]Term, error)
}

func ExtractTerms(ctx context.Context, model TermExtractor, glossary *Glossary, document, language, text string) error {
	values, err := model.ExtractTerms(ctx, language, text)
	if err != nil {
		return err
	}
	for _, value := range values {
		value.Source = TermAutomatic
		value.DocumentID = document
		value.Language = language
		if err := glossary.Save(value); err != nil {
			return err
		}
	}
	return nil
}
