package answer

import (
	"context"
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
	budget := window*(100-input.Policy.ContextReservePercent)/100 - maximumOutput
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
	result, err := modelagent.Run(ctx, modelagent.Config{
		Messages: input.Prompt.Messages, Tools: AgentTools(), MaximumRounds: input.Policy.MaximumRetrievalRounds, MaximumToolCalls: input.Policy.MaximumToolCalls,
		MaximumInputTokens: budget, MaximumToolResultBytes: min(56000, max(4096, budget/3)), DisableTools: input.DisableTools,
		Invoke: func(ctx context.Context, round int, messages []platform.GatewayMessage, choice string) (platform.ModelOutput, error) {
			outcome, err := qaruntime.InvokeModel(ctx, op, qaruntime.InvokeModelInput{Stream: true, Request: platform.CreateInvocationInput{TaskID: input.TaskID, ProfileID: input.ProfileID, Capability: platform.CapabilityChat, Messages: messages, MaximumOutputTokens: int64(maximumOutput), Tools: AgentTools(), ToolChoice: choice, IDempotencyKey: "answer.agent." + strconv.Itoa(round)}})
			if err != nil {
				return platform.ModelOutput{}, err
			}
			if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil {
				return platform.ModelOutput{}, platform.NewError(platform.CodeDependencyUnavailable, false, "agent原模型调用尚无完整持久结果")
			}
			return *outcome.CurrentSnapshot.Output, nil
		},
		Execute: func(ctx context.Context, call platform.GatewayToolCall, maximum int) (modelagent.ToolResult, error) {
			tool, err := ExecuteAgentTool(ctx, op, call, AgentToolInput{TurnID: input.TurnID, Language: input.Language, Policy: input.Policy, MaximumBytes: maximum})
			if err != nil {
				if ctx.Err() != nil {
					return modelagent.ToolResult{}, ctx.Err()
				}
				tool = AgentToolError(err)
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
