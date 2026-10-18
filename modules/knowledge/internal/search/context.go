package search

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type SelectContextInput struct {
	Candidates    []gen.Candidate `json:"candidates"`
	MaximumTokens int             `json:"maximumTokens"`
	MaximumChunks int             `json:"maximumChunks"`
}
type SelectContextResult struct {
	Candidates []gen.Candidate `json:"candidates"`
	Truncated  bool            `json:"truncated"`
}

// SelectContext 保留整块和原始顺序，每文档最多三块；跳过的块明确记截断。
func SelectContext(ctx context.Context, op OperationContext, input SelectContextInput) (SelectContextResult, error) {
	if input.MaximumTokens < 0 || input.MaximumChunks < 0 || input.MaximumChunks > 20 {
		return SelectContextResult{}, platform.NewError(platform.CodeValidation, false, "上下文预算不合法")
	}
	result := SelectContextResult{Candidates: make([]gen.Candidate, 0)}
	perDocument := make(map[string]int)
	seen := make(map[string]bool)
	remaining := input.MaximumTokens
	for _, candidate := range input.Candidates {
		id := candidate.Chunk.ChunkId
		if seen[id] {
			continue
		}
		seen[id] = true
		document := candidate.Chunk.Document.DocumentId
		if perDocument[document] >= 3 || len(result.Candidates) >= input.MaximumChunks || candidate.Chunk.TokenCount > remaining {
			result.Truncated = true
			continue
		}
		remaining -= candidate.Chunk.TokenCount
		perDocument[document]++
		result.Candidates = append(result.Candidates, candidate)
	}
	return result, nil
}
