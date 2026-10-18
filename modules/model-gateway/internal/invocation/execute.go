package invocation

import (
	"context"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/billing"
	"nice-wiki/modules/model-gateway/internal/catalog"
	"nice-wiki/modules/model-gateway/internal/gen"
	"nice-wiki/modules/model-gateway/internal/provider"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"nice-wiki/modules/model-gateway/internal/tokens"
	"strconv"
	"time"
)

type ExecuteInvocationInput struct {
	InvocationID string `json:"invocationId"`
}
type ExecuteInvocationResult struct {
	State string `json:"state"`
}

func InvocationResultState(state string) string {
	switch state {
	case "succeeded":
		return "completed"
	case "cancelled":
		return "cancelled"
	case "charge_unknown":
		return "charge_unknown"
	default:
		return "failed"
	}
}

func ExecuteInvocation(ctx context.Context, op OperationContext, input ExecuteInvocationInput) (ExecuteInvocationResult, error) {
	record, claimed, err := ClaimInvocationLease(ctx, op, input.InvocationID)
	if err != nil {
		return ExecuteInvocationResult{}, err
	}
	if !claimed {
		return ExecuteInvocationResult{State: InvocationResultState(string(record.Invocation.State))}, nil
	}
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	stop, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-work.Done():
				return
			case <-ticker.C:
				if err := RenewInvocationLease(work, op, input.InvocationID, record.LeaseID); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { close(stop); <-stopped }()
	for {
		current, err := ReadExecutionRecord(work, op, input.InvocationID)
		if err != nil {
			_ = FinishInterruptedInvocation(ctx, op, input.InvocationID, "failed", ErrorCode(err))
			return ExecuteInvocationResult{}, err
		}
		record = current
		if record.CancelRequested || work.Err() != nil {
			state, err := FinishCancelledInvocation(ctx, op, record)
			return ExecuteInvocationResult{State: state}, err
		}
		if record.ActiveAttemptID == "" {
			prepared, err := PrepareInvocationAttempt(work, op, record, 0, "initial")
			if err != nil {
				_ = FinishInvocation(ctx, op, input.InvocationID, "failed", ErrorCode(err))
				return ExecuteInvocationResult{}, err
			}
			maximum, err := billing.ComputeUsageCost(prepared.Price, prepared.InputTokenBudget, prepared.OutputTokenBudget)
			if err != nil {
				_ = FinishInvocation(ctx, op, input.InvocationID, "failed", ErrorCode(err))
				return ExecuteInvocationResult{}, err
			}
			_, err = billing.ReserveCost(work, op, billing.ReserveCostInput{InvocationID: input.InvocationID, AttemptID: prepared.Attempt.AttemptId, MaximumCost: maximum, PricingKnown: maximum != nil, IdempotencyKey: "reserve." + prepared.Attempt.AttemptId, PreparedAttempt: &prepared})
			if err != nil {
				_ = FinishInvocation(ctx, op, input.InvocationID, "failed", ErrorCode(err))
				return ExecuteInvocationResult{}, err
			}
		}
		dispatched, err := DispatchInvocation(work, op, DispatchInvocationInput{InvocationID: input.InvocationID, LeaseID: record.LeaseID})
		if err != nil {
			_ = FinishInterruptedInvocation(ctx, op, input.InvocationID, "failed", ErrorCode(err))
			return ExecuteInvocationResult{}, err
		}
		outcome := dispatched.Outcome
		finalRead, cancelRead := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		record, err = ReadExecutionRecord(finalRead, op, input.InvocationID)
		if err != nil {
			cancelRead()
			return ExecuteInvocationResult{}, err
		}
		attempt, err := ReadExecutionAttempt(finalRead, op, record.ActiveAttemptID)
		cancelRead()
		if err != nil {
			return ExecuteInvocationResult{}, err
		}
		usageRecorded := false
		if outcome.InputTokens != nil && outcome.OutputTokens != nil && outcome.RequestID != "" {
			usage := billing.RecordUsageInput{InvocationID: input.InvocationID, AttemptID: attempt.Attempt.AttemptId, ProviderRequestID: outcome.RequestID, InputTokens: *outcome.InputTokens, OutputTokens: *outcome.OutputTokens, ReportedCost: outcome.ReportedCost, PricingComplete: outcome.PricingComplete}
			usage.UsageHash = billing.UsageDigest(usage)
			if _, err := billing.RecordUsage(ctx, op, usage); err == nil {
				if _, err = billing.SettleInvocation(ctx, op, billing.SettleInvocationInput{InvocationID: input.InvocationID, AttemptID: attempt.Attempt.AttemptId, UsageHash: usage.UsageHash, IdempotencyKey: "settle." + attempt.Attempt.AttemptId}); err == nil {
					usageRecorded = true
				}
			}
		}
		if !usageRecorded && outcome.AcceptanceProof != "not_accepted" {
			_, err = billing.MarkUnknownCharge(ctx, op, billing.MarkUnknownChargeInput{InvocationID: input.InvocationID, AttemptID: attempt.Attempt.AttemptId, Reason: "usage_missing", ProviderRequestID: Nonempty(outcome.RequestID)})
			if err != nil {
				return ExecuteInvocationResult{}, err
			}
		}
		if dispatched.FailureCode == "" && outcome.Completed {
			// 结果与费用分离：完整结果交付，缺账单的attempt仍unknown+held，不自动重算。
			if err := FinishInvocation(ctx, op, input.InvocationID, "succeeded", ""); err != nil {
				return ExecuteInvocationResult{}, err
			}
			return ExecuteInvocationResult{State: "completed"}, nil
		}
		if outcome.AcceptanceProof == "not_accepted" {
			if err := ReleaseUnacceptedAttempt(ctx, op, input.InvocationID, attempt.Attempt.AttemptId); err != nil {
				return ExecuteInvocationResult{}, err
			}
		}
		providerCode := dispatched.FailureCode
		if usageRecorded && (providerCode == platform.CodeIntegrityError || providerCode == platform.CodeUnsupportedSchema) {
			providerCode = "invalid_structured_output"
		}
		classification, _ := provider.ClassifyFailure(ctx, op, provider.ClassifyFailureInput{HTTPStatus: outcome.HTTPStatus, ProviderCode: providerCode, ResponseStarted: outcome.OutputStarted, AcceptanceProof: outcome.AcceptanceProof, UsageRecorded: usageRecorded})
		var fixed gatewayruntime.FixedRoute
		if json.Unmarshal(record.Route, &fixed) != nil {
			return ExecuteInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "固定备用路线损坏")
		}
		index := -1
		for i, candidate := range fixed.Candidates {
			if candidate.Model.ModelId == attempt.Model.ModelId {
				index = i
				break
			}
		}
		if index < 0 {
			return ExecuteInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "已派发尝试不属于不可变路线")
		}
		if work.Err() != nil || record.CancelRequested {
			state, err := FinishCancelledInvocation(ctx, op, record)
			return ExecuteInvocationResult{State: state}, err
		}
		if op.CostClass==platform.CostClassAdminProbe || outcome.OutputStarted || work.Err() != nil || record.CancelRequested || outcome.Completed || classification.Classification == "terminal" {
			state := "failed"
			if !usageRecorded && outcome.AcceptanceProof != "not_accepted" {
				state = "charge_unknown"
			}
			err := FinishInvocation(ctx, op, input.InvocationID, state, dispatched.FailureCode)
			return ExecuteInvocationResult{State: InvocationResultState(state)}, err
		}
		reason := "fallback"
		next := index + 1
		if classification.Classification == "retry_same" {
			used := false
			for _, old := range record.Invocation.Attempts {
				if old.ModelId == attempt.Model.ModelId && old.RetryReason == "structured_retry" {
					used = true
				}
			}
			if !used {
				reason = "structured_retry"
				next = index
			}
		}
		if next < 0 || next >= len(fixed.Candidates) {
			state := "failed"
			if !usageRecorded && outcome.AcceptanceProof != "not_accepted" {
				state = "charge_unknown"
			}
			err := FinishInvocation(ctx, op, input.InvocationID, state, dispatched.FailureCode)
			return ExecuteInvocationResult{State: InvocationResultState(state)}, err
		}
		_, err = CreateFallbackAttempt(work, op, CreateFallbackAttemptInput{InvocationID: input.InvocationID, PreviousAttemptID: attempt.Attempt.AttemptId, NextModelID: fixed.Candidates[next].Model.ModelId, Reason: reason, IdempotencyKey: "fallback." + attempt.Attempt.AttemptId + ":" + reason})
		if err != nil {
			state := "failed"
			if !usageRecorded && outcome.AcceptanceProof != "not_accepted" {
				state = "charge_unknown"
			}
			_ = FinishInvocation(ctx, op, input.InvocationID, state, ErrorCode(err))
			return ExecuteInvocationResult{}, err
		}
	}
}
func ReadExecutionRecord(ctx context.Context, op OperationContext, id string) (gatewayruntime.InvocationRecord, error) {
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return gatewayruntime.InvocationRecord{}, err
	}
	defer tx.Close()
	return gatewayruntime.ReadInvocationRecord(tx, op, id)
}
func ReadExecutionAttempt(ctx context.Context, op OperationContext, id string) (gatewayruntime.AttemptRecord, error) {
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return gatewayruntime.AttemptRecord{}, err
	}
	defer tx.Close()
	return gatewayruntime.ReadAttemptRecord(tx, id)
}
func ClaimInvocationLease(ctx context.Context, op OperationContext, id string) (gatewayruntime.InvocationRecord, bool, error) {
	tx, err := gatewayruntime.BeginExecution(ctx, op, true)
	if err != nil {
		return gatewayruntime.InvocationRecord{}, false, err
	}
	defer tx.Close()
	record, err := gatewayruntime.ReadInvocationRecord(tx, op, id)
	if err != nil {
		return record, false, err
	}
	if record.Invocation.State == "succeeded" || record.Invocation.State == "failed" || record.Invocation.State == "cancelled" || record.Invocation.State == "charge_unknown" || record.Invocation.State == "reconciled" {
		return record, false, nil
	}
	now := time.Now()
	if record.LeaseID != "" && record.LeaseUntilNS > now.UnixNano() {
		return record, false, platform.NewError(platform.CodeConflict, false, "原模型调用仍由有效执行租约持有")
	}
	record.LeaseID = platform.NewIdentifier()
	until := min(now.Add(30*time.Second).UnixNano(), record.Invocation.Deadline.UnixNano())
	record.LeaseUntilNS = until
	if record.ActiveAttemptID != "" {
		attempt, err := gatewayruntime.ReadAttemptRecord(tx, record.ActiveAttemptID)
		if err != nil {
			return record, false, err
		}
		if attempt.DispatchedNS > 0 {
			return record, false, platform.NewError(platform.CodeConflict, false, "已派发租约只能收敛未知，不能重新消费")
		}
		attempt.LeaseID = record.LeaseID
		attempt.LeaseUntilNS = until
		if err := gatewayruntime.SaveAttemptRecord(tx, attempt); err != nil {
			return record, false, err
		}
	}
	if err := gatewayruntime.SaveInvocationRecord(tx, record); err != nil {
		return record, false, err
	}
	if err := tx.Commit(); err != nil {
		return record, false, err
	}
	return record, true, nil
}
func RenewInvocationLease(ctx context.Context, op OperationContext, id, leaseID string) error {
	tx, err := gatewayruntime.BeginExecution(ctx, op, true)
	if err != nil {
		return err
	}
	defer tx.Close()
	record, err := gatewayruntime.ReadInvocationRecord(tx, op, id)
	if err != nil {
		return err
	}
	now := time.Now()
	if record.LeaseID != leaseID || record.LeaseUntilNS <= now.UnixNano() || record.CancelRequested {
		return platform.NewError(platform.CodeStaleResult, false, "模型执行租约或取消状态已变化")
	}
	until := min(now.Add(30*time.Second).UnixNano(), record.Invocation.Deadline.UnixNano())
	record.LeaseUntilNS = until
	if record.ActiveAttemptID != "" {
		attempt, err := gatewayruntime.ReadAttemptRecord(tx, record.ActiveAttemptID)
		if err != nil {
			return err
		}
		attempt.LeaseID = leaseID
		attempt.LeaseUntilNS = until
		if err := gatewayruntime.SaveAttemptRecord(tx, attempt); err != nil {
			return err
		}
	}
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE provider_slots SET expires_ns=? WHERE invocation_id=? AND data_epoch=?`, until, id, op.DataEpoch); err != nil {
		return gatewayruntime.ExecutionStorageError(err)
	}
	if err := gatewayruntime.SaveInvocationRecord(tx, record); err != nil {
		return err
	}
	return tx.Commit()
}
func PrepareInvocationAttempt(ctx context.Context, op OperationContext, record gatewayruntime.InvocationRecord, index int, reason string) (gatewayruntime.AttemptRecord, error) {
	if err := gatewayruntime.CheckLanguageSuggestionApproval(ctx, op, record.Request); err != nil {
		return gatewayruntime.AttemptRecord{}, err
	}
	var route gatewayruntime.FixedRoute
	if json.Unmarshal(record.Route, &route) != nil || index < 0 || index >= len(route.Candidates) {
		return gatewayruntime.AttemptRecord{}, platform.NewError(platform.CodeIntegrityError, false, "固定尝试路线不可读")
	}
	candidate := route.Candidates[index]
	if err := ValidateInputModel(record.Request, candidate.Model, string(candidate.Provider.Adapter)); err != nil {
		return gatewayruntime.AttemptRecord{}, err
	}
	if record.Request.ExpectedEmbeddingSpace != nil {
		space, err := catalog.ModelEmbeddingSpace(candidate.Model)
		if err != nil {
			return gatewayruntime.AttemptRecord{}, err
		}
		expected := record.Request.ExpectedEmbeddingSpace
		if expected.SpaceId != space.SpaceID || expected.Provider != space.Provider || expected.ModelVersion != space.ModelVersion || expected.Dimensions != space.Dimensions || expected.PreprocessingVersion != space.PreprocessingVersion {
			return gatewayruntime.AttemptRecord{}, platform.NewError(platform.CodeStaleResult, false, "备用预留前固定嵌入空间不匹配")
		}
	}
	inputBudget := 0
	if record.Request.Capability == "vision" {
		inputBudget = candidate.Model.MaxInputTokens
		if inputBudget < 1 {
			return gatewayruntime.AttemptRecord{}, platform.NewError(platform.CodeDependencyUnavailable, false, "视觉模型须声明完整输入窗口作保守费用上界")
		}
	} else {
		texts := append([]string{}, record.Request.Texts...)
		if record.Request.Query != nil {
			texts = append(texts, *record.Request.Query)
		}
		estimated, err := tokens.EstimateTokens(ctx, op, tokens.EstimateTokensInput{ModelId: candidate.Model.ModelId, Texts: texts, Messages: record.Request.Messages})
		if err != nil {
			return gatewayruntime.AttemptRecord{}, err
		}
		inputBudget = estimated.InputTokens
		if candidate.Model.MaxInputTokens > 0 && inputBudget > candidate.Model.MaxInputTokens {
			return gatewayruntime.AttemptRecord{}, platform.NewError(platform.CodeBudgetExceeded, false, "输入保守预算超过固定模型窗口")
		}
	}
	ordinal := len(record.Invocation.Attempts) + 1
	id := "attempt." + platform.HashBytes([]byte(record.Invocation.InvocationId+":"+candidate.Model.ModelId+":"+reason+":"+strconv.Itoa(ordinal)))
	hash, err := gatewayruntime.ModelSnapshotHash(candidate.Model)
	if err != nil {
		return gatewayruntime.AttemptRecord{}, err
	}
	attempt := gen.Attempt{ModelSnapshotHash: &hash, AttemptId: id, InvocationId: record.Invocation.InvocationId, Ordinal: ordinal, ModelId: candidate.Model.ModelId, State: gen.AttemptState("reserved"), ReservationId: "reservation." + id, RetryReason: gen.AttemptRetryReason(reason)}
	return gatewayruntime.AttemptRecord{Attempt: attempt, Model: candidate.Model, Provider: candidate.Provider, CredentialVersion: candidate.CredentialVersion, Price: candidate.Price, LeaseID: record.LeaseID, LeaseUntilNS: record.LeaseUntilNS, InputTokenBudget: inputBudget, OutputTokenBudget: min(record.Request.MaximumOutputTokens, candidate.Model.MaxOutputTokens)}, nil
}
func ErrorCode(err error) string {
	var problem *platform.Error
	if errors.As(err, &problem) {
		return problem.Code
	}
	return platform.CodeInternal
}
func Nonempty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
