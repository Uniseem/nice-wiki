package answer

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/cache"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type RetrieveEvidenceInput struct {
	TurnID     string                    `json:"turnId"`
	Normalized string                    `json:"normalized"`
	Policy     gen.TypesQaPolicy         `json:"policy"`
	Embedding  *qaruntime.QueryEmbedding `json:"embedding,omitempty"`
	Language   string                    `json:"language"`
	// Select 是材料筛选：给出候选片段，返回要保留的片段 ID；nil 表示原样使用。
	Select func([]EvidenceCandidate) []string `json:"-"`
}

// EvidenceCandidate 是交给材料筛选的一段候选材料；Document 和 BlockID 用来找它的英文译文。
type EvidenceCandidate struct {
	ChunkID  string
	Text     string
	Document gen.PublishedRevisionRef
	BlockID  string
}
type RetrieveEvidenceResult struct {
	Citations        []gen.Citation `json:"citations"`
	EvidenceText     string         `json:"evidenceText"`
	IndexVersionHash string         `json:"indexVersionHash"`
}
type evidenceChunk struct {
	Ordinal       int                      `json:"ordinal"`
	Asset         *platform.AssetRef       `json:"asset,omitempty"`
	ChunkID       string                   `json:"chunkId"`
	Document      gen.PublishedRevisionRef `json:"document"`
	Block         gen.BlockRef             `json:"block"`
	Text          string                   `json:"text"`
	ChunkHash     string                   `json:"chunkHash"`
	TokenCount    int                      `json:"tokenCount"`
	SourceOffsets []int                    `json:"sourceOffsets"`
	ByteStart     int                      `json:"byteStart"`
	ByteEnd       int                      `json:"byteEnd"`
}

func RetrieveEvidence(ctx context.Context, op OperationContext, input RetrieveEvidenceInput) (RetrieveEvidenceResult, error) {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return RetrieveEvidenceResult{}, err
	}
	if runtime.Knowledge == nil {
		return RetrieveEvidenceResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "知识证据权威未装配")
	}
	embedding := input.Embedding
	if embedding == nil {
		created, err := cache.AcquireQueryEmbedding(ctx, op, cache.AcquireQueryEmbeddingInput{CanonicalQuestion: input.Normalized, Space: input.Policy.RetrievalSpace})
		if err != nil {
			return RetrieveEvidenceResult{}, err
		}
		embedding = &created.Embedding
	}
	search := func() ([]evidenceChunk, bool, string, error) {
		request := map[string]any{"query": input.Normalized, "language": runtime.BaseLanguage, "space": input.Policy.RetrievalSpace, "maximumTokens": min(4096, input.Policy.MaximumContextTokens), "lexicalOnly": false, "embedding": embedding}
		if permit := runtime.Permit(); permit != nil {
			request["admissionPermit"] = permit
		}
		raw, err := runtime.Knowledge.SearchKnowledge(qaruntime.ServiceReadContext(ctx, op), request)
		if err != nil {
			return nil, false, "", err
		}
		var response struct {
			Result struct {
				Candidates []struct {
					Chunk evidenceChunk `json:"chunk"`
				} `json:"candidates"`
				Space            gen.EmbeddingSpace `json:"space"`
				IndexVersionHash string             `json:"indexVersionHash"`
				Truncated        bool               `json:"truncated"`
			} `json:"result"`
		}
		if err := qaruntime.DecodeResponse(runtime, "KnowledgeSearchResult", raw, &response); err != nil {
			return nil, false, "", err
		}
		if response.Result.Space != input.Policy.RetrievalSpace || len(response.Result.Candidates) > 8 {
			return nil, false, "", platform.NewError(platform.CodeIntegrityError, false, "证据结果空间或数量不一致")
		}
		chunks := make([]evidenceChunk, 0, len(response.Result.Candidates))
		for _, candidate := range response.Result.Candidates {
			chunks = append(chunks, candidate.Chunk)
		}
		return chunks, response.Result.Truncated, response.Result.IndexVersionHash, nil
	}
	found, truncated, indexVersion, err := search()
	if err != nil {
		return RetrieveEvidenceResult{}, err
	}
	original := make(map[string]evidenceChunk)
	candidates := make([]evidenceChunk, 0, len(found))
	for _, chunk := range found {
		if _, exists := original[chunk.ChunkID]; !exists {
			original[chunk.ChunkID] = chunk
			candidates = append(candidates, chunk)
		}
	}
	if input.Select != nil && len(candidates) > 0 {
		offered := make([]EvidenceCandidate, len(candidates))
		for i, chunk := range candidates {
			offered[i] = EvidenceCandidate{ChunkID: chunk.ChunkID, Text: chunk.Text, Document: chunk.Document, BlockID: chunk.Block.BlockId}
		}
		keep := map[string]bool{}
		for _, id := range input.Select(offered) {
			keep[id] = true
		}
		kept := candidates[:0]
		for _, chunk := range candidates {
			if keep[chunk.ChunkID] {
				kept = append(kept, chunk)
			}
		}
		candidates = kept
	}
	ids := make([]string, 0, 8)
	refs := make([]gen.PublishedRevisionRef, 0, 8)
	budget := min(4096, input.Policy.MaximumContextTokens)
	for _, chunk := range candidates {
		if len(ids) == 8 || chunk.TokenCount > budget {
			continue
		}
		budget -= chunk.TokenCount
		ids = append(ids, chunk.ChunkID)
		refs = append(refs, chunk.Document)
	}
	result := RetrieveEvidenceResult{Citations: make([]gen.Citation, 0), EvidenceText: "[]", IndexVersionHash: indexVersion}
	if len(ids) == 0 {
		if truncated && input.Select == nil {
			return RetrieveEvidenceResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "材料预算不足，不能把未召回材料当作无来源答案")
		}
		return result, nil
	}
	raw, err := runtime.Knowledge.ReadSections(qaruntime.ServiceReadContext(ctx, op), struct {
		IDs  []string                   `json:"chunkIds"`
		Refs []gen.PublishedRevisionRef `json:"documentRefs"`
	}{ids, refs})
	if err != nil {
		return RetrieveEvidenceResult{}, err
	}
	var sections struct {
		Chunks []evidenceChunk `json:"chunks"`
	}
	if err := qaruntime.DecodeResponse(runtime, "KnowledgeReadSectionsResult", raw, &sections); err != nil {
		return RetrieveEvidenceResult{}, err
	}
	if len(sections.Chunks) != len(ids) {
		return RetrieveEvidenceResult{}, platform.NewError(platform.CodeStaleResult, false, "证据片段读取不完整")
	}
	type material struct {
		CitationID string `json:"citationId"`
		Text       string `json:"text"`
	}
	materials := make([]material, 0, len(ids))
	tokens := 0
	seen := make(map[string]bool)
	for _, chunk := range sections.Chunks {
		prior, exists := original[chunk.ChunkID]
		a, _ := platform.CanonicalJSON(prior)
		b, _ := platform.CanonicalJSON(chunk)
		if !exists || seen[chunk.ChunkID] || string(a) != string(b) || platform.HashBytes([]byte(chunk.Text)) != chunk.ChunkHash || len(chunk.SourceOffsets) != len(chunk.Text) || chunk.TokenCount > min(4096, input.Policy.MaximumContextTokens)-tokens {
			return RetrieveEvidenceResult{}, platform.NewError(platform.CodeStaleResult, false, "证据来源、摘要、偏移或预算已变化")
		}
		seen[chunk.ChunkID] = true
		tokens += chunk.TokenCount
		document, err := qaruntime.ReadPublishedDocument(ctx, op, chunk.Document)
		if err != nil {
			return RetrieveEvidenceResult{}, err
		}
		citationID := "citation." + platform.HashBytes([]byte(input.TurnID+":"+chunk.ChunkID))
		citation := gen.Citation{CitationId: citationID, Document: chunk.Document, Block: chunk.Block, ChunkId: chunk.ChunkID, ExcerptHash: chunk.ChunkHash, Title: document.Metadata.Title, VisibilityRevision: document.VisibilityRevision}
		checked, err := qaruntime.ValidateCitations(ctx, op, []gen.Citation{citation}, input.Language)
		if err != nil {
			return RetrieveEvidenceResult{}, err
		}
		result.Citations = append(result.Citations, checked[0])
		materials = append(materials, material{citationID, chunk.Text})
	}
	encoded, _ := json.Marshal(materials)
	result.EvidenceText = string(encoded)
	return result, nil
}
