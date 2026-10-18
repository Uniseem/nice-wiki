package search

import (
	"context"
	"math"
	"sort"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type SearchVectorInput struct {
	Vector []float64          `json:"vector"`
	Space  gen.EmbeddingSpace `json:"space"`
	Limit  int                `json:"limit"`
}
type SearchVectorResult struct {
	Candidates []gen.Candidate `json:"candidates"`
}

func validVector(vector []float64, dimensions int) bool {
	if len(vector) != dimensions || dimensions < 1 {
		return false
	}
	for _, value := range vector {
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return false
		}
	}
	return true
}

// unitVector 先除以最大绝对值，避免合法有限分量在平方时溢出。
func unitVector(vector []float64) ([]float64, bool) {
	largest := 0.0
	for _, value := range vector {
		largest = max(largest, math.Abs(value))
	}
	if largest == 0 {
		return nil, false
	}
	unit := make([]float64, len(vector))
	sum := 0.0
	for i, value := range vector {
		unit[i] = value / largest
		sum += unit[i] * unit[i]
	}
	norm := math.Sqrt(sum)
	for i := range unit {
		unit[i] /= norm
	}
	return unit, true
}

// SearchVector 仅在完整同空间 active 快照中比较 cosine，零向量不召回。
func SearchVector(ctx context.Context, op OperationContext, input SearchVectorInput) (SearchVectorResult, error) {
	snapshot, err := activeSnapshot(ctx, op)
	if err != nil {
		return SearchVectorResult{}, err
	}
	if snapshot.Space != input.Space || !validVector(input.Vector, input.Space.Dimensions) {
		return SearchVectorResult{}, platform.NewError(platform.CodeIntegrityError, false, "查询向量空间或维度不匹配")
	}
	result := SearchVectorResult{Candidates: make([]gen.Candidate, 0)}
	query, nonzero := unitVector(input.Vector)
	if !nonzero {
		return result, nil
	}
	type ranked struct {
		chunk gen.Chunk
		score float64
	}
	ranks := make([]ranked, 0)
	for _, row := range snapshot.Rows {
		if !validVector(row.Vector, input.Space.Dimensions) {
			return SearchVectorResult{}, platform.NewError(platform.CodeIntegrityError, false, "索引向量损坏")
		}
		unit, nonzero := unitVector(row.Vector)
		if !nonzero {
			continue
		}
		score := 0.0
		for i, value := range query {
			score += value * unit[i]
		}
		ranks = append(ranks, ranked{chunk: row.Chunk, score: score})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].score != ranks[j].score {
			return ranks[i].score > ranks[j].score
		}
		return ranks[i].chunk.ChunkId < ranks[j].chunk.ChunkId
	})
	for rank, row := range ranks[:min(len(ranks), min(80, max(input.Limit, 0)))] {
		result.Candidates = append(result.Candidates, gen.Candidate{Chunk: row.chunk, VectorRank: rank + 1})
	}
	return result, nil
}
