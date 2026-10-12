package cache

import "context"

type Translator interface {
	TranslateAnswer(context.Context, string, string, string) (string, error)
}

func Translate(ctx context.Context, model Translator, value Entry, target string) (Entry, error) {
	answer, err := model.TranslateAnswer(ctx, value.Answer, value.Language, target)
	if err != nil {
		return Entry{}, err
	}
	value.Answer = answer
	value.Language = target
	return value, nil
}
