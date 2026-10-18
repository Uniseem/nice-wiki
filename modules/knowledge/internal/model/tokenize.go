package model

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type TokenizeSourceInput struct {
	ModelID string             `json:"modelId"`
	Space   gen.EmbeddingSpace `json:"space"`
	Text    string             `json:"text"`
}
type TokenizeSourceResult struct {
	TokenEnds  []int `json:"tokenEnds"`
	TokenCount int   `json:"tokenCount"`
}

// TokenizeSource 只调用免费真实BPE端口，核完整空间与原文字节，不猜测分词。
func TokenizeSource(ctx context.Context, op OperationContext, input TokenizeSourceInput) (TokenizeSourceResult, error) {
	runtime, ok := ctx.Value(modelRuntimeKey{}).(ModelRuntime)
	if !ok || runtime.Raw == nil || runtime.ValidateTokenization == nil {
		return TokenizeSourceResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "真实分词端口未装配")
	}
	service := op
	service.CallerService = "knowledge"
	service.Actor = platform.AuthContext{SubjectID: "knowledge", Kind: "service", SiteID: op.Actor.SiteID, ExpiresAt: op.Deadline, Grants: []string{}}
	raw, err := runtime.Raw.TokenizeText(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service), struct {
		ModelID string `json:"modelId"`
		Text    string `json:"text"`
	}{input.ModelID, input.Text})
	if err != nil {
		return TokenizeSourceResult{}, err
	}
	if err = runtime.ValidateTokenization(raw); err != nil {
		return TokenizeSourceResult{}, err
	}
	var result struct {
		Space            gen.EmbeddingSpace `json:"embeddingSpace"`
		TokenizerVersion string             `json:"tokenizerVersion"`
		TokenCount       int                `json:"tokenCount"`
		TokenEnds        []int              `json:"byteTokenEnds"`
		TextHash         string             `json:"textHash"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Space != input.Space || result.TokenizerVersion != input.Space.PreprocessingVersion || result.TextHash != platform.HashBytes([]byte(input.Text)) || result.TokenCount != len(result.TokenEnds) {
		return TokenizeSourceResult{}, platform.NewError(platform.CodeIntegrityError, false, "真实分词来源、空间或计数不一致")
	}
	previous := 0
	for _, end := range result.TokenEnds {
		if end <= previous || end > len(input.Text) {
			return TokenizeSourceResult{}, platform.NewError(platform.CodeIntegrityError, false, "真实分词字节范围不一致")
		}
		previous = end
	}
	if previous != len(input.Text) {
		return TokenizeSourceResult{}, platform.NewError(platform.CodeIntegrityError, false, "真实分词未覆盖完整原文")
	}
	return TokenizeSourceResult{TokenEnds: result.TokenEnds, TokenCount: result.TokenCount}, nil
}
