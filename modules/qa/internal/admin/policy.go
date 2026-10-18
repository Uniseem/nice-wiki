package admin

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type OperationContext = platform.OperationContext
type ReadQaPolicyInput struct{}
type ReadQaPolicyResult = gen.ReadQaPolicyResult
type ReadPublicCapabilityInput struct{}
type ReadPublicCapabilityResult = gen.ReadPublicCapabilityResult

func ReadQaPolicy(ctx context.Context, op OperationContext, input ReadQaPolicyInput) (ReadQaPolicyResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.manage"); err != nil {
		return ReadQaPolicyResult{}, err
	}
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return ReadQaPolicyResult{}, err
	}
	defer tx.Close()
	policy, err := qaruntime.ReadPolicy(tx)
	if err != nil {
		return ReadQaPolicyResult{}, err
	}
	return ReadQaPolicyResult{Policy: policy, Version: policy.Version}, nil
}

// ReadPublicCapability 只读固定三项配置可用性，未知不能降成已知未配置。
func ReadPublicCapability(ctx context.Context, op OperationContext, input ReadPublicCapabilityInput) (ReadPublicCapabilityResult, error) {
	if op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService || (op.CallerService != "site-config" && op.CallerService != "public-site") || platform.PortalProofFrom(ctx) != "" {
		return ReadPublicCapabilityResult{}, platform.NewError(platform.CodeForbidden, false, "不允许读取公开问答能力")
	}
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return ReadPublicCapabilityResult{}, err
	}
	runtime := tx.Runtime
	tx.Close()
	if runtime.Models == nil {
		return ReadPublicCapabilityResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "模型配置权威未装配")
	}
	tasks := []string{"qa.preflight", "qa.inspect", "qa.answer"}
	raw, err := runtime.Models.ReadTaskAvailability(qaruntime.ServiceReadContext(ctx, op), struct {
		Tasks []string `json:"taskIds"`
	}{tasks})
	if err != nil {
		return ReadPublicCapabilityResult{}, err
	}
	var response struct {
		Tasks []struct {
			TaskID    string `json:"taskId"`
			Available bool   `json:"available"`
			Reason    string `json:"reason"`
		} `json:"tasks"`
	}
	if err := qaruntime.DecodeResponse(runtime, "ModelTaskAvailabilityResult", raw, &response); err != nil {
		return ReadPublicCapabilityResult{}, err
	}
	if len(response.Tasks) != len(tasks) {
		return ReadPublicCapabilityResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "任务配置可用性回执不完整")
	}
	result := ReadPublicCapabilityResult{Available: true, Reason: gen.ReadPublicCapabilityResultReason("available")}
	for i, item := range response.Tasks {
		if item.TaskID != tasks[i] || item.Available != (item.Reason == "available") {
			return ReadPublicCapabilityResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "任务配置可用性回执不一致")
		}
		if !item.Available {
			result.Available = false
			if item.Reason == "not_configured" {
				result.Reason = gen.ReadPublicCapabilityResultReason("not_configured")
			} else if result.Reason != gen.ReadPublicCapabilityResultReason("not_configured") {
				result.Reason = gen.ReadPublicCapabilityResultReason("unavailable")
			}
		}
	}
	return result, nil
}
