package retrieve

import "context"

type KeywordSearcher interface {
	SearchKeywords(context.Context, string, int) ([]Hit, error)
}

func Keyword(ctx context.Context, searcher KeywordSearcher, query string, limit int) ([]Hit, error) {
	if limit < 1 {
		limit = 20
	}
	return searcher.SearchKeywords(ctx, query, limit)
}
