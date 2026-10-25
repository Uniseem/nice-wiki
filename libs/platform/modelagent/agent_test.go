package modelagent

import (
	"context"
	"errors"
	"nice-wiki/libs/platform"
	"strconv"
	"strings"
	"testing"
)

func agentTestConfig() Config {
	return Config{Messages: []platform.GatewayMessage{{Role: "system", Content: "只读问答"}, {Role: "user", Content: "请比较两篇文章"}}, Tools: []platform.GatewayTool{{Name: "read_document", Description: "读取公开正文", Parameters: map[string]any{"type": "object"}}}, MaximumRounds: 8, MaximumToolCalls: 16, MaximumInputTokens: 6500, MaximumToolResultBytes: 16000}
}

func agentFinal(text string) platform.ModelOutput {
	return platform.ModelOutput{Capability: "chat", Text: &text, FinishReason: "stop"}
}

func TestAgentReadsAndMakesRoomForLatestResult(t *testing.T) {
	config := agentTestConfig()
	executed := 0
	config.Execute = func(context.Context, platform.GatewayToolCall, int) (ToolResult, error) {
		executed++
		return ToolResult{Content: strings.Repeat("材", 2000)}, nil
	}
	config.Invoke = func(_ context.Context, round int, messages []platform.GatewayMessage, choice string) (platform.ModelOutput, error) {
		if round == 0 {
			return platform.ModelOutput{Capability: "chat", FinishReason: "tool_calls", ToolCalls: []platform.GatewayToolCall{{ID: "c1", Name: "read_document", Arguments: `{"documentId":"one"}`}, {ID: "c2", Name: "read_document", Arguments: `{"documentId":"two"}`}}}, nil
		}
		if len(messages) != 5 || messages[3].ToolCallID != "c1" || !strings.Contains(messages[3].Content, "contextEvicted") || messages[4].ToolCallID != "c2" || strings.Contains(messages[4].Content, "contextEvicted") {
			t.Fatal("工具配对或最新正文在窗口整理中丢失")
		}
		return agentFinal("比较完成"), nil
	}
	result, err := Run(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if executed != 2 || result.ToolCalls != 2 || *result.Output.Text != "比较完成" || strings.Contains(result.Messages[3].Content, "contextEvicted") {
		t.Fatal("查阅执行或完整原过程保存错误")
	}
}

func TestOversizedToolBatchCanBeSplitWithoutExecutingIt(t *testing.T) {
	config := agentTestConfig()
	executed := 0
	config.Execute = func(context.Context, platform.GatewayToolCall, int) (ToolResult, error) {
		executed++
		return ToolResult{Content: "公开正文"}, nil
	}
	config.Invoke = func(_ context.Context, round int, messages []platform.GatewayMessage, choice string) (platform.ModelOutput, error) {
		if round == 0 {
			calls := []platform.GatewayToolCall{}
			for i := 0; i < 4; i++ {
				calls = append(calls, platform.GatewayToolCall{ID: "batch" + strconv.Itoa(i), Name: "read_document", Arguments: `{"documentId":"` + strconv.Itoa(i) + `"}`})
			}
			return platform.ModelOutput{FinishReason: "tool_calls", ToolCalls: calls}, nil
		}
		if round == 1 {
			if executed != 0 || !strings.Contains(messages[len(messages)-1].Content, "TOOL_BATCH_WINDOW") {
				t.Fatal("过大批次被执行或没有返回拆批提示")
			}
			return platform.ModelOutput{FinishReason: "tool_calls", ToolCalls: []platform.GatewayToolCall{{ID: "split", Name: "read_document", Arguments: `{"documentId":"0"}`}}}, nil
		}
		return agentFinal("完成"), nil
	}
	result, err := Run(context.Background(), config)
	if err != nil || executed != 1 || result.ToolCalls != 5 {
		t.Fatalf("拆批没有正常继续：executed=%d calls=%d err=%v", executed, result.ToolCalls, err)
	}
}

func TestAgentStopsOnCancellationAndUnknownTools(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		config := agentTestConfig()
		ctx, cancel := context.WithCancel(context.Background())
		executed := 0
		config.Execute = func(context.Context, platform.GatewayToolCall, int) (ToolResult, error) {
			executed++
			cancel()
			return ToolResult{Content: "公开内容"}, nil
		}
		config.Invoke = func(context.Context, int, []platform.GatewayMessage, string) (platform.ModelOutput, error) {
			name := "read_document"
			if unknown {
				name = "write_document"
			}
			return platform.ModelOutput{FinishReason: "tool_calls", ToolCalls: []platform.GatewayToolCall{{ID: "c1", Name: name, Arguments: `{}`}, {ID: "c2", Name: "read_document", Arguments: `{}`}}}, nil
		}
		_, err := Run(ctx, config)
		cancel()
		if unknown {
			if err == nil || executed != 0 {
				t.Fatal("未开放工具被执行")
			}
		} else if !errors.Is(err, context.Canceled) || executed != 1 {
			t.Fatal("取消后仍开始新工具")
		}
	}
}

func TestAgentReusesRepeatedReadsAndFinishes(t *testing.T) {
	config := agentTestConfig()
	executed, final := 0, false
	config.Execute = func(context.Context, platform.GatewayToolCall, int) (ToolResult, error) {
		executed++
		return ToolResult{Content: "相同公开内容"}, nil
	}
	config.Invoke = func(_ context.Context, round int, _ []platform.GatewayMessage, choice string) (platform.ModelOutput, error) {
		if choice == "none" {
			final = true
			return agentFinal("已有材料说明"), nil
		}
		return platform.ModelOutput{FinishReason: "tool_calls", ToolCalls: []platform.GatewayToolCall{{ID: "c" + strconv.Itoa(round), Name: "read_document", Arguments: `{}`}}}, nil
	}
	result, err := Run(context.Background(), config)
	if err != nil || executed != 1 || !final || result.Rounds != 5 {
		t.Fatalf("重复查阅未复用或没有收尾：执行=%d，轮数=%d，错误=%v", executed, result.Rounds, err)
	}
}
