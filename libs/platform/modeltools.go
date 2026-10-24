package platform

import (
	"encoding/json"
	"regexp"
	"unicode/utf8"
)

var gatewayToolName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidateToolCalls 只接受可完整解码的参数对象；工具参数仍由执行域按各工具合同检查。
func ValidateToolCalls(calls []GatewayToolCall) error {
	if len(calls) > 32 {
		return NewError(CodeValidation, false, "一次最多请求32个工具调用")
	}
	seen := map[string]bool{}
	for _, call := range calls {
		var arguments map[string]json.RawMessage
		if call.ID == "" || len(call.ID) > 256 || !utf8.ValidString(call.ID) || seen[call.ID] || !gatewayToolName.MatchString(call.Name) || len(call.Arguments) > 65536 || !utf8.ValidString(call.Arguments) || json.Unmarshal([]byte(call.Arguments), &arguments) != nil || arguments == nil || len(call.Signature) > 65536 {
			return NewError(CodeValidation, false, "工具调用身份、名称、完整参数对象或签名不合法")
		}
		seen[call.ID] = true
	}
	return nil
}

// ValidateToolConversation 在模型网关入口检查工具声明和调用/结果配对，防止丢结果或混入伪造结果。
func ValidateToolConversation(messages []GatewayMessage, tools []GatewayTool, choice string) error {
	if len(tools) > 32 || (choice != "" && choice != "auto" && choice != "none" && choice != "required") || (choice != "" && len(tools) == 0) {
		return NewError(CodeValidation, false, "工具声明或调用方式不合法")
	}
	names := map[string]bool{}
	for _, tool := range tools {
		raw, err := json.Marshal(tool.Parameters)
		if !gatewayToolName.MatchString(tool.Name) || names[tool.Name] || tool.Description == "" || len(tool.Description) > 8192 || err != nil || len(raw) > 65536 || tool.Parameters["type"] != "object" {
			return NewError(CodeValidation, false, "工具名称重复或参数声明不是有界JSON对象schema")
		}
		names[tool.Name] = true
	}
	pending := map[string]bool{}
	ids := map[string]bool{}
	for _, message := range messages {
		if len(message.Content) > 65536 || !utf8.ValidString(message.Content) {
			return NewError(CodeValidation, false, "消息文本超过UTF8字节边界")
		}
		if len(pending) > 0 && message.Role != "tool" {
			return NewError(CodeValidation, false, "工具调用后必须先回传全部结果")
		}
		if message.Role == "tool" {
			if !pending[message.ToolCallID] || len(message.ToolCalls) > 0 || len(message.Parts) > 0 || message.Content == "" {
				return NewError(CodeValidation, false, "工具结果缺少对应调用或携带了媒体/新调用")
			}
			delete(pending, message.ToolCallID)
			continue
		}
		if message.ToolCallID != "" || (message.Role != "assistant" && len(message.ToolCalls) > 0) || (message.Content == "" && len(message.ToolCalls) == 0 && len(message.Parts) == 0) {
			return NewError(CodeValidation, false, "普通消息不能伪造工具结果或空内容")
		}
		if err := ValidateToolCalls(message.ToolCalls); err != nil {
			return err
		}
		for _, call := range message.ToolCalls {
			if !names[call.Name] || ids[call.ID] {
				return NewError(CodeValidation, false, "工具调用不属于声明或调用ID重复")
			}
			pending[call.ID], ids[call.ID] = true, true
		}
	}
	if len(pending) != 0 {
		return NewError(CodeValidation, false, "本次请求缺少工具调用的结果")
	}
	return nil
}
