package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"nice-wiki/libs/platform"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"nice-wiki/modules/model-gateway/internal/security"
	"slices"
	"strings"
)

type OperationContext = platform.OperationContext

type ClassifyFailureInput struct {
	HTTPStatus      int    `json:"httpStatus"`
	ProviderCode    string `json:"providerCode"`
	BytesSent       int    `json:"bytesSent"`
	ResponseStarted bool   `json:"responseStarted"`
	AcceptanceProof string `json:"acceptanceProof"`
	UsageRecorded   bool   `json:"usageRecorded"`
}
type ClassifyFailureResult struct {
	Classification            string `json:"classification"`
	RetryAfterMS              int    `json:"retryAfterMs"`
	RetainPreviousReservation bool   `json:"retainPreviousReservation"`
}

func ClassifyFailure(ctx context.Context, op OperationContext, input ClassifyFailureInput) (ClassifyFailureResult, error) {
	if input.AcceptanceProof == "unknown" || (input.AcceptanceProof == "accepted" && !input.UsageRecorded) {
		return ClassifyFailureResult{Classification: "charge_unknown", RetainPreviousReservation: true}, nil
	}
	if input.ResponseStarted {
		return ClassifyFailureResult{Classification: "terminal"}, nil
	}
	if input.ProviderCode == "invalid_structured_output" && input.UsageRecorded {
		return ClassifyFailureResult{Classification: "retry_same"}, nil
	}
	if input.HTTPStatus == 0 || input.HTTPStatus == 429 || input.HTTPStatus >= 500 {
		return ClassifyFailureResult{Classification: "fallback"}, nil
	}
	return ClassifyFailureResult{Classification: "terminal"}, nil
}

type BuildProviderRequestInput struct {
	InvocationID string `json:"invocationId"`
	Attempt      int    `json:"attempt"`
}
type BuildProviderRequestResult struct {
	Method           string      `json:"method"`
	URL              string      `json:"url"`
	Headers          http.Header `json:"headers"`
	Body             string      `json:"body"`
	ModelVersion     string      `json:"modelVersion"`
	ReportedCurrency string      `json:"reportedCurrency,omitempty"`
}

// BuildProviderRequest只使用accepted固定的模型/渠道/密文引用，不追随新active配置。
// EffectiveAdapter 让一个供应方同时承载对话和向量模型（参照 opencode 一个供应方挂多种模型）：
// OpenAI 兼容供应方的向量模型走 /embeddings；Gemini 的向量请求本来就在 gemini 分支里按能力区分。
// OpenRouter 上的决策模型（TypeSafe Jev）走同一供应方的 Decisions 接口（/api/alpha/decisions）。
func EffectiveAdapter(adapter, capability string) string {
	if adapter == "openai_chat" && capability == "embedding" {
		return "openai_embeddings"
	}
	if adapter == "openai_chat" && capability == "decision" {
		return "openrouter_decisions"
	}
	return adapter
}

func BuildProviderRequest(ctx context.Context, op OperationContext, input BuildProviderRequestInput) (BuildProviderRequestResult, error) {
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return BuildProviderRequestResult{}, err
	}
	record, err := gatewayruntime.ReadInvocationRecord(tx, op, input.InvocationID)
	if err != nil {
		tx.Close()
		return BuildProviderRequestResult{}, err
	}
	var selected string
	for _, attempt := range record.Invocation.Attempts {
		if attempt.Ordinal == input.Attempt {
			selected = attempt.AttemptId
		}
	}
	if selected == "" {
		tx.Close()
		return BuildProviderRequestResult{}, platform.NewError(platform.CodeNotFound, false, "供应方请求尝试不存在")
	}
	attempt, err := gatewayruntime.ReadAttemptRecord(tx, selected)
	tx.Close()
	if err != nil {
		return BuildProviderRequestResult{}, err
	}
	var route gatewayruntime.FixedRoute
	if json.Unmarshal(record.Route, &route) != nil {
		return BuildProviderRequestResult{}, platform.NewError(platform.CodeIntegrityError, false, "固定供应方路线损坏")
	}
	images, err := LoadImages(ctx, op, record.Request.Messages, route.AssetVersions)
	if err != nil {
		return BuildProviderRequestResult{}, err
	}
	headers, err := security.OpenAttemptHeaders(ctx, op, selected)
	if err != nil {
		return BuildProviderRequestResult{}, err
	}
	key, err := security.OpenAttemptCredential(ctx, op, selected)
	if err != nil {
		return BuildProviderRequestResult{}, err
	}
	defer clear(key)
	if len(key) == 0 || strings.ContainsAny(string(key), "\r\n\x00") {
		return BuildProviderRequestResult{}, platform.NewError(platform.CodeValidation, false, "供应方主认证值不符合HTTP头规则")
	}
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	request := record.Request
	output := min(request.MaximumOutputTokens, attempt.Model.MaxOutputTokens)
	path := ""
	body := ""
	base := strings.TrimRight(attempt.Provider.BaseUrl, "/")
	switch EffectiveAdapter(string(attempt.Provider.Adapter), string(request.Capability)) {
	case "anthropic_messages":
		mapped, err := MapAnthropicRequest(ctx, op, MapAnthropicRequestInput{Messages: request.Messages, ModelID: attempt.Model.ProviderModelVersion, MaximumOutputTokens: output, Images: images, Stream: true, Tools: request.Tools, ToolChoice: request.ToolChoice})
		if err != nil {
			return BuildProviderRequestResult{}, err
		}
		body = mapped.Body
		path = "/messages"
		headers.Set("x-api-key", string(key))
		headers.Set("anthropic-version", "2023-06-01")
		headers.Set("Accept", "text/event-stream")
	case "gemini", "gemini_embeddings":
		mapped, err := MapGeminiRequest(ctx, op, MapGeminiRequestInput{Messages: request.Messages, ModelID: attempt.Model.ProviderModelVersion, MaximumOutputTokens: output, Capability: string(request.Capability), Texts: request.Texts, Dimensions: attempt.Model.Dimension, Images: images, Stream: request.Capability != "embedding", Tools: request.Tools, ToolChoice: request.ToolChoice})
		if err != nil {
			return BuildProviderRequestResult{}, err
		}
		body = mapped.Body
		path = "/models/" + url.PathEscape(strings.TrimPrefix(attempt.Model.ProviderModelVersion, "models/")) + ":" + mapped.Method
		if mapped.Method == "streamGenerateContent" {
			path += "?alt=sse"
			headers.Set("Accept", "text/event-stream")
		}
		headers.Set("x-goog-api-key", string(key))
	case "openai_embeddings":
		raw, _ := json.Marshal(map[string]any{"model": attempt.Model.ProviderModelVersion, "input": request.Texts, "dimensions": attempt.Model.Dimension, "encoding_format": "float"})
		body = string(raw)
		path = "/embeddings"
		headers.Set("Authorization", "Bearer "+string(key))
	case "rerank":
		if request.Query == nil {
			return BuildProviderRequestResult{}, platform.NewError(platform.CodeValidation, false, "原生重排缺少query")
		}
		raw, _ := json.Marshal(map[string]any{"model": attempt.Model.ProviderModelVersion, "query": *request.Query, "documents": request.Texts, "top_n": len(request.CandidateIds), "return_documents": false})
		body = string(raw)
		path = "/rerank"
		headers.Set("Authorization", "Bearer "+string(key))
	case "openrouter_decisions":
		if request.Decision == nil {
			return BuildProviderRequestResult{}, platform.NewError(platform.CodeValidation, false, "决策请求缺少题目")
		}
		raw, _ := json.Marshal(map[string]any{"model": attempt.Model.ProviderModelVersion, "state": request.Decision.State, "questions": request.Decision.Questions})
		body = string(raw)
		// 供应方地址按对话接口配置（…/api/v1），决策接口在同级的 /api/alpha/decisions。
		base = strings.TrimSuffix(base, "/v1")
		path = "/alpha/decisions"
		headers.Set("Authorization", "Bearer "+string(key))
	case "openai_chat":
		payload, err := mapOpenAIChatPayload(request, attempt.Model.ProviderModelVersion, output, images)
		if err != nil {
			return BuildProviderRequestResult{}, err
		}
		// OpenRouter 上的混合推理模型（如 DeepSeek Flash）默认先“思考”，思考也占 max_tokens，
		// 输出额度小的任务（路径名、摘要）会只剩空回复。没标“推理”能力的模型关掉思考。
		if parsed, err := url.Parse(base); err == nil && strings.EqualFold(parsed.Hostname(), "openrouter.ai") && !slices.Contains(attempt.Model.Capabilities, "reasoning") {
			payload["reasoning"] = map[string]any{"enabled": false}
		}
		raw, _ := json.Marshal(payload)
		body = string(raw)
		path = "/chat/completions"
		headers.Set("Authorization", "Bearer "+string(key))
		headers.Set("Accept", "text/event-stream")
	default:
		return BuildProviderRequestResult{}, platform.NewError(platform.CodeUnsupportedCapability, false, "供应方协议不支持固定请求")
	}
	if len(body) > 32<<20 {
		return BuildProviderRequestResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "供应方编码请求超过32MiB预算")
	}
	reportedCurrency := ""
	if parsed, err := url.Parse(base); err == nil && strings.EqualFold(parsed.Hostname(), "openrouter.ai") {
		reportedCurrency = "USD"
	}
	return BuildProviderRequestResult{Method: "POST", URL: base + path, Headers: headers, Body: body, ModelVersion: attempt.Model.ProviderModelVersion, ReportedCurrency: reportedCurrency}, nil
}
