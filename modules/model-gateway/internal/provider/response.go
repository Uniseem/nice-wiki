package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptrace"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

type ProviderOutcome struct {
	RequestID       string                `json:"requestId"`
	InputTokens     *int                  `json:"inputTokens,omitempty"`
	OutputTokens    *int                  `json:"outputTokens,omitempty"`
	ReportedCost    *gen.Money            `json:"reportedCost,omitempty"`
	PricingComplete bool                  `json:"pricingComplete"`
	Output          *platform.ModelOutput `json:"output,omitempty"`
	Completed       bool                  `json:"completed"`
	HTTPStatus      int                   `json:"httpStatus"`
	AcceptanceProof string                `json:"acceptanceProof"`
	Code            string                `json:"code"`
	OutputStarted   bool                  `json:"outputStarted"`
}
type providerFrame struct {
	ID           string  `json:"id"`
	Model        string  `json:"model"`
	ModelVersion string  `json:"modelVersion"`
	ResponseID   string  `json:"responseId"`
	Type         string  `json:"type"`
	StopReason   *string `json:"stop_reason"`
	Choices      []struct {
		Delta struct {
			Content *string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Delta struct {
		Text       *string `json:"text"`
		StopReason *string `json:"stop_reason"`
	} `json:"delta"`
	Usage   *providerUsage `json:"usage"`
	Message *struct {
		ID    string         `json:"id"`
		Usage *providerUsage `json:"usage"`
	} `json:"message"`
	UsageMetadata *struct {
		Input    *int `json:"promptTokenCount"`
		Output   *int `json:"candidatesTokenCount"`
		Thoughts *int `json:"thoughtsTokenCount"`
		Cached   *int `json:"cachedContentTokenCount"`
		Total    *int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Data []struct {
		Index     *int      `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Embedding *struct {
		Values []float64 `json:"values"`
	} `json:"embedding"`
	Embeddings []struct {
		Values []float64 `json:"values"`
	} `json:"embeddings"`
	Error *struct {
		Type   string `json:"type"`
		Code   any    `json:"code"`
		Status string `json:"status"`
	} `json:"error"`
}
type providerUsage struct {
	Prompt        *int        `json:"prompt_tokens"`
	Completion    *int        `json:"completion_tokens"`
	Input         *int        `json:"input_tokens"`
	Output        *int        `json:"output_tokens"`
	Total         *int        `json:"total_tokens"`
	Cost          json.Number `json:"cost"`
	Currency      string      `json:"currency"`
	CacheCreation int         `json:"cache_creation_input_tokens"`
	CacheRead     int         `json:"cache_read_input_tokens"`
	PromptDetails *struct {
		Cached int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	BilledUnits *struct {
		Input  *int `json:"input_tokens"`
		Output *int `json:"output_tokens"`
	} `json:"billed_units"`
}

// ProviderReportedCost将供应方明确USD金额按整数微额向上表示，不经float64。
func ProviderReportedCost(raw json.Number) (*gen.Money, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 128 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "供应方金额表示超过精度预算")
	}
	if index := strings.IndexAny(string(raw), "eE"); index >= 0 {
		exponent, err := strconv.ParseInt(string(raw)[index+1:], 10, 32)
		if err != nil || exponent < -128 || exponent > 128 {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "供应方金额指数超过有界精度预算")
		}
	}
	amount, ok := new(big.Rat).SetString(string(raw))
	if !ok || amount.Sign() < 0 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "供应方金额不合法")
	}
	amount.Mul(amount, big.NewRat(1000000, 1))
	integer, remainder := new(big.Int).QuoRem(amount.Num(), amount.Denom(), new(big.Int))
	if remainder.Sign() > 0 {
		integer.Add(integer, big.NewInt(1))
	}
	if integer.Cmp(big.NewInt(9007199254740991)) > 0 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "供应方金额超出整数微额范围")
	}
	return &gen.Money{Currency: "USD", Micros: int(integer.Int64())}, nil
}
func MergeProviderUsage(outcome *ProviderOutcome, usage *providerUsage, nonText bool, currency string) error {
	if usage == nil {
		return nil
	}
	for _, count := range []*int{usage.Prompt, usage.Completion, usage.Input, usage.Output, usage.Total} {
		if count != nil && (*count < 0 || *count > 9007199254740991) {
			return platform.NewError(platform.CodeIntegrityError, false, "供应方token数超出非负整数范围")
		}
	}
	if usage.CacheCreation < 0 || usage.CacheRead < 0 || usage.CacheCreation > 9007199254740991 || usage.CacheRead > 9007199254740991 {
		return platform.NewError(platform.CodeIntegrityError, false, "供应方缓存token数不合法")
	}
	if usage.Prompt != nil {
		outcome.InputTokens = usage.Prompt
	}
	if usage.Input != nil {
		outcome.InputTokens = usage.Input
	}
	if usage.Completion != nil {
		outcome.OutputTokens = usage.Completion
	}
	if usage.Output != nil {
		outcome.OutputTokens = usage.Output
	}
	if usage.BilledUnits != nil {
		if usage.BilledUnits.Input != nil {
			outcome.InputTokens = usage.BilledUnits.Input
		}
		if usage.BilledUnits.Output != nil {
			outcome.OutputTokens = usage.BilledUnits.Output
		}
	}
	if nonText {
		if outcome.OutputTokens == nil {
			zero := 0
			outcome.OutputTokens = &zero
		}
		if outcome.InputTokens == nil && usage.Total != nil {
			input := *usage.Total - *outcome.OutputTokens
			outcome.InputTokens = &input
		}
	}
	if usage.CacheCreation > 0 || usage.CacheRead > 0 {
		if outcome.InputTokens != nil {
			total := *outcome.InputTokens + usage.CacheCreation + usage.CacheRead
			outcome.InputTokens = &total
		}
		outcome.PricingComplete = false
	}
	if usage.PromptDetails != nil {
		if usage.PromptDetails.Cached < 0 || usage.PromptDetails.Cached > 9007199254740991 {
			return platform.NewError(platform.CodeIntegrityError, false, "供应方缓存token详情越界")
		}
		if usage.PromptDetails.Cached > 0 {
			outcome.PricingComplete = false
		}
	}
	var amount *gen.Money
	var err error
	if usage.Currency == "USD" || (usage.Currency == "" && currency == "USD") {
		amount, err = ProviderReportedCost(usage.Cost)
	}
	if err != nil {
		return err
	}
	if amount != nil {
		outcome.ReportedCost = amount
	}
	for _, count := range []*int{outcome.InputTokens, outcome.OutputTokens} {
		if count != nil && (*count < 0 || *count > 9007199254740991) {
			return platform.NewError(platform.CodeIntegrityError, false, "供应方合计token数越界")
		}
	}
	return nil
}
func ConsumeProviderFrame(raw []byte, adapter, capability, expectedModel, currency string, outcome *ProviderOutcome, emit func(string) error) error {
	if !utf8.Valid(raw) {
		return platform.NewError(platform.CodeIntegrityError, false, "供应方响应不是有效UTF8")
	}
	var frame providerFrame
	if json.Unmarshal(raw, &frame) != nil {
		return platform.NewError(platform.CodeUnsupportedSchema, false, "供应方响应JSON结构不可读")
	}
	if len(frame.Choices) > 1 || len(frame.Candidates) > 1 {
		return platform.NewError(platform.CodeUnsupportedSchema, false, "供应方返回多个未请求的候选答案")
	}
	if frame.ID != "" {
		outcome.RequestID = frame.ID
	}
	if frame.ResponseID != "" {
		outcome.RequestID = frame.ResponseID
	}
	if frame.Message != nil {
		if frame.Message.ID != "" {
			outcome.RequestID = frame.Message.ID
		}
		if err := MergeProviderUsage(outcome, frame.Message.Usage, false, currency); err != nil {
			return err
		}
	}
	if err := MergeProviderUsage(outcome, frame.Usage, capability == "embedding" || capability == "rerank", currency); err != nil {
		return err
	}
	if frame.UsageMetadata != nil {
		usage := frame.UsageMetadata
		if usage.Input != nil {
			outcome.InputTokens = usage.Input
		}
		if usage.Output != nil {
			total := *usage.Output
			if usage.Thoughts != nil {
				total += *usage.Thoughts
			}
			outcome.OutputTokens = &total
		}
		if usage.Cached != nil && *usage.Cached > 0 {
			outcome.PricingComplete = false
		}
		for _, count := range []*int{outcome.InputTokens, outcome.OutputTokens, usage.Thoughts, usage.Cached} {
			if count != nil && (*count < 0 || *count > 9007199254740991) {
				return platform.NewError(platform.CodeIntegrityError, false, "供应方实际token元数据越界")
			}
		}
	}
	if frame.Error != nil {
		code := frame.Error.Type
		if frame.Error.Status != "" {
			code = frame.Error.Status
		}
		if value, ok := frame.Error.Code.(string); ok {
			code = value
		}
		if outcome.HTTPStatus >= 400 && outcome.HTTPStatus < 500 && slices.Contains([]string{"invalid_api_key", "authentication_error", "permission_error", "invalid_request_error", "model_not_found", "rate_limit_exceeded", "rate_limit_error", "INVALID_ARGUMENT", "UNAUTHENTICATED", "PERMISSION_DENIED", "NOT_FOUND", "RESOURCE_EXHAUSTED"}, code) {
			outcome.AcceptanceProof = "not_accepted"
		}
		if os.Getenv("NW_DEBUG") != "" {
			// 只记 HTTP 状态和供应方错误码，不记请求正文或密钥。
			fmt.Fprintf(os.Stderr, "供应方返回错误 status=%d code=%v\n", outcome.HTTPStatus, code)
		}
		outcome.Code = platform.CodeDependencyUnavailable
		return platform.NewError(platform.CodeDependencyUnavailable, false, "供应方明确返回错误，原费用证据保留")
	}
	if capability == "embedding" {
		if (frame.Model != "" && frame.Model != expectedModel) || (frame.ModelVersion != "" && frame.ModelVersion != expectedModel) {
			return platform.NewError(platform.CodeStaleResult, false, "返回嵌入模型版本与固定请求版本不一致")
		}
		vectors := make([][]float64, 0)
		if adapter == "openai_embeddings" {
			ordered := make([][]float64, len(frame.Data))
			for _, item := range frame.Data {
				if item.Index == nil || *item.Index < 0 || *item.Index >= len(ordered) || ordered[*item.Index] != nil {
					return platform.NewError(platform.CodeIntegrityError, false, "向量响应index越界或重复")
				}
				ordered[*item.Index] = item.Embedding
			}
			vectors = ordered
		} else if frame.Embedding != nil {
			vectors = append(vectors, frame.Embedding.Values)
		} else {
			for _, item := range frame.Embeddings {
				vectors = append(vectors, item.Values)
			}
		}
		outcome.Output = &platform.ModelOutput{Capability: "embedding", Vectors: vectors}
		outcome.Completed = len(vectors) > 0
		return nil
	}
	if capability == "rerank" {
		outcome.Completed = true
		return nil
	}
	for _, choice := range frame.Choices {
		if choice.Delta.Content != nil {
			if err := emit(*choice.Delta.Content); err != nil {
				return err
			}
		}
		if choice.Message.Content != "" {
			if err := emit(choice.Message.Content); err != nil {
				return err
			}
		}
		if choice.FinishReason != nil {
			outcome.Completed = true
		}
	}
	for _, block := range frame.Content {
		if block.Type == "text" {
			if err := emit(block.Text); err != nil {
				return err
			}
		}
	}
	if frame.Delta.Text != nil {
		if err := emit(*frame.Delta.Text); err != nil {
			return err
		}
	}
	if frame.Type == "message_stop" || frame.StopReason != nil || frame.Delta.StopReason != nil {
		outcome.Completed = true
	}
	for _, candidate := range frame.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				if err := emit(part.Text); err != nil {
					return err
				}
			}
		}
		if candidate.FinishReason != "" {
			outcome.Completed = true
		}
	}
	return nil
}

// SendProviderRequest只执行一次已持久派发的请求，重定向禁止，错误正文不进入日志。
func SendProviderRequest(ctx context.Context, request BuildProviderRequestResult, provider gen.Provider, capability string, candidateIDs []string, emit func(string) error) (ProviderOutcome, error) {
	outcome := ProviderOutcome{PricingComplete: true, AcceptanceProof: "unknown"}
	work := ctx
	cancel := func() {}
	if provider.RequestTimeoutMs > 0 {
		work, cancel = context.WithTimeout(ctx, time.Duration(min(provider.RequestTimeoutMs, 180000))*time.Millisecond)
	} else {
		work, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	req, err := http.NewRequestWithContext(work, request.Method, request.URL, strings.NewReader(request.Body))
	if err != nil {
		return outcome, platform.NewError(platform.CodeIntegrityError, false, "固定供应方请求无法建立")
	}
	req.Header = request.Headers.Clone()
	var sent atomic.Bool
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{WroteHeaders: func() { sent.Store(true) }}))
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: time.Duration(min(provider.ConnectTimeoutMs, 10000)) * time.Millisecond}).DialContext, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: time.Duration(min(provider.RequestTimeoutMs, 180000)) * time.Millisecond}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		if !sent.Load() {
			outcome.AcceptanceProof = "not_accepted"
		}
		return outcome, platform.NewError(platform.CodeDependencyUnavailable, false, "供应方请求未完成，原费用状态不能假定为零")
	}
	defer response.Body.Close()
	outcome.HTTPStatus = response.StatusCode
	outcome.RequestID = response.Header.Get("x-request-id")
	if outcome.RequestID == "" {
		outcome.RequestID = response.Header.Get("request-id")
	}
	textBytes := 0
	var text strings.Builder
	writeText := func(delta string) error {
		if delta == "" {
			return nil
		}
		if !utf8.ValidString(delta) || textBytes+len(delta) > 65536 {
			return platform.NewError(platform.CodeBudgetExceeded, false, "供应方文本输出超过64KiB预算")
		}
		textBytes += len(delta)
		_, _ = text.WriteString(delta)
		for len(delta) > 0 {
			end := min(len(delta), 16384)
			for end < len(delta) && !utf8.RuneStart(delta[end]) {
				end--
			}
			if err := emit(delta[:end]); err != nil {
				return err
			}
			outcome.OutputStarted = true
			delta = delta[end:]
		}
		return nil
	}
	var idle *time.Timer
	if provider.StreamIdleTimeoutMs > 0 {
		idle = time.AfterFunc(time.Duration(min(provider.StreamIdleTimeoutMs, 180000))*time.Millisecond, cancel)
		defer idle.Stop()
	}
	reader := &providerIdleReader{source: response.Body, idle: idle, duration: time.Duration(min(provider.StreamIdleTimeoutMs, 180000)) * time.Millisecond}
	if strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") && response.StatusCode < 400 {
		scanner := bufio.NewScanner(io.LimitReader(reader, 64<<20))
		scanner.Buffer(make([]byte, 4096), 4<<20)
		data := make([]string, 0)
		done := false
		consume := func() error {
			if len(data) == 0 {
				return nil
			}
			raw := strings.Join(data, "\n")
			data = data[:0]
			if raw == "[DONE]" {
				outcome.Completed = true
				done = true
				return nil
			}
			if err := ConsumeProviderFrame([]byte(raw), string(provider.Adapter), capability, request.ModelVersion, request.ReportedCurrency, &outcome, writeText); err != nil {
				return err
			}
			var terminal struct {
				Type       string `json:"type"`
				Candidates []struct {
					Finish string `json:"finishReason"`
				} `json:"candidates"`
			}
			_ = json.Unmarshal([]byte(raw), &terminal)
			if terminal.Type == "message_stop" {
				done = true
			}
			if string(provider.Adapter) == "gemini" && len(terminal.Candidates) == 1 && terminal.Candidates[0].Finish != "" {
				done = true
			}
			return nil
		}
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if err := consume(); err != nil {
					return outcome, err
				}
				if done {
					break
				}
				continue
			}
			if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		if err := scanner.Err(); err != nil {
			return outcome, platform.NewError(platform.CodeDependencyUnavailable, false, "供应方流中断，保留已派发费用")
		}
		if err := consume(); err != nil {
			return outcome, err
		}
		if !done {
			return outcome, platform.NewError(platform.CodeDependencyUnavailable, false, "供应方流缺完整结束帧，原费用证据保留")
		}
	} else {
		raw, err := io.ReadAll(io.LimitReader(reader, (32<<20)+1))
		if err != nil {
			return outcome, platform.NewError(platform.CodeDependencyUnavailable, false, "供应方正文读取未完成")
		}
		if len(raw) > 32<<20 {
			return outcome, platform.NewError(platform.CodeBudgetExceeded, false, "供应方响应超过32MiB预算")
		}
		if err := ConsumeProviderFrame(raw, string(provider.Adapter), capability, request.ModelVersion, request.ReportedCurrency, &outcome, writeText); err != nil {
			return outcome, err
		}
		if capability == "rerank" && response.StatusCode < 400 {
			parsed, err := ParseRerankResponse(ctx, platform.OperationContext{}, ParseRerankResponseInput{CandidateIDs: candidateIDs, ResponseBody: string(raw)})
			if err != nil {
				return outcome, err
			}
			outcome.Output = &platform.ModelOutput{Capability: "rerank", CandidateIDs: parsed.OrderedIDs}
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "供应方HTTP状态 status=%d\n", response.StatusCode)
		}
		return outcome, platform.NewError(platform.CodeDependencyUnavailable, false, "供应方HTTP状态不表示成功")
	}
	outcome.AcceptanceProof = "accepted"
	if !outcome.Completed || outcome.Code != "" {
		return outcome, platform.NewError(platform.CodeDependencyUnavailable, false, "供应方没有给出完整终态证据")
	}
	if capability == "chat" || capability == "vision" || capability == "translation" {
		complete := text.String()
		outcome.Output = &platform.ModelOutput{Capability: capability, Text: &complete}
	}
	return outcome, nil
}

type providerIdleReader struct {
	source   io.Reader
	idle     *time.Timer
	duration time.Duration
}

func (r *providerIdleReader) Read(data []byte) (int, error) {
	n, err := r.source.Read(data)
	if n > 0 && r.idle != nil {
		r.idle.Reset(r.duration)
	}
	return n, err
}
