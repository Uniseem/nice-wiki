package invocation

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/billing"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"time"
)

type RecoverLeasesInput struct {
	Now   time.Time `json:"now"`
	Limit int       `json:"limit"`
}
type RecoverLeasesResult struct {
	Recovered      int `json:"recovered"`
	UnknownCharges int `json:"unknownCharges"`
}

// InvocationSafeToRetry只授权继续原hash的未派发阶段，不是新收费准入。
func InvocationSafeToRetry(ctx context.Context, op OperationContext, record gatewayruntime.InvocationRecord) (bool, error) {
	if record.Invocation.State != "accepted" && record.Invocation.State != "reserved" {
		return false, nil
	}
	if record.CancelRequested {
		return false, nil
	}
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return false, err
	}
	defer tx.Close()
	for _, attempt := range record.Invocation.Attempts {
		stored, err := gatewayruntime.ReadAttemptRecord(tx, attempt.AttemptId)
		if err != nil {
			return false, err
		}
		if stored.DispatchedNS > 0 || stored.Attempt.State == "charge_unknown" || stored.Attempt.State == "succeeded" || stored.Attempt.State == "reconciled" {
			return false, nil
		}
	}
	return true, nil
}
func RecoverLeases(ctx context.Context, op OperationContext, input RecoverLeasesInput) (RecoverLeasesResult, error) {
	if op.CallerService != "model-gateway" || op.Actor.Kind != "service" || op.Actor.SubjectID != "model-gateway" {
		return RecoverLeasesResult{}, platform.NewError(platform.CodeForbidden, false, "调用租约恢复仅本域后台执行")
	}
	options, err := gatewayruntime.ExecutionDependencies()
	if err != nil {
		return RecoverLeasesResult{}, err
	}
	if options.Tasks == nil {
		return RecoverLeasesResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "调用恢复任务组未装配")
	}
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return RecoverLeasesResult{}, err
	}
	rows, err := tx.Tx.QueryContext(tx.Context, `SELECT invocation_id,operation_json FROM invocations WHERE data_epoch=? AND lease_until_ns<=? AND (state IN ('accepted','reserved','dispatching','streaming','cancel_requested') OR (state='charge_unknown' AND terminal_ns=0)) ORDER BY created_ns,invocation_id LIMIT ?`, op.DataEpoch, input.Now.UnixNano(), input.Limit)
	if err != nil {
		tx.Close()
		return RecoverLeasesResult{}, gatewayruntime.ExecutionStorageError(err)
	}
	type candidate struct {
		ID        string
		Operation OperationContext
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		var raw []byte
		if err := rows.Scan(&item.ID, &raw); err != nil {
			rows.Close()
			tx.Close()
			return RecoverLeasesResult{}, gatewayruntime.ExecutionStorageError(err)
		}
		if json.Unmarshal(raw, &item.Operation) != nil || item.Operation.DataEpoch != op.DataEpoch {
			rows.Close()
			tx.Close()
			return RecoverLeasesResult{}, platform.NewError(platform.CodeIntegrityError, false, "原调用恢复上下文损坏")
		}
		candidates = append(candidates, item)
	}
	err = rows.Err()
	rows.Close()
	tx.Close()
	if err != nil {
		return RecoverLeasesResult{}, gatewayruntime.ExecutionStorageError(err)
	}
	result := RecoverLeasesResult{}
	for _, item := range candidates {
		record, err := ReadExecutionRecord(ctx, item.Operation, item.ID)
		if err != nil {
			return result, err
		}
		safe, err := InvocationSafeToRetry(ctx, item.Operation, record)
		if err != nil {
			return result, err
		}
		if safe && record.Invocation.Deadline.After(input.Now) {
			started, err := StartAcceptedInvocation(ctx, options, record)
			if err != nil {
				return result, err
			}
			if started {
				result.Recovered++
			}
			continue
		}
		if record.CancelRequested {
			if _, err := FinishCancelledInvocation(ctx, item.Operation, record); err != nil {
				return result, err
			}
			result.Recovered++
			continue
		}
		if record.ActiveAttemptID == "" {
			if err := FinishInvocation(ctx, item.Operation, item.ID, "failed", platform.CodeDeadlineExceeded); err != nil {
				return result, err
			}
			result.Recovered++
			continue
		}
		attempt, err := ReadExecutionAttempt(ctx, item.Operation, record.ActiveAttemptID)
		if err != nil {
			return result, err
		}
		if attempt.DispatchedNS == 0 {
			if err := ReleaseUnacceptedAttempt(ctx, item.Operation, item.ID, attempt.Attempt.AttemptId); err != nil {
				return result, err
			}
			if err := FinishInvocation(ctx, item.Operation, item.ID, "failed", platform.CodeDeadlineExceeded); err != nil {
				return result, err
			}
			result.Recovered++
			continue
		}
		evidenceTx, err := gatewayruntime.BeginExecution(ctx, item.Operation, false)
		if err != nil {
			return result, err
		}
		evidence, evidenceErr := ReadAttemptEvidence(evidenceTx, attempt.Attempt.AttemptId)
		evidenceTx.Close()
		if evidenceErr != nil && ErrorCode(evidenceErr) != platform.CodeNotFound {
			return result, evidenceErr
		}
		if evidenceErr == nil && evidence.Completed && evidence.Code == "" {
			settled := false
			if evidence.InputTokens != nil && evidence.OutputTokens != nil && evidence.RequestID != "" {
				usage := billing.RecordUsageInput{InvocationID: item.ID, AttemptID: attempt.Attempt.AttemptId, ProviderRequestID: evidence.RequestID, InputTokens: *evidence.InputTokens, OutputTokens: *evidence.OutputTokens, ReportedCost: evidence.ReportedCost, PricingComplete: evidence.PricingComplete}
				usage.UsageHash = billing.UsageDigest(usage)
				if _, err := billing.RecordUsage(ctx, item.Operation, usage); err == nil {
					if _, err := billing.SettleInvocation(ctx, item.Operation, billing.SettleInvocationInput{InvocationID: item.ID, AttemptID: attempt.Attempt.AttemptId, UsageHash: usage.UsageHash, IdempotencyKey: "settle." + attempt.Attempt.AttemptId}); err == nil {
						settled = true
					}
				}
			}
			if !settled {
				if _, err := billing.MarkUnknownCharge(ctx, item.Operation, billing.MarkUnknownChargeInput{InvocationID: item.ID, AttemptID: attempt.Attempt.AttemptId, Reason: "usage_missing", ProviderRequestID: Nonempty(evidence.RequestID)}); err != nil {
					return result, err
				}
				result.UnknownCharges++
			}
			if err := FinishInvocation(ctx, item.Operation, item.ID, "succeeded", ""); err != nil {
				return result, err
			}
			result.Recovered++
			continue
		}
		if _, err := billing.MarkUnknownCharge(ctx, item.Operation, billing.MarkUnknownChargeInput{InvocationID: item.ID, AttemptID: attempt.Attempt.AttemptId, Reason: "connection_lost", ProviderRequestID: attempt.Attempt.ProviderRequestId}); err != nil {
			return result, err
		}
		if err := FinishInvocation(ctx, item.Operation, item.ID, "charge_unknown", platform.CodeDependencyUnavailable); err != nil {
			return result, err
		}
		result.UnknownCharges++
	}
	return result, nil
}
