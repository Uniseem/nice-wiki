package zcode

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelagent"
	"strconv"
	"strings"
	"time"
)

// Client 是生成的 agent 客户端所实现的版本化端口。
type Client interface {
	StartAgentRun(context.Context, any) (json.RawMessage, error)
	ReadAgentEvents(context.Context, string, ...url.Values) (json.RawMessage, error)
	ReplyAgentRequest(context.Context, string, string, any) (json.RawMessage, error)
	CancelAgentRun(context.Context, string, any) (json.RawMessage, error)
}

type Config struct {
	Input       Input
	Invoke      func(context.Context, int, Invocation) (InvokeResult, error)
	Execute     func(context.Context, platform.GatewayToolCall, int) (modelagent.ToolResult, error)
	OnToolRound func(context.Context, platform.ModelOutput) error
	OnToolStart func(context.Context, platform.GatewayToolCall) error
	OnToolDone  func(context.Context, platform.GatewayToolCall, modelagent.ToolResult) error
	OnEvent     func(context.Context, string) error
}

// Run 只转发原生运行时请求；决策、工具循环和压缩全部在 ZCode 服务执行。
func Run(ctx context.Context, client Client, config Config) (Result, error) {
	result := Result{Messages: append([]platform.GatewayMessage{}, config.Input.Messages[config.Input.TurnStart:]...)}
	raw, err := client.StartAgentRun(ctx, config.Input)
	if err != nil {
		return result, err
	}
	var started struct {
		RunID string `json:"runId"`
	}
	if json.Unmarshal(raw, &started) != nil || started.RunID == "" {
		return result, unavailable()
	}
	defer func() {
		op, _ := platform.OperationContextFrom(ctx)
		op.Deadline = time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339Nano)
		cleanup, cancel := context.WithTimeout(platform.WithOperationContext(context.WithoutCancel(ctx), op), 5*time.Second)
		defer cancel()
		_, _ = client.CancelAgentRun(cleanup, started.RunID, struct{}{})
	}()
	after := 0
	for {
		raw, err = client.ReadAgentEvents(ctx, started.RunID, url.Values{"afterSequence": {strconv.Itoa(after)}})
		if err != nil {
			return result, err
		}
		var events Events
		if json.Unmarshal(raw, &events) != nil {
			return result, unavailable()
		}
		for _, frame := range events.Frames {
			after = frame.Sequence
			var reply any
			switch frame.Kind {
			case "invoke":
				var request Invocation
				if json.Unmarshal([]byte(frame.Payload), &request) != nil {
					return result, unavailable()
				}
				output, callErr := config.Invoke(ctx, result.Rounds, request)
				if callErr != nil {
					var failure *platform.Error
					if errors.As(callErr, &failure) && failure.Code == platform.CodeBudgetExceeded && strings.Contains(failure.Message, "窗口") {
						// 把网关的窗口错误交给原生 reactive compact，而不另造重试循环。
						reply = map[string]string{"error": "context length exceeded"}
						result.Rounds++
						break
					}
					return result, callErr
				}
				result.Rounds++
				if len(output.Output.ToolCalls) > 0 {
					if request.Public && config.OnToolRound != nil {
						if err := config.OnToolRound(ctx, output.Output); err != nil {
							return result, err
						}
					}
					text := ""
					if output.Output.Text != nil {
						text = *output.Output.Text
					}
					result.Messages = append(result.Messages, platform.GatewayMessage{Role: "assistant", Content: text, ToolCalls: output.Output.ToolCalls})
				}
				reply = output
			case "tool":
				var request ToolRequest
				if json.Unmarshal([]byte(frame.Payload), &request) != nil {
					return result, unavailable()
				}
				if config.OnToolStart != nil {
					if err := config.OnToolStart(ctx, request.Call); err != nil {
						return result, err
					}
				}
				output, callErr := config.Execute(ctx, request.Call, request.MaximumBytes)
				if callErr != nil {
					return result, callErr
				}
				result.ToolCalls++
				result.Messages = append(result.Messages, platform.GatewayMessage{Role: "tool", ToolCallID: request.Call.ID, Content: output.Content})
				if config.OnToolDone != nil {
					if err := config.OnToolDone(ctx, request.Call, output); err != nil {
						return result, err
					}
				}
				reply = map[string]any{"content": output.Content, "isError": output.IsError}
			case "event":
				if config.OnEvent != nil {
					if err := config.OnEvent(ctx, frame.Payload); err != nil {
						return result, err
					}
				}
				continue
			case "done":
				var completed Result
				if json.Unmarshal([]byte(frame.Payload), &completed) != nil {
					return result, unavailable()
				}
				result.Text, result.State = completed.Text, completed.State
				return result, nil
			case "failed":
				var failure struct {
					Code string `json:"code"`
				}
				_ = json.Unmarshal([]byte(frame.Payload), &failure)
				if failure.Code == "CANCELLED" {
					return result, platform.NewError(platform.CodeCancelled, false, "原生 Agent 已取消")
				}
				return result, unavailable()
			default:
				return result, unavailable()
			}
			if _, err = client.ReplyAgentRequest(ctx, started.RunID, frame.ID, Reply{Payload: payload(reply)}); err != nil {
				return result, err
			}
		}
		if events.Done {
			return result, unavailable()
		}
	}
}
