package answer

import (
	"context"
	"encoding/json"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

// ReadDocumentsInput 描述整篇读取：文档路由选出的文档按优先顺序，材料（含每段的引用标识）按网关估算口径合计不超过 MaximumTokens。
type ReadDocumentsInput struct {
	TurnID        string
	DocumentIDs   []string
	Language      string
	MaximumTokens int
}

// ReadDocuments 把文档路由选中的文章整篇（按文内顺序的全部分块）交给回答模型；预算不够时截断，后面的文档不再给。
// 每个分块仍是一个可引用的单元。一篇都读不到时返回空材料，由调用方改走片段检索。
func ReadDocuments(ctx context.Context, op OperationContext, input ReadDocumentsInput) (RetrieveEvidenceResult, error) {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return RetrieveEvidenceResult{}, err
	}
	if runtime.Knowledge == nil {
		return RetrieveEvidenceResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "知识证据权威未装配")
	}
	raw, err := runtime.Knowledge.ReadDocumentChunks(qaruntime.ServiceReadContext(ctx, op), map[string]any{"documentIds": input.DocumentIDs, "maximumTokens": input.MaximumTokens})
	if err != nil {
		return RetrieveEvidenceResult{}, err
	}
	var response struct {
		Documents []struct {
			Chunks    []evidenceChunk `json:"chunks"`
			Truncated bool            `json:"truncated"`
		} `json:"documents"`
		IndexVersionHash string `json:"indexVersionHash"`
	}
	if err := qaruntime.DecodeResponse(runtime, "KnowledgeReadDocumentChunksResult", raw, &response); err != nil {
		return RetrieveEvidenceResult{}, err
	}
	type section struct {
		CitationID string `json:"citationId"`
		Text       string `json:"text"`
	}
	type article struct {
		Title     string    `json:"title"`
		Truncated bool      `json:"truncated,omitempty"`
		Sections  []section `json:"sections"`
	}
	result := RetrieveEvidenceResult{Citations: []gen.Citation{}, EvidenceText: "[]", IndexVersionHash: response.IndexVersionHash}
	articles := []article{}
	used := 0 // 知识索引按分块正文的 tokenCount 预筛；这里再按网关口径算上引用标识和 JSON，片段很碎时也不会撑爆回答窗口
	for _, document := range response.Documents {
		if used >= input.MaximumTokens {
			break
		}
		if len(document.Chunks) == 0 {
			continue
		}
		published, err := qaruntime.ReadPublishedDocument(ctx, op, document.Chunks[0].Document)
		if err != nil {
			return RetrieveEvidenceResult{}, err
		}
		citations := make([]gen.Citation, 0, len(document.Chunks))
		for _, chunk := range document.Chunks {
			if platform.HashBytes([]byte(chunk.Text)) != chunk.ChunkHash {
				return RetrieveEvidenceResult{}, platform.NewError(platform.CodeStaleResult, false, "文档分块摘要不一致")
			}
			citations = append(citations, gen.Citation{CitationId: "citation." + platform.HashBytes([]byte(input.TurnID+":"+chunk.ChunkID)), Document: chunk.Document, Block: chunk.Block, ChunkId: chunk.ChunkID, ExcerptHash: chunk.ChunkHash, Title: published.Metadata.Title, VisibilityRevision: published.VisibilityRevision})
		}
		checked, err := qaruntime.ValidateCitations(ctx, op, citations, input.Language)
		if err != nil {
			return RetrieveEvidenceResult{}, err
		}
		item := article{Title: published.Metadata.Title, Truncated: document.Truncated, Sections: make([]section, 0, len(checked))}
		for i, citation := range checked {
			cost := platform.EstimateInputTokens(len(document.Chunks[i].Text)+len(citation.CitationId)+32, 0)
			if used+cost > input.MaximumTokens {
				item.Truncated = true
				break
			}
			used += cost
			item.Sections = append(item.Sections, section{citation.CitationId, document.Chunks[i].Text})
			result.Citations = append(result.Citations, citation)
		}
		if len(item.Sections) > 0 {
			articles = append(articles, item)
		}
		if item.Truncated {
			break
		}
	}
	encoded, _ := json.Marshal(articles)
	result.EvidenceText = string(encoded)
	return result, nil
}
