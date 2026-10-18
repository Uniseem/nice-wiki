package search

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type ReadSectionsInput = gen.ReadSectionsInput
type ReadSectionsResult = gen.ReadSectionsResult

// ReadSections 仅从 active 快照选精确引用，任一旧来源使整次读取失败。
func ReadSections(ctx context.Context, op OperationContext, input ReadSectionsInput) (ReadSectionsResult, error) {
	if len(input.ChunkIds) > 20 {
		return ReadSectionsResult{}, platform.NewError(platform.CodeValidation, false, "一次最多读取二十个证据块")
	}
	snapshot, err := activeSnapshot(ctx, op)
	if err != nil {
		return ReadSectionsResult{}, err
	}
	refs := make(map[string]bool)
	for _, ref := range input.DocumentRefs {
		refs[publicationKey(ref)] = true
	}
	byID := make(map[string]gen.Chunk)
	for _, row := range snapshot.Rows {
		byID[row.Chunk.ChunkId] = row.Chunk
	}
	candidates := make([]gen.Candidate, 0, len(input.ChunkIds))
	for _, id := range input.ChunkIds {
		chunk, ok := byID[id]
		if !ok {
			return ReadSectionsResult{}, platform.NewError(platform.CodeNotFound, false, "证据块不是当前活动索引")
		}
		if !refs[publicationKey(chunk.Document)] {
			return ReadSectionsResult{}, platform.NewError(platform.CodeStaleResult, false, "证据块来源与调用引用不一致")
		}
		candidates = append(candidates, gen.Candidate{Chunk: chunk})
	}
	validated, err := ValidateCandidates(ctx, op, ValidateCandidatesInput{Candidates: candidates})
	if err != nil {
		return ReadSectionsResult{}, err
	}
	if len(validated.Candidates) != len(candidates) {
		return ReadSectionsResult{}, platform.NewError(platform.CodeStaleResult, false, "证据来源已撤回或被替换")
	}
	result := ReadSectionsResult{Chunks: make([]gen.Chunk, 0, len(candidates))}
	for _, candidate := range validated.Candidates {
		result.Chunks = append(result.Chunks, candidate.Chunk)
	}
	if err := ValidateSectionSources(ctx, op, result.Chunks); err != nil {
		return ReadSectionsResult{}, err
	}
	return result, nil
}
