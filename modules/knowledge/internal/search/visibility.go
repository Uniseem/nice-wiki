package search

import (
	"context"
	"encoding/json"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type ValidateCandidatesInput struct { Candidates []gen.Candidate `json:"candidates"` }
type ValidateCandidatesResult struct { Candidates []gen.Candidate `json:"candidates"` }
type publicationValidation struct {
	Reference gen.PublishedRevisionRef `json:"reference"`
	Visible bool `json:"visible"`
	MatchesCurrent bool `json:"matchesCurrent"`
	CurrentRevision int `json:"currentRevision"`
	VisibilityRevision int `json:"visibilityRevision"`
}

// ValidateCandidates 不把依赖失败视为空结果，引用逐项与权威响应身份对应。
func ValidateCandidates(ctx context.Context, op OperationContext, input ValidateCandidatesInput) (ValidateCandidatesResult, error) {
	result := ValidateCandidatesResult{Candidates: make([]gen.Candidate, 0)}
	if len(input.Candidates) == 0 { return result, nil }
	runtime, err := searchRuntime(ctx)
	if err != nil { return ValidateCandidatesResult{}, err }
	refs := make([]gen.PublishedRevisionRef, 0)
	seen := make(map[string]bool)
	for _, candidate := range input.Candidates {
		ref := candidate.Chunk.Document
		if ref.DataEpoch != op.DataEpoch || candidate.Chunk.Block.Document != ref { return ValidateCandidatesResult{}, platform.NewError(platform.CodeStaleResult, false, "候选来源代次不匹配") }
		key := publicationKey(ref)
		if !seen[key] { seen[key] = true; refs = append(refs, ref) }
	}
	approved := make(map[string]int)
	for start := 0; start < len(refs); start += 100 {
		batch := refs[start:min(start+100, len(refs))]
		raw, err := runtime.Content.ValidatePublications(platform.WithOperationContext(ctx, op), struct { Refs []gen.PublishedRevisionRef `json:"refs"` }{batch})
		if err != nil { return ValidateCandidatesResult{}, err }
		var response struct { Validations []publicationValidation `json:"validations"` }
		if runtime.ValidatePublicationResult(raw) != nil || json.Unmarshal(raw, &response) != nil || len(response.Validations) != len(batch) { return ValidateCandidatesResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "发布权威返回不完整") }
		for i, validation := range response.Validations {
			if validation.Reference != batch[i] || validation.VisibilityRevision < 0 { return ValidateCandidatesResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "发布权威引用不匹配") }
			if validation.Visible && validation.MatchesCurrent && validation.CurrentRevision == batch[i].Revision { approved[publicationKey(batch[i])] = validation.VisibilityRevision }
		}
	}
	for _, candidate := range input.Candidates {
		if revision, ok := approved[publicationKey(candidate.Chunk.Document)]; ok { candidate.VisibilityRevision = revision; result.Candidates = append(result.Candidates, candidate) }
	}
	return result, nil
}
