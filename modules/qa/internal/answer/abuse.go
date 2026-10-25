package answer

import (
	"context"
	"nice-wiki/libs/platform"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

// reportAgentAbuse 仅上报结构化判断；会话和网络来自已消费许可，不接受模型指定封禁对象或时长。
func reportAgentAbuse(ctx context.Context, op OperationContext, turnID, verdict string) (AgentToolResult, error) {
	if verdict != "suspicious" && verdict != "abuse" {
		return AgentToolResult{}, platform.NewError(platform.CodeValidation, false, "滥用信号只能为suspicious或abuse")
	}
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return AgentToolResult{}, err
	}
	if runtime.Guard == nil || runtime.Permit() == nil {
		return AgentToolResult{Content: `{"reported":false,"message":"诊断模式不处罚真实访客"}`, Title: "上报滥用信号"}, nil
	}
	read, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return AgentToolResult{}, err
	}
	record, err := qaruntime.ReadTurn(read, op, turnID, false)
	read.Close()
	if err != nil {
		return AgentToolResult{}, err
	}
	raw, err := runtime.Guard.ReportModelRisk(qaruntime.ServiceReadContext(ctx, op), struct {
		SessionKey   string `json:"sessionKey"`
		NetworkKey   string `json:"networkKey"`
		QuestionHash string `json:"questionHash"`
		ModelVerdict string `json:"modelVerdict"`
	}{runtime.Permit().SessionBudgetScope, runtime.Permit().NetworkBudgetScope, record.Turn.QuestionHash, verdict}, "qa.agent-risk."+turnID)
	if err != nil {
		return AgentToolResult{}, err
	}
	var result struct {
		Decision     string `json:"decision"`
		BanExpiresAt string `json:"banExpiresAt,omitempty"`
	}
	if err := qaruntime.DecodeResponse(runtime, "GuardReportQuestionSignalsResult", raw, &result); err != nil {
		return AgentToolResult{}, err
	}
	// 不把网络、准入信息或内部处罚参数交给模型和公开活动流。
	return AgentToolResult{Content: `{"reported":true,"message":"信号已交给服务端风控；继续按本站公开文档范围回应。"}`, Title: "上报滥用信号"}, nil
}
