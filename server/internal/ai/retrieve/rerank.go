package retrieve

import (
	"context"
	"nice-wiki/internal/llm/protocol"
	"sort"
)

func RerankHits(ctx context.Context, model protocol.Reranker, modelID, query string, hits []Hit, text func(string) string) ([]Hit, error) {
	documents := make([]string, len(hits))
	for i := range hits {
		documents[i] = text(hits[i].ID)
	}
	scores, _, err := model.Rerank(ctx, modelID, query, documents)
	if err != nil {
		return nil, err
	}
	result := append([]Hit{}, hits...)
	for i := range result {
		if i < len(scores) {
			result[i].Score = scores[i]
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	return result, nil
}
