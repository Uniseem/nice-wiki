// Package modelagent 提供有界的原生工具调用循环；业务域只提供只读工具及调用装配。
package modelagent

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"strings"
)

type ToolResult struct {
	Content string
	Title   string
	IsError bool
}

type Config struct {
	Messages               []platform.GatewayMessage
	Tools                  []platform.GatewayTool
	MaximumRounds          int
	MaximumToolCalls       int
	MaximumInputTokens     int
	MaximumToolResultBytes int
	CompactToolResult      func(string, int) string
	DisableTools           bool
	Invoke                 func(context.Context, int, []platform.GatewayMessage, string) (platform.ModelOutput, error)
	Execute                func(context.Context, platform.GatewayToolCall, int) (ToolResult, error)
	OnToolRound            func(context.Context, platform.ModelOutput) error
	OnToolStart            func(context.Context, platform.GatewayToolCall) error
	OnToolDone             func(context.Context, platform.GatewayToolCall, ToolResult) error
}

type Result struct {
	Output    platform.ModelOutput
	Messages  []platform.GatewayMessage
	Rounds    int
	ToolCalls int
}

// Run 保持工具调用与结果成对，工具只在完整响应后执行；重读复用本轮材料，重复空转后收尾。
func Run(ctx context.Context, config Config) (Result, error) {
	result := Result{Messages: append([]platform.GatewayMessage{}, config.Messages...)}
	if config.Invoke == nil || config.Execute == nil || config.MaximumRounds < 1 || config.MaximumRounds > 32 || config.MaximumToolCalls < 1 || config.MaximumToolCalls > 128 || config.MaximumInputTokens < 1 {
		return result, platform.NewError(platform.CodeValidation, false, "agent循环配置不完整")
	}
	if err := platform.ValidateToolConversation(config.Messages, config.Tools, "auto"); err != nil {
		return result, err
	}
	known := map[string]bool{}
	for _, tool := range config.Tools {
		known[tool.Name] = true
	}
	seenCalls := map[string]bool{}
	for _, message := range config.Messages {
		for _, call := range message.ToolCalls {
			seenCalls[call.ID] = true
		}
	}
	cache := map[string]ToolResult{}
	repeated := map[string]int{}
	stop := false
	for round := 0; round <= config.MaximumRounds; round++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		choice := "auto"
		if config.DisableTools || stop || round == config.MaximumRounds || result.ToolCalls >= config.MaximumToolCalls {
			choice = "none"
		}
		messages, err := fitMessages(result.Messages, config.Tools, config.MaximumInputTokens, config.CompactToolResult)
		if err != nil {
			return result, err
		}
		if choice == "none" && !config.DisableTools {
			messages = append(messages, platform.GatewayMessage{Role: "user", Content: "本轮查阅已结束。请根据已经取得的材料回答，说明尚未查明的部分，不再请求工具。"})
		}
		output, err := config.Invoke(ctx, round, messages, choice)
		if err != nil {
			return result, err
		}
		result.Rounds++
		if err := platform.ValidateToolCalls(output.ToolCalls); err != nil {
			return result, err
		}
		if len(output.ToolCalls) == 0 {
			if output.Text == nil || strings.TrimSpace(*output.Text) == "" {
				return result, platform.NewError(platform.CodeDependencyUnavailable, false, "agent没有返回完整答案或工具调用")
			}
			result.Output = output
			return result, nil
		}
		if choice == "none" || output.FinishReason != "tool_calls" {
			return result, platform.NewError(platform.CodeIntegrityError, false, "agent未遵守本轮工具结束边界")
		}
		for _, call := range output.ToolCalls {
			if !known[call.Name] {
				return result, platform.NewError(platform.CodeForbidden, false, "agent请求未开放的工具")
			}
			if seenCalls[call.ID] {
				return result, platform.NewError(platform.CodeIntegrityError, false, "agent复用了此前工具调用ID")
			}
			seenCalls[call.ID] = true
		}
		if config.OnToolRound != nil {
			if err := config.OnToolRound(ctx, output); err != nil {
				return result, err
			}
		}
		text := ""
		if output.Text != nil {
			text = *output.Text
		}
		result.Messages = append(result.Messages, platform.GatewayMessage{Role: "assistant", Content: text, ToolCalls: output.ToolCalls})
		batchBytes := max(4096, min(60000, config.MaximumToolResultBytes))
		batchTooLarge := len(output.ToolCalls) > max(1, batchBytes/4096)
		maximum := max(4096, batchBytes/len(output.ToolCalls))
		for _, call := range output.ToolCalls {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			if config.OnToolStart != nil {
				if err := config.OnToolStart(ctx, call); err != nil {
					return result, err
				}
			}
			var arguments any
			_ = json.Unmarshal([]byte(call.Arguments), &arguments)
			canonical, _ := platform.CanonicalJSON(arguments)
			key := call.Name + ":" + string(canonical)
			repeated[key]++
			tool, reused := cache[key]
			if reused && len(tool.Content) > maximum {
				tool = ToolResult{Content: `{"error":"TOOL_WINDOW","message":"本批工具读取过多，请减少同一批的调用数后重读。"}`, Title: "请分批读取", IsError: true}
			}
			if stop || result.ToolCalls >= config.MaximumToolCalls || repeated[key] > 3 {
				tool = ToolResult{Content: `{"error":"TOOL_LIMIT","message":"本轮已达到查阅边界，请用已有材料回答并说明未知部分。"}`, Title: "查阅结束"}
				stop = true
			} else if batchTooLarge {
				tool = ToolResult{Content: `{"error":"TOOL_BATCH_WINDOW","message":"本批工具数量超过可读窗口，请拆成更小的批次重新调用。"}`, Title: "请分批查阅", IsError: true}
			} else if !reused {
				tool, err = config.Execute(ctx, call, maximum)
				if err != nil {
					return result, err
				}
				if tool.Content == "" || len(tool.Content) > maximum {
					return result, platform.NewError(platform.CodeBudgetExceeded, false, "工具结果超过当前可读窗口")
				}
				if !tool.IsError {
					cache[key] = tool
				}
			}
			if result.ToolCalls < config.MaximumToolCalls {
				result.ToolCalls++
			}
			result.Messages = append(result.Messages, platform.GatewayMessage{Role: "tool", ToolCallID: call.ID, Content: tool.Content})
			if config.OnToolDone != nil {
				if err := config.OnToolDone(ctx, call, tool); err != nil {
					return result, err
				}
			}
		}
	}
	return result, platform.NewError(platform.CodeBudgetExceeded, false, "agent查阅未在本轮执行边界内完成")
}

// fitMessages 窗口不足时先由调用方保留有界的事实节选，再移出仍放不下的旧正文；原始消息不改写。
func fitMessages(messages []platform.GatewayMessage, tools []platform.GatewayTool, maximum int, compact func(string, int) string) ([]platform.GatewayMessage, error) {
	result := append([]platform.GatewayMessage{}, messages...)
	raw, _ := json.Marshal(tools)
	budget := maximum - platform.EstimateInputTokens(len(raw), 1) - 256
	estimate := func() int { return platform.EstimateMessageTokens(result) }
	if compact != nil && estimate() > budget {
		for maximum := 2048; maximum >= 512 && estimate() > budget; maximum /= 2 {
			for index := 0; index < len(result)-1 && estimate() > budget; index++ {
				if result[index].Role != "tool" {
					continue
				}
				shortened := compact(result[index].Content, maximum)
				if len(shortened) > 0 && len(shortened) <= maximum && len(shortened) < len(result[index].Content) {
					result[index].Content = shortened
				}
			}
		}
	}
	for index := 0; index < len(result)-1 && (estimate() > budget || len(result) > 250); index++ {
		if result[index].Role == "tool" {
			result[index].Content = `{"contextEvicted":true,"message":"旧工具正文已移出模型窗口，需要时按此前参数重新读取；不能把移出当作没有内容。"}`
		}
	}
	if len(result) > 250 || estimate() > budget {
		return nil, platform.NewError(platform.CodeBudgetExceeded, false, "当前模型窗口放不下阅读指南、问题和最新工具结果")
	}
	return result, nil
}
