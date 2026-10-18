package search

import (
	"context"
	"sort"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type FuseCandidatesInput struct {
	Lexical []gen.Candidate `json:"lexical"`
	Vector []gen.Candidate `json:"vector"`
	Limit int `json:"limit"`
}
type FuseCandidatesResult struct { Candidates []gen.Candidate `json:"candidates"` }

// FuseCandidates 用召回顺序计算 RRF，重复 ID 的来源正文必须相同。
func FuseCandidates(ctx context.Context, op OperationContext, input FuseCandidatesInput) (FuseCandidatesResult, error) {
	merged := make(map[string]gen.Candidate)
	for source, candidates := range [][]gen.Candidate{input.Lexical, input.Vector} {
		for rank, candidate := range candidates {
			id := candidate.Chunk.ChunkId
			current, exists := merged[id]
			if exists {
				a, _ := platform.CanonicalJSON(current.Chunk)
				b, _ := platform.CanonicalJSON(candidate.Chunk)
				if string(a) != string(b) { return FuseCandidatesResult{}, platform.NewError(platform.CodeIntegrityError, false, "同分块标识对应不同来源") }
			} else {
				current = gen.Candidate{Chunk: candidate.Chunk}
			}
			if source == 0 && current.LexicalRank == 0 { current.LexicalRank = rank + 1 }
			if source == 1 && current.VectorRank == 0 { current.VectorRank = rank + 1 }
			merged[id] = current
		}
	}
	result := FuseCandidatesResult{Candidates: make([]gen.Candidate, 0, len(merged))}
	for _, candidate := range merged {
		if candidate.LexicalRank > 0 { candidate.Score += 1 / (60 + float64(candidate.LexicalRank)) }
		if candidate.VectorRank > 0 { candidate.Score += 1 / (60 + float64(candidate.VectorRank)) }
		result.Candidates = append(result.Candidates, candidate)
	}
	sort.Slice(result.Candidates, func(i, j int) bool {
		a, b := result.Candidates[i], result.Candidates[j]
		if a.Score != b.Score { return a.Score > b.Score }
		return a.Chunk.ChunkId < b.Chunk.ChunkId
	})
	limit := min(input.Limit, 40)
	if limit < 1 { return FuseCandidatesResult{}, platform.NewError(platform.CodeValidation, false, "融合数量必须为正") }
	if len(result.Candidates) > limit { result.Candidates = result.Candidates[:limit] }
	return result, nil
}
