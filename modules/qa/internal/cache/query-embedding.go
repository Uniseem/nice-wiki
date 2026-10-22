package cache

import (
	"context"
	"math"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type AcquireQueryEmbeddingInput struct {
	CanonicalQuestion string             `json:"canonicalQuestion"`
	Space             gen.EmbeddingSpace `json:"space"`
}
type AcquireQueryEmbeddingResult struct {
	Embedding qaruntime.QueryEmbedding `json:"embedding"`
}

func AcquireQueryEmbedding(ctx context.Context, op OperationContext, input AcquireQueryEmbeddingInput) (AcquireQueryEmbeddingResult, error) {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return AcquireQueryEmbeddingResult{}, err
	}
	if runtime.Knowledge == nil {
		return AcquireQueryEmbeddingResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "知识查询嵌入权威未装配")
	}
	if op.CostClass == platform.CostClassQA && runtime.Permit() == nil {
		return AcquireQueryEmbeddingResult{}, platform.NewError(platform.CodeForbidden, false, "查询嵌入缺少已消费准入许可")
	}
	queryHash := platform.HashBytes([]byte(input.CanonicalQuestion))
	key := "qa.query." + platform.HashBytes([]byte(op.RootOperationID+":"+queryHash+":"+input.Space.SpaceId))
	raw, err := runtime.Knowledge.CreateQueryEmbedding(qaruntime.ServiceReadContext(ctx, op), struct {
		Query  string                    `json:"query"`
		Space  gen.EmbeddingSpace        `json:"space"`
		Key    string                    `json:"idempotencyKey"`
		Permit *platform.AdmissionPermit `json:"admissionPermit,omitempty"`
	}{input.CanonicalQuestion, input.Space, key, runtime.Permit()})
	if err != nil {
		return AcquireQueryEmbeddingResult{}, err
	}
	var result AcquireQueryEmbeddingResult
	if err := qaruntime.DecodeResponse(runtime, "KnowledgeCreateQueryEmbeddingResult", raw, &result); err != nil {
		return AcquireQueryEmbeddingResult{}, err
	}
	embedding := result.Embedding
	if embedding.QueryHash != queryHash || embedding.Space != input.Space || embedding.DataEpoch != op.DataEpoch || len(embedding.Vector) != input.Space.Dimensions {
		return AcquireQueryEmbeddingResult{}, platform.NewError(platform.CodeIntegrityError, false, "查询嵌入身份、空间或维度不一致")
	}
	for _, value := range embedding.Vector {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return AcquireQueryEmbeddingResult{}, platform.NewError(platform.CodeIntegrityError, false, "查询嵌入包含非有限分量")
		}
	}
	encoded, _ := platform.CanonicalJSON(embedding.Vector)
	if platform.HashBytes(encoded) != embedding.VectorHash {
		return AcquireQueryEmbeddingResult{}, platform.NewError(platform.CodeIntegrityError, false, "查询嵌入摘要不一致")
	}
	return result, nil
}
