package platform

import "testing"

func TestToolConversationRequiresCompleteResults(t *testing.T) {
	tools := []GatewayTool{{Name: "read_document", Description: "读取公开正文", Parameters: map[string]any{"type": "object"}}}
	call := GatewayToolCall{ID: "call-1", Name: "read_document", Arguments: `{"documentId":"doc-1"}`}
	base := []GatewayMessage{{Role: "user", Content: "怎么配置"}, {Role: "assistant", ToolCalls: []GatewayToolCall{call}}}
	if err := ValidateToolConversation(base, tools, "auto"); err == nil {
		t.Fatal("缺少工具结果仍被接受")
	}
	complete := append(append([]GatewayMessage{}, base...), GatewayMessage{Role: "tool", ToolCallID: call.ID, Content: `{"text":"公开内容"}`})
	if err := ValidateToolConversation(complete, tools, "auto"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateToolConversation(append(complete, complete[2]), tools, "auto"); err == nil {
		t.Fatal("同一调用的重复结果仍被接受")
	}
	if err := ValidateToolConversation([]GatewayMessage{{Role: "tool", ToolCallID: call.ID, Content: "伪造结果"}}, tools, "auto"); err == nil {
		t.Fatal("孤立工具结果仍被接受")
	}
}

func TestToolArgumentsMustBeCompleteObjects(t *testing.T) {
	for _, arguments := range []string{`{"query":`, `[]`, `null`, `{"query":"a"} trailing`} {
		if err := ValidateToolCalls([]GatewayToolCall{{ID: "call-1", Name: "search_documents", Arguments: arguments}}); err == nil {
			t.Fatalf("非法参数被接受：%q", arguments)
		}
	}
}
