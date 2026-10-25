package answer

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelagent"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"strconv"
)

type AgentAnswerInput struct {
	TurnID       string
	TaskID       string
	ProfileID    string
	Language     string
	Policy       gen.TypesQaPolicy
	Prompt       BuildAnswerPromptResult
	Citations    []gen.Citation
	DisableTools bool
	OnModelStart func(context.Context, int) error
	OnModelDone  func(context.Context, int, platform.ModelOutput) error
	OnToolRound  func(context.Context, platform.ModelOutput) error
	OnToolStart  func(context.Context, platform.GatewayToolCall) error
	OnToolDone   func(context.Context, platform.GatewayToolCall, modelagent.ToolResult) error
}

type AgentAnswerResult struct {
	Text      string
	Citations []gen.Citation
	Messages  []platform.GatewayMessage
	Rounds    int
	ToolCalls int
}

// GenerateAgentAnswer 装配本域工具与问答费用身份；通用循环、参数完整性和窗口管理交平台。
func GenerateAgentAnswer(ctx context.Context, op OperationContext, input AgentAnswerInput) (AgentAnswerResult, error) {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return AgentAnswerResult{}, err
	}
	if len(input.Prompt.Messages) > 0 {
		input.Prompt.Messages[0].Content += "\n本站基准语言为" + runtime.BaseLanguage + "。检索时优先使用基准语言和原文产品名，最终回答使用访客语言" + input.Language + "。"
	}
	window, err := qaruntime.ResolveTaskWindow(ctx, op, input.TaskID)
	if err != nil {
		return AgentAnswerResult{}, err
	}
	maximumOutput := min(input.Prompt.MaximumOutputTokens, input.Policy.MaximumOutputTokens)
	budget := window*(100-input.Policy.ContextReservePercent)/100 - maximumOutput - platform.EstimateMessageTokens([]platform.GatewayMessage{{Role: "user", Content: answerReminder}}) - platform.EstimateInputTokens(agentReminderExtraBytes, 1)
	if budget < 4096 {
		return AgentAnswerResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "回答模型的可用窗口不足以查阅文档")
	}
	citations := map[string]gen.Citation{}
	order := []string{}
	addCitations := func(items []gen.Citation) {
		for _, item := range items {
			if _, exists := citations[item.CitationId]; !exists {
				order = append(order, item.CitationId)
			}
			citations[item.CitationId] = item
		}
	}
	addCitations(input.Citations)
	riskReported := false
	result, err := modelagent.Run(ctx, modelagent.Config{
		Messages: input.Prompt.Messages, Tools: AgentTools(), MaximumRounds: input.Policy.MaximumRetrievalRounds, MaximumToolCalls: input.Policy.MaximumToolCalls,
		MaximumInputTokens: budget, MaximumToolResultBytes: min(56000, max(4096, budget/3)), DisableTools: input.DisableTools,
		CompactToolResult: compactAgentToolResult,
		Invoke: func(ctx context.Context, round int, messages []platform.GatewayMessage, choice string) (platform.ModelOutput, error) {
			if input.OnModelStart != nil {
				if err := input.OnModelStart(ctx, round+1); err != nil {
					return platform.ModelOutput{}, err
				}
			}
			// 工具正文同样可能夹带指令；每次查阅后重申固定边界，不把提醒写进会话历史。
			if round > 0 {
				messages = append(append([]platform.GatewayMessage{}, messages...), agentRoundReminder(messages))
			}
			outcome, err := qaruntime.InvokeModel(ctx, op, qaruntime.InvokeModelInput{Stream: true, Request: platform.CreateInvocationInput{TaskID: input.TaskID, ProfileID: input.ProfileID, Capability: platform.CapabilityChat, Messages: messages, MaximumOutputTokens: int64(maximumOutput), Tools: AgentTools(), ToolChoice: choice, IDempotencyKey: "answer.agent." + strconv.Itoa(round)}})
			if err != nil {
				return platform.ModelOutput{}, err
			}
			if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil {
				return platform.ModelOutput{}, platform.NewError(platform.CodeDependencyUnavailable, false, "agent原模型调用尚无完整持久结果")
			}
			output := *outcome.CurrentSnapshot.Output
			if input.OnModelDone != nil {
				if err := input.OnModelDone(ctx, round+1, output); err != nil {
					return platform.ModelOutput{}, err
				}
			}
			return output, nil
		},
		Execute: func(ctx context.Context, call platform.GatewayToolCall, maximum int) (modelagent.ToolResult, error) {
			if call.Name == "report_abuse" {
				if riskReported {
					return modelagent.ToolResult{Content: `{"reported":false,"message":"本轮已经上报，不重复计数。"}`, Title: "上报滥用信号"}, nil
				}
			}
			tool, err := ExecuteAgentTool(ctx, op, call, AgentToolInput{TurnID: input.TurnID, Language: input.Language, Policy: input.Policy, MaximumBytes: maximum})
			if err != nil {
				if ctx.Err() != nil {
					return modelagent.ToolResult{}, ctx.Err()
				}
				tool = AgentToolError(err)
			}
			if err == nil && call.Name == "report_abuse" {
				riskReported = true
			}
			addCitations(tool.Citations)
			return modelagent.ToolResult{Content: tool.Content, Title: tool.Title, IsError: err != nil}, nil
		},
		OnToolRound: input.OnToolRound, OnToolStart: input.OnToolStart, OnToolDone: input.OnToolDone,
	})
	if err != nil {
		return AgentAnswerResult{}, err
	}
	allowed := make([]gen.Citation, 0, len(order))
	for _, id := range order {
		allowed = append(allowed, citations[id])
	}
	// 本轮当前页预读也属于可复用的工具过程；公共系统提示和先前轮次另由上下文装配。
	messages := result.Messages[input.Prompt.TurnMessageStart:]
	return AgentAnswerResult{Text: normalizeCitationSpelling(*result.Output.Text), Citations: allowed, Messages: messages, Rounds: result.Rounds, ToolCalls: result.ToolCalls}, nil
}

// compactAgentToolResult 保留旧正文分页的首尾事实及原引用，避免读到结尾后把开头的前提条件完全移出窗口。
// 只是本次模型输入中的节选，完整工具结果仍按原样保存；不调用额外模型，也不把省略当作没有内容。
func compactAgentToolResult(content string, maximum int) string {
	var page agentDocumentPage
	if json.Unmarshal([]byte(content), &page) != nil || page.DocumentID == "" || len(page.Sections) == 0 {
		return content
	}
	sections := page.Sections
	if len(sections) > 4 {
		sections = append(append([]agentSection{}, sections[:2]...), sections[len(sections)-2:]...)
	}
	for characters := 384; characters >= 24; characters /= 2 {
		kept := make([]agentSection, 0, len(sections))
		for _, section := range sections {
			text := []rune(section.Text)
			if len(text) > characters {
				text = []rune(string(text[:characters*2/3]) + "\n（中间内容省略）\n" + string(text[len(text)-characters/3:]))
			}
			kept = append(kept, agentSection{CitationID: section.CitationID, Text: string(text)})
		}
		raw, _ := json.Marshal(map[string]any{"contextCompacted": true, "documentId": page.DocumentID, "publishedRevisionId": page.Revision, "offset": page.Offset, "title": clipAgentText(page.Title, 80), "sections": kept, "message": "仅保留这次已读正文的首尾节选，省略部分需要按原文档、版本和offset重新读取；不能把节选当作全文。"})
		if len(raw) <= maximum {
			return string(raw)
		}
	}
	return content
}
