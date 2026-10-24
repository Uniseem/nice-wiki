package provider

import (
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	"sort"
	"strconv"
)

// providerToolCall 同时承接OpenAI完整调用与流式参数片段；不在片段到达时执行。
type providerToolCall struct {
	Index    *int   `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func openAIToolCalls(calls *[]gen.GatewayToolCall) []map[string]any {
	result := []map[string]any{}
	if calls != nil {
		for _, call := range *calls {
			result = append(result, map[string]any{"id": call.Id, "type": "function", "function": map[string]any{"name": call.Name, "arguments": call.Arguments}})
		}
	}
	return result
}

func mapOpenAIChatPayload(request gen.CreateInvocationInput, model string, maximum int, images map[string][]byte) (map[string]any, error) {
	messages := make([]map[string]any, 0, len(request.Messages))
	for _, message := range request.Messages {
		var content any = message.Content
		if message.Parts != nil {
			parts, err := ProviderContent(message, images, "openai")
			if err != nil {
				return nil, err
			}
			content = parts
		}
		item := map[string]any{"role": message.Role, "content": content}
		if message.ToolCalls != nil {
			item["tool_calls"] = openAIToolCalls(message.ToolCalls)
		}
		if message.ToolCallId != nil {
			item["tool_call_id"] = *message.ToolCallId
		}
		messages = append(messages, item)
	}
	payload := map[string]any{"model": model, "messages": messages, "max_tokens": maximum, "stream": true, "stream_options": map[string]any{"include_usage": true}}
	addProviderTools(payload, request.Tools, request.ToolChoice, "openai")
	return payload, nil
}

func addProviderTools(payload map[string]any, tools *[]gen.GatewayTool, choice *gen.CreateInvocationInputToolChoice, protocol string) {
	if tools == nil || len(*tools) == 0 {
		return
	}
	declarations := []map[string]any{}
	for _, tool := range *tools {
		item := map[string]any{"name": tool.Name, "description": tool.Description}
		switch protocol {
		case "anthropic":
			item["input_schema"] = tool.Parameters
		case "gemini":
			item["parametersJsonSchema"] = tool.Parameters
		default:
			item["parameters"] = tool.Parameters
			item = map[string]any{"type": "function", "function": item}
		}
		declarations = append(declarations, item)
	}
	mode := "auto"
	if choice != nil {
		mode = string(*choice)
	}
	switch protocol {
	case "anthropic":
		if mode == "required" {
			mode = "any"
		}
		payload["tools"], payload["tool_choice"] = declarations, map[string]any{"type": mode}
	case "gemini":
		mapped := map[string]string{"auto": "AUTO", "none": "NONE", "required": "ANY"}
		payload["tools"] = []map[string]any{{"functionDeclarations": declarations}}
		payload["toolConfig"] = map[string]any{"functionCallingConfig": map[string]any{"mode": mapped[mode]}}
	default:
		payload["tools"], payload["tool_choice"] = declarations, mode
	}
}

func appendProviderMessage(messages []map[string]any, role string, content []map[string]any) []map[string]any {
	if len(messages) > 0 && messages[len(messages)-1]["role"] == role {
		last := messages[len(messages)-1]
		last["content"] = append(last["content"].([]map[string]any), content...)
		return messages
	}
	return append(messages, map[string]any{"role": role, "content": content})
}

func (outcome *ProviderOutcome) tool(index int) (*platform.GatewayToolCall, error) {
	if index < 0 || index > 255 {
		return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "工具调用索引越界")
	}
	if outcome.toolCalls == nil {
		outcome.toolCalls = map[int]*platform.GatewayToolCall{}
	}
	if call := outcome.toolCalls[index]; call != nil {
		return call, nil
	}
	if len(outcome.toolCalls) >= 32 {
		return nil, platform.NewError(platform.CodeBudgetExceeded, false, "一次工具调用超过32项")
	}
	call := &platform.GatewayToolCall{}
	outcome.toolCalls[index] = call
	return call, nil
}

func (outcome *ProviderOutcome) consumeOpenAITools(items []providerToolCall, streamed bool) error {
	for position, item := range items {
		index := position
		if item.Index != nil {
			index = *item.Index
		}
		call, err := outcome.tool(index)
		if err != nil {
			return err
		}
		if item.Type != "" && item.Type != "function" {
			return platform.NewError(platform.CodeUnsupportedSchema, false, "只支持函数工具调用")
		}
		if item.ID != "" {
			if call.ID != "" && call.ID != item.ID {
				return platform.NewError(platform.CodeIntegrityError, false, "同索引工具调用ID变化")
			}
			call.ID = item.ID
		}
		if streamed {
			call.Name += item.Function.Name
			call.Arguments += item.Function.Arguments
		} else {
			call.Name, call.Arguments = item.Function.Name, item.Function.Arguments
		}
		if len(call.Name) > 64 || len(call.Arguments) > 65536 {
			return platform.NewError(platform.CodeBudgetExceeded, false, "工具名称或参数片段超界")
		}
	}
	return nil
}

func normalizeFinishReason(reason string) string {
	switch reason {
	case "tool_calls", "tool_use":
		return "tool_calls"
	case "length", "max_tokens", "MAX_TOKENS":
		return "length"
	case "content_filter", "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT":
		return "content_filter"
	case "stop", "end_turn", "stop_sequence", "STOP":
		return "stop"
	}
	return ""
}

func (outcome *ProviderOutcome) completedTools() ([]platform.GatewayToolCall, error) {
	indices := []int{}
	for index := range outcome.toolCalls {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	calls := []platform.GatewayToolCall{}
	for _, index := range indices {
		call := *outcome.toolCalls[index]
		if call.Arguments == "" {
			call.Arguments = "{}"
		}
		calls = append(calls, call)
	}
	if len(calls) > 0 && outcome.finishReason != "tool_calls" {
		return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "工具参数缺少正常结束证明")
	}
	if outcome.finishReason == "tool_calls" && len(calls) == 0 {
		return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "工具结束帧缺少调用")
	}
	if err := platform.ValidateToolCalls(calls); err != nil {
		return nil, err
	}
	return calls, nil
}

func (outcome *ProviderOutcome) geminiCallID(id string, index int) string {
	if id != "" {
		return id
	}
	if outcome.toolPrefix == "" {
		outcome.toolPrefix = platform.NewIdentifier()
	}
	return "gemini.call." + outcome.toolPrefix + "." + strconv.Itoa(index)
}

func (outcome *ProviderOutcome) consumeNativeTools(frame providerFrame, adapter string) error {
	for _, choice := range frame.Choices {
		if err := outcome.consumeOpenAITools(choice.Delta.ToolCalls, true); err != nil {
			return err
		}
		if err := outcome.consumeOpenAITools(choice.Message.ToolCalls, false); err != nil {
			return err
		}
		if choice.FinishReason != nil {
			outcome.finishReason = normalizeFinishReason(*choice.FinishReason)
		}
	}
	if block := frame.ContentBlock; block != nil && block.Type == "tool_use" {
		if frame.Index == nil {
			return platform.NewError(platform.CodeUnsupportedSchema, false, "工具块缺少索引")
		}
		call, err := outcome.tool(*frame.Index)
		if err != nil {
			return err
		}
		call.ID, call.Name = block.ID, block.Name
		if len(block.Input) > 0 && string(block.Input) != "{}" {
			call.Arguments = string(block.Input)
		}
	}
	if frame.Delta.PartialJSON != nil {
		if frame.Index == nil || outcome.toolCalls[*frame.Index] == nil {
			return platform.NewError(platform.CodeUnsupportedSchema, false, "工具参数片段缺少起始调用")
		}
		call := outcome.toolCalls[*frame.Index]
		call.Arguments += *frame.Delta.PartialJSON
		if len(call.Arguments) > 65536 {
			return platform.NewError(platform.CodeBudgetExceeded, false, "工具参数片段超过64KiB")
		}
	}
	for index, block := range frame.Content {
		if block.Type != "tool_use" {
			continue
		}
		call, err := outcome.tool(index)
		if err != nil {
			return err
		}
		call.ID, call.Name, call.Arguments = block.ID, block.Name, string(block.Input)
	}
	for _, reason := range []*string{frame.StopReason, frame.Delta.StopReason} {
		if reason != nil {
			outcome.finishReason = normalizeFinishReason(*reason)
		}
	}
	for _, candidate := range frame.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall == nil {
				continue
			}
			index := len(outcome.toolCalls)
			call, err := outcome.tool(index)
			if err != nil {
				return err
			}
			call.ID, call.Name, call.Arguments, call.Signature = outcome.geminiCallID(part.FunctionCall.ID, index), part.FunctionCall.Name, string(part.FunctionCall.Args), part.ThoughtSignature
		}
		if candidate.FinishReason != "" {
			outcome.finishReason = normalizeFinishReason(candidate.FinishReason)
			if adapter == "gemini" && outcome.finishReason == "stop" && len(outcome.toolCalls) > 0 {
				outcome.finishReason = "tool_calls"
			}
		}
	}
	return nil
}

func decodeToolArguments(raw string) (map[string]any, error) {
	var input map[string]any
	if json.Unmarshal([]byte(raw), &input) != nil || input == nil {
		return nil, platform.NewError(platform.CodeValidation, false, "工具参数不是JSON对象")
	}
	return input, nil
}
