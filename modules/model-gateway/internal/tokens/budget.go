// Package tokens 提供不读取正文存储、不调用供应方的保守文本预算。
package tokens

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"nice-wiki/modules/model-gateway/internal/catalog"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/gen"
)

type OperationContext = platform.OperationContext
type EstimateTokensInput struct {
	ModelId  string               `json:"modelId"`
	Texts    []string             `json:"texts"`
	Messages []gen.GatewayMessage `json:"messages"`
}
type EstimateTokensResult struct {
	InputTokens      int    `json:"inputTokens"`
	EstimatorVersion string `json:"estimatorVersion"`
}

// EstimateTokens 使用已确认的UTF8回退规则；没有内置模型tokenizer或图像预算声明。
func EstimateTokens(ctx context.Context, op OperationContext, input EstimateTokensInput) (EstimateTokensResult, error) {
	size := 0
	for _, text := range input.Texts {
		size += len(text)
	}
	for _, message := range input.Messages {
		if len(message.Content) > 65536 {
			return EstimateTokensResult{}, platform.NewError(platform.CodeValidation, false, "单条消息正文超过UTF8字节预算")
		}
		size += len(message.Content)
		if message.Parts != nil {
			for _, part := range *message.Parts {
				if part.Kind == "image" {
					return EstimateTokensResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "图像token预算尚无确认算法，不能给出文本估算上限")
				}
				size += len(*part.Text)
			}
		}
	}
	base := (size+1)/2 + len(input.Messages)*32
	return EstimateTokensResult{InputTokens: (base*6 + 4) / 5, EstimatorVersion: "utf8-half-message32-six-fifths-v1"}, nil
}

// NewTokenHandlers 只绑定localization免费文本估算，严格结构与服务身份由公共入口建立。
func NewTokenHandlers() map[string]http.Handler {
	return map[string]http.Handler{"estimateTokens": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		op, _ := platform.OperationContextFrom(r.Context())
		if op.CallerService != "localization" || op.Actor.Kind != "service" || op.Actor.SubjectID != "localization" {
			_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeForbidden, false, "文本预算只允许localization纯服务"), op.RequestID))
			return
		}
		var input EstimateTokensInput
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "文本预算输入不可读"), op.RequestID))
			return
		}
		result, err := EstimateTokens(r.Context(), op, input)
		if err != nil {
			_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_ = json.NewEncoder(w).Encode(result)
	})}
}

// ValidateEmbeddingSpace核对MG原空间身份和原始有限向量；默认none，不擅自L2归一化。
type ValidateEmbeddingSpaceInput struct {
	Space   gen.EmbeddingSpace `json:"space"`
	Vectors [][]float64        `json:"vectors"`
}
type ValidateEmbeddingSpaceResult struct {
	Valid            bool   `json:"valid"`
	EmbeddingSpaceID string `json:"embeddingSpaceId"`
}

func ValidateEmbeddingSpace(ctx context.Context, op OperationContext, input ValidateEmbeddingSpaceInput) (ValidateEmbeddingSpaceResult, error) {
	model := catalog.Model{ProviderId: input.Space.Provider, ProviderModelVersion: input.Space.ModelVersion, Dimension: input.Space.Dimensions, PreprocessorVersion: input.Space.PreprocessingVersion}
	space, err := catalog.ModelEmbeddingSpace(model)
	if err != nil {
		return ValidateEmbeddingSpaceResult{}, err
	}
	if space.SpaceID != input.Space.SpaceId {
		return ValidateEmbeddingSpaceResult{}, platform.NewError(platform.CodeIntegrityError, false, "嵌入空间摘要与原五字段身份不一致")
	}
	for _, vector := range input.Vectors {
		if len(vector) != space.Dimensions {
			return ValidateEmbeddingSpaceResult{}, platform.NewError(platform.CodeIntegrityError, false, "向量维度与固定空间不一致")
		}
		for _, component := range vector {
			if math.IsNaN(component) || math.IsInf(component, 0) {
				return ValidateEmbeddingSpaceResult{}, platform.NewError(platform.CodeIntegrityError, false, "向量存在非有限分量")
			}
		}
	}
	return ValidateEmbeddingSpaceResult{Valid: true, EmbeddingSpaceID: space.SpaceID}, nil
}
