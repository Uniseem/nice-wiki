// Package search 实现基准语言检索；所有候选须经 content 权威过滤后交付。
package search

import (
	"context"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

type OperationContext = platform.OperationContext

type NormalizeQueryInput struct {
	Query    string `json:"query"`
	Language string `json:"language"`
}
type NormalizeQueryResult struct {
	Normalized string `json:"normalized"`
	QueryHash  string `json:"queryHash"`
}

// NormalizeQuery 保留大小写、统一换行和 NFC；摘要只绑定规范后的查询。
func NormalizeQuery(ctx context.Context, op OperationContext, input NormalizeQueryInput) (NormalizeQueryResult, error) {
	if !utf8.ValidString(input.Query) || len(input.Query) > 8192 || utf8.RuneCountInString(input.Query) > 2048 {
		return NormalizeQueryResult{}, platform.NewError(platform.CodeValidation, false, "查询超过字符或字节预算")
	}
	query := strings.TrimSpace(norm.NFC.String(strings.ReplaceAll(strings.ReplaceAll(input.Query, "\r\n", "\n"), "\r", "\n")))
	if query == "" || len(query) > 8192 || utf8.RuneCountInString(query) > 2048 {
		return NormalizeQueryResult{}, platform.NewError(platform.CodeValidation, false, "规范查询为空或超过预算")
	}
	return NormalizeQueryResult{Normalized: query, QueryHash: platform.HashBytes([]byte(query))}, nil
}

type EmbedQueryInput struct {
 AdmissionPermit *gen.AdmissionPermit `json:"admissionPermit,omitempty"`
	Normalized string             `json:"normalized"`
	Space      gen.EmbeddingSpace `json:"space"`
	ProfileID  string             `json:"profileId"`
}
type EmbedQueryResult struct {
	Vector []float64          `json:"vector"`
	Space  gen.EmbeddingSpace `json:"space"`
}

// EmbedQuery 只接受网关返回的完整同空间向量，付费执行交由装配的网关调用链。
func EmbedQuery(ctx context.Context, op OperationContext, input EmbedQueryInput) (EmbedQueryResult, error) {
	runtime, err := searchRuntime(ctx)
	if err != nil {
		return EmbedQueryResult{}, err
	}
	if runtime.EmbeddingProfileID != input.ProfileID {
		return EmbedQueryResult{}, platform.NewError(platform.CodeIntegrityError, false, "查询嵌入档位不匹配")
	}
	embedded, err := CreateQueryEmbedding(ctx, op, CreateQueryEmbeddingInput{Query: input.Normalized, Space: input.Space, AdmissionPermit: input.AdmissionPermit, IdempotencyKey: "query." + platform.HashBytes([]byte(op.RootOperationID+":"+input.Space.SpaceId+":"+input.Normalized))})
	if err != nil {
		return EmbedQueryResult{}, err
	}
	result := EmbedQueryResult{Vector: embedded.Embedding.Vector, Space: embedded.Embedding.Space}
	if result.Space != input.Space || !validVector(result.Vector, input.Space.Dimensions) {
		return EmbedQueryResult{}, platform.NewError(platform.CodeIntegrityError, false, "查询嵌入空间或向量不一致")
	}
	if err := ctx.Err(); err != nil {
		return EmbedQueryResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "嵌入已停止")
	}
	return result, nil
}
