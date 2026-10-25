package answer

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelagent"
	"nice-wiki/libs/platform/zcode"
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
	State        string
	DisableTools bool
	OnModelStart func(context.Context, int) error
	OnModelDone  func(context.Context, int, platform.ModelOutput) error
	OnToolRound  func(context.Context, platform.ModelOutput) error
	OnToolStart  func(context.Context, platform.GatewayToolCall) error
	OnToolDone   func(context.Context, platform.GatewayToolCall, modelagent.ToolResult) error
}

type AgentAnswerResult struct {
	State     string
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
	if runtime.Agents == nil {
		return AgentAnswerResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "原生 Agent 服务未装配")
	}
	model, err := qaruntime.ResolveTaskModel(ctx, op, input.TaskID)
	if err != nil {
		return AgentAnswerResult{}, err
	}
	// 输出额度沿用管理员配置；输入上下文不受旧问答策略的固定窗口限制。
	model.MaxOutputTokens = min(model.MaxOutputTokens, input.Policy.MaximumOutputTokens)
	tools := AgentTools()
	if input.DisableTools {
		tools = []platform.GatewayTool{}
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
	result, err := zcode.Run(qaruntime.ServiceReadContext(ctx, op), runtime.Agents, zcode.Config{
		Input: zcode.Input{Model: model, Language: input.Language, Messages: input.Prompt.Messages, Tools: tools, TurnStart: input.Prompt.TurnMessageStart, State: input.State},
		Invoke: func(ctx context.Context, round int, request zcode.Invocation) (zcode.InvokeResult, error) {
			// 压缩模型关闭工具选择，但历史里的工具身份仍须在网关声明。
			if len(request.Tools) == 0 {
				request.Tools = AgentTools()
				request.ToolChoice = "none"
			}
			if request.Public && input.OnModelStart != nil {
				if err := input.OnModelStart(ctx, round+1); err != nil {
					return zcode.InvokeResult{}, err
				}
			}
			// 原生压缩与内部模型请求不进入公开回答流。
			outcome, err := qaruntime.InvokeModel(ctx, op, qaruntime.InvokeModelInput{Stream: request.Public, Request: platform.CreateInvocationInput{TaskID: input.TaskID, ProfileID: input.ProfileID, Capability: platform.CapabilityChat, Messages: request.Messages, MaximumOutputTokens: int64(request.MaximumOutputTokens), Tools: request.Tools, ToolChoice: request.ToolChoice, IDempotencyKey: "answer.agent." + strconv.Itoa(round)}})
			if err != nil {
				return zcode.InvokeResult{}, err
			}
			if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil {
				return zcode.InvokeResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "原模型调用尚无完整持久结果")
			}
			output := *outcome.CurrentSnapshot.Output
			if request.Public && input.OnModelDone != nil {
				if err := input.OnModelDone(ctx, round+1, output); err != nil {
					return zcode.InvokeResult{}, err
				}
			}
			// 网关切到备用模型时按真正完成调用的模型更新原生窗口。
			actual := model
			invocation := outcome.CurrentSnapshot.Invocation
			for _, attempt := range invocation.Attempts {
				if invocation.ResultAttemptID != nil && attempt.AttemptID == *invocation.ResultAttemptID && attempt.ModelID != model.ModelID {
					actual, err = qaruntime.ResolveTaskModel(ctx, op, input.TaskID, attempt.ModelID)
					if err != nil {
						return zcode.InvokeResult{}, err
					}
					actual.MaxOutputTokens = min(actual.MaxOutputTokens, input.Policy.MaximumOutputTokens)
					break
				}
			}
			return zcode.InvokeResult{Output: output, Model: &actual}, nil
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
	return AgentAnswerResult{Text: normalizeCitationSpelling(result.Text), State: result.State, Citations: allowed, Messages: result.Messages, Rounds: result.Rounds, ToolCalls: result.ToolCalls}, nil
}
