package search

import (
	"context"
	"sort"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type ReadSectionsInput = gen.ReadSectionsInput
type ReadSectionsResult = gen.ReadSectionsResult
type ReadDocumentChunksInput = gen.ReadDocumentChunksInput
type ReadDocumentChunksResult = gen.ReadDocumentChunksResult

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
	if runtime, rerr := searchRuntime(ctx); rerr == nil {
		if _, store, ok := runtime.remote(); ok {
			ids := make([]string, len(input.ChunkIds))
			copy(ids, input.ChunkIds)
			found, err := store.ChunksByID(ctx, op, snapshot.Space, ids)
			if err != nil {
				return ReadSectionsResult{}, err
			}
			byID = found
		}
	}
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

// ReadDocumentChunks 按文档读出活动索引里的全部分块（文内顺序），供问答把路由选中的文档整篇交给回答模型。
// 按请求顺序装入 tokenCount 预算：装不下的文档截断并标记，之后的文档不再给；已撤回或被替换的文档跳过。
func ReadDocumentChunks(ctx context.Context, op OperationContext, input ReadDocumentChunksInput) (ReadDocumentChunksResult, error) {
	snapshot, err := activeSnapshot(ctx, op)
	if err != nil {
		return ReadDocumentChunksResult{}, err
	}
	byDocument := map[string][]gen.Chunk{}
	if runtime, rerr := searchRuntime(ctx); rerr == nil {
		if _, store, ok := runtime.remote(); ok {
			ids := make([]string, len(input.DocumentIds))
			copy(ids, input.DocumentIds)
			found, err := store.ChunksByDocument(ctx, op, snapshot.Space, ids)
			if err != nil {
				return ReadDocumentChunksResult{}, err
			}
			byDocument = found
		}
	}
	for _, row := range snapshot.Rows {
		byDocument[row.Chunk.Document.DocumentId] = append(byDocument[row.Chunk.Document.DocumentId], row.Chunk)
	}
	result := ReadDocumentChunksResult{Documents: []gen.DocumentChunks{}, IndexVersionHash: snapshot.VersionHash}
	budget := input.MaximumTokens
	for _, id := range input.DocumentIds {
		chunks := byDocument[id]
		if len(chunks) == 0 {
			continue
		}
		sort.Slice(chunks, func(i, j int) bool { return chunks[i].Ordinal < chunks[j].Ordinal })
		candidates := make([]gen.Candidate, len(chunks))
		for i, chunk := range chunks {
			candidates[i] = gen.Candidate{Chunk: chunk}
		}
		validated, err := ValidateCandidates(ctx, op, ValidateCandidatesInput{Candidates: candidates})
		if err != nil {
			return ReadDocumentChunksResult{}, err
		}
		if len(validated.Candidates) != len(candidates) {
			continue
		}
		document := gen.DocumentChunks{DocumentId: id, Chunks: []gen.Chunk{}}
		for _, chunk := range chunks {
			if chunk.TokenCount > budget {
				document.Truncated = true
				break
			}
			budget -= chunk.TokenCount
			document.Chunks = append(document.Chunks, chunk)
		}
		if len(document.Chunks) == 0 {
			break
		}
		if err := ValidateSectionSources(ctx, op, document.Chunks); err != nil {
			return ReadDocumentChunksResult{}, err
		}
		result.Documents = append(result.Documents, document)
		if document.Truncated {
			break
		}
	}
	return result, nil
}
