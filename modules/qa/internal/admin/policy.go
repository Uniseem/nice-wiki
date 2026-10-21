package admin

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"strconv"
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
	policy, policyErr := qaruntime.ReadPolicy(tx)
	tx.Close()
	if policyErr != nil {
		return ReadPublicCapabilityResult{}, policyErr
	}
	if !policy.Enabled {
		return ReadPublicCapabilityResult{Available: false, Reason: gen.ReadPublicCapabilityResultReason("not_configured")}, nil
	}
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

type PutQaPolicyInput = gen.PutQaPolicyInput
type PutQaPolicyResult = gen.PutQaPolicyResult

// PutQaPolicy 乐观并发地替换公开问答策略；同幂等键重放原结果，不重复写。
func PutQaPolicy(ctx context.Context, op OperationContext, input PutQaPolicyInput) (PutQaPolicyResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.manage"); err != nil {
		return PutQaPolicyResult{}, err
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return PutQaPolicyResult{}, err
	}
	defer tx.Close()
	var replayed PutQaPolicyResult
	if found, replayErr := qaruntime.ReplayReceipt(tx, op, "qa.PutQaPolicy", input.IdempotencyKey, input, &replayed); replayErr != nil || found {
		return replayed, replayErr
	}
	current, err := qaruntime.ReadPolicy(tx)
	if err != nil {
		return PutQaPolicyResult{}, err
	}
	if current.Version != input.ExpectedVersion {
		return PutQaPolicyResult{}, platform.NewError(platform.CodeVersionMismatch, false, "问答策略版本已变化")
	}
	next := input.Policy
	next.Version = current.Version + 1
	raw, err := json.Marshal(next)
	if err != nil || tx.Runtime.Validators["QaPolicy"] == nil || tx.Runtime.Validators["QaPolicy"](raw) != nil {
		return PutQaPolicyResult{}, platform.NewError(platform.CodeValidation, false, "问答策略不符合合同")
	}
	if _, err = tx.Tx.ExecContext(tx.Context, `UPDATE qa_policy SET version=?,policy_json=? WHERE singleton=1 AND version=?`, next.Version, raw, current.Version); err != nil {
		return PutQaPolicyResult{}, qaruntime.StorageError(err)
	}
	// 保留天数改了，进行中的会话也按“创建时间 + 新天数”到期：缩短立刻生效，延长也一并顺延。
	if delta := next.RecordRetentionDays - current.RecordRetentionDays; delta != 0 {
		if _, err = tx.Tx.ExecContext(tx.Context, `UPDATE conversations SET expires_at=strftime('%Y-%m-%dT%H:%M:%fZ',expires_at,?) WHERE state='active'`, strconv.Itoa(delta)+" days"); err != nil {
			return PutQaPolicyResult{}, qaruntime.StorageError(err)
		}
	}
	result := PutQaPolicyResult{Policy: next}
	if err = qaruntime.SaveReceipt(tx, op, "qa.PutQaPolicy", input.IdempotencyKey, input, result); err != nil {
		return PutQaPolicyResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return PutQaPolicyResult{}, err
	}
	return result, nil
}
