package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelagent"
	"nice-wiki/modules/model-gateway/internal/gen"
	"strings"
	"testing"
)

func sendToolStream(t *testing.T, adapter string, frames []string, done bool) (ProviderOutcome, string, error) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, frame := range frames {
			fmt.Fprintf(w, "data: %s\n\n", frame)
		}
		if done {
			fmt.Fprint(w, "data: [DONE]\n\n")
		}
	}))
	t.Cleanup(server.Close)
	var text strings.Builder
	outcome, err := SendProviderRequest(context.Background(), BuildProviderRequestResult{Method: "POST", URL: server.URL, Headers: http.Header{}, Body: `{}`, ModelVersion: "fake-model"}, gen.Provider{Adapter: gen.ProviderAdapter(adapter), ConnectTimeoutMs: 1000, RequestTimeoutMs: 5000, StreamIdleTimeoutMs: 1000}, "chat", nil, func(piece string) error { text.WriteString(piece); return nil })
	return outcome, text.String(), err
}

func TestNativeAgentHTTPReadLoop(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
				CallID  string `json:"tool_call_id"`
				Calls   []struct {
					ID string `json:"id"`
				} `json:"tool_calls"`
			} `json:"messages"`
			Tools []any `json:"tools"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || len(request.Tools) != 2 {
			http.Error(w, "原生工具请求丢失", 400)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		requests++
		if requests == 3 {
			last := request.Messages[len(request.Messages)-1]
			if last.Role != "tool" || last.CallID != "c2" || !strings.Contains(last.Content, "citation.smoke") {
				http.Error(w, "正文结果没有配对回传", 400)
				return
			}
			fmt.Fprint(w, "data: {\"id\":\"fake\",\"choices\":[{\"delta\":{\"content\":\"答案 [[citation.smoke]]\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		name, id, arguments := "read_guide", "c1", `{}`
		if requests == 2 {
			last := request.Messages[len(request.Messages)-1]
			if last.Role != "tool" || last.CallID != "c1" {
				http.Error(w, "目录结果没有配对回传", 400)
				return
			}
			name, id, arguments = "read_document", "c2", `{"documentId":"doc"}`
		}
		frame := map[string]any{"id": "fake", "choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": id, "type": "function", "function": map[string]any{"name": name, "arguments": arguments}}}}, "finish_reason": "tool_calls"}}}
		raw, _ := json.Marshal(frame)
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", raw)
	}))
	defer server.Close()
	tools := []platform.GatewayTool{{Name: "read_guide", Description: "公开目录", Parameters: map[string]any{"type": "object"}}, {Name: "read_document", Description: "公开正文", Parameters: map[string]any{"type": "object"}}}
	executed := 0
	result, err := modelagent.Run(context.Background(), modelagent.Config{
		Messages: []platform.GatewayMessage{{Role: "user", Content: "请阅读文档再回答"}}, Tools: tools, MaximumRounds: 4, MaximumToolCalls: 4, MaximumInputTokens: 12000, MaximumToolResultBytes: 8000,
		Invoke: func(ctx context.Context, round int, messages []platform.GatewayMessage, choice string) (platform.ModelOutput, error) {
			if err := platform.ValidateToolConversation(messages, tools, choice); err != nil {
				return platform.ModelOutput{}, err
			}
			raw, _ := json.Marshal(platform.CreateInvocationInput{Messages: messages, Tools: tools, ToolChoice: choice})
			var input gen.CreateInvocationInput
			if err := json.Unmarshal(raw, &input); err != nil {
				return platform.ModelOutput{}, err
			}
			payload, err := mapOpenAIChatPayload(input, "fake", 200, nil)
			if err != nil {
				return platform.ModelOutput{}, err
			}
			body, _ := json.Marshal(payload)
			outcome, err := SendProviderRequest(ctx, BuildProviderRequestResult{Method: "POST", URL: server.URL, Headers: http.Header{}, Body: string(body)}, gen.Provider{Adapter: "openai_chat", ConnectTimeoutMs: 1000, RequestTimeoutMs: 5000}, "chat", nil, func(string) error { return nil })
			if err != nil {
				return platform.ModelOutput{}, err
			}
			return *outcome.Output, nil
		},
		Execute: func(_ context.Context, call platform.GatewayToolCall, _ int) (modelagent.ToolResult, error) {
			executed++
			if call.Name == "read_guide" {
				return modelagent.ToolResult{Content: `{"markdown":"文档ID：doc"}`}, nil
			}
			return modelagent.ToolResult{Content: `{"sections":[{"citationId":"citation.smoke","text":"只读正文"}]}`}, nil
		},
	})
	if err != nil || requests != 3 || executed != 2 || result.Output.Text == nil || *result.Output.Text != "答案 [[citation.smoke]]" {
		t.Fatalf("原生HTTP工具循环没有走通：请求=%d，读取=%d，错误=%v", requests, executed, err)
	}
}

func TestOpenAIToolArgumentsAcrossFrames(t *testing.T) {
	frames := []string{
		`{"id":"fake","choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"search_documents","arguments":"{\"query\":\"访问"}}]}}]}`,
		`{"id":"fake","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"权限\"}"}}]}}]}`,
		`{"id":"fake","choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":10}}`,
	}
	outcome, text, err := sendToolStream(t, "openai_chat", frames, true)
	if err != nil {
		t.Fatal(err)
	}
	if text != "" || outcome.Output == nil || outcome.Output.FinishReason != "tool_calls" || len(outcome.Output.ToolCalls) != 1 || outcome.Output.ToolCalls[0].Arguments != `{"query":"访问权限"}` {
		t.Fatal("工具参数被当成答案或没有完整拼接")
	}
	frames[2] = `{"id":"fake","choices":[{"delta":{},"finish_reason":"length"}]}`
	if _, _, err := sendToolStream(t, "openai_chat", frames, true); err == nil {
		t.Fatal("截断的工具调用仍被接受")
	}
}

func TestAnthropicToolInputDeltas(t *testing.T) {
	frames := []string{
		`{"type":"message_start","message":{"id":"fake","usage":{"input_tokens":10,"output_tokens":0}}}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"c1","name":"read_document","input":{}}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"documentId\":"}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"doc\"}"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":10}}`,
		`{"type":"message_stop"}`,
	}
	outcome, text, err := sendToolStream(t, "anthropic_messages", frames, false)
	if err != nil {
		t.Fatal(err)
	}
	if text != "" || len(outcome.Output.ToolCalls) != 1 || outcome.Output.ToolCalls[0].Arguments != `{"documentId":"doc"}` {
		t.Fatal("Messages工具参数片段丢失或混入答案")
	}
}

func TestGeminiToolSignatureAndParallelCalls(t *testing.T) {
	frame := `{"responseId":"fake","candidates":[{"content":{"parts":[{"text":"内部思考","thought":true},{"functionCall":{"name":"read_document","args":{"documentId":"one"}},"thoughtSignature":"opaque-signature"},{"functionCall":{"name":"read_document","args":{"documentId":"two"}}}]},"finishReason":"STOP"}]} `
	outcome, text, err := sendToolStream(t, "gemini", []string{frame}, false)
	if err != nil {
		t.Fatal(err)
	}
	if text != "" || len(outcome.Output.ToolCalls) != 2 || outcome.Output.ToolCalls[0].Signature != "opaque-signature" || outcome.Output.ToolCalls[0].ID == outcome.Output.ToolCalls[1].ID {
		t.Fatal("Gemini签名、调用身份或思考隔离错误")
	}
	second, _, err := sendToolStream(t, "gemini", []string{frame}, false)
	if err != nil || second.Output.ToolCalls[0].ID == outcome.Output.ToolCalls[0].ID {
		t.Fatal("后续轮次复用了已有工具调用ID")
	}
}

func TestNativeRequestsReturnToolResultsInPairs(t *testing.T) {
	tools := []gen.GatewayTool{{Name: "read_document", Description: "读取正文", Parameters: map[string]any{"type": "object", "properties": map[string]any{}}}}
	signature := "opaque-signature"
	calls := []gen.GatewayToolCall{{Id: "c1", Name: "read_document", Arguments: `{"documentId":"one"}`, Signature: &signature}, {Id: "c2", Name: "read_document", Arguments: `{"documentId":"two"}`}}
	one, two := "c1", "c2"
	messages := []gen.GatewayMessage{{Role: "user", Content: "比较文章"}, {Role: "assistant", ToolCalls: &calls}, {Role: "tool", ToolCallId: &one, Content: `{"text":"第一篇"}`}, {Role: "tool", ToolCallId: &two, Content: `{"text":"第二篇"}`}}
	value, err := MapAnthropicRequest(context.Background(), OperationContext{}, MapAnthropicRequestInput{Messages: messages, ModelID: "fake", MaximumOutputTokens: 100, Tools: &tools})
	if err != nil {
		t.Fatal(err)
	}
	var anthropic struct {
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				ID   string `json:"tool_use_id"`
			} `json:"content"`
		} `json:"messages"`
		Tools []any `json:"tools"`
	}
	if json.Unmarshal([]byte(value.Body), &anthropic) != nil || len(anthropic.Messages) != 3 || len(anthropic.Messages[2].Content) != 2 || anthropic.Messages[2].Content[0].ID != "c1" || anthropic.Messages[2].Content[1].ID != "c2" || len(anthropic.Tools) != 1 {
		t.Fatal("Messages并行工具结果没有成组回传")
	}
	gemini, err := MapGeminiRequest(context.Background(), OperationContext{}, MapGeminiRequestInput{Messages: messages, ModelID: "fake", Capability: "chat", MaximumOutputTokens: 100, Tools: &tools})
	if err != nil {
		t.Fatal(err)
	}
	var google struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Signature string `json:"thoughtSignature"`
				Response  *struct {
					Name string `json:"name"`
				} `json:"functionResponse"`
			} `json:"parts"`
		} `json:"contents"`
	}
	if json.Unmarshal([]byte(gemini.Body), &google) != nil || len(google.Contents) != 3 || google.Contents[1].Parts[0].Signature != signature || len(google.Contents[2].Parts) != 2 || google.Contents[2].Parts[1].Response.Name != "read_document" {
		t.Fatal("Gemini没有保留工具签名或结果名称")
	}
	if err := platform.ValidateToolCalls([]platform.GatewayToolCall{{ID: "c1", Name: "read_document", Arguments: calls[0].Arguments}}); err != nil {
		t.Fatal(err)
	}
}
