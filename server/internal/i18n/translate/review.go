package translate

import "context"

type Reviewer interface {
	Review(context.Context, string, string, string) (string, error)
}

func Review(ctx context.Context, model Reviewer, language, source, target string, enabled bool) (string, error) {
	if !enabled {
		return target, nil
	}
	return model.Review(ctx, language, source, target)
}
