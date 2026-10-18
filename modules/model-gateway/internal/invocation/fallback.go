package invocation

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/billing"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

type CreateFallbackAttemptInput struct {
	InvocationID      string `json:"invocationId"`
	PreviousAttemptID string `json:"previousAttemptId"`
	NextModelID       string `json:"nextModelId"`
	Reason            string `json:"reason"`
	IdempotencyKey    string `json:"idempotencyKey"`
}
type CreateFallbackAttemptResult struct {
	Attempt gen.Attempt `json:"attempt"`
}

func CreateFallbackAttempt(ctx context.Context, op OperationContext, input CreateFallbackAttemptInput) (CreateFallbackAttemptResult, error) {
	record, err := ReadExecutionRecord(ctx, op, input.InvocationID)
	if err != nil {
		return CreateFallbackAttemptResult{}, err
	}
	if record.Probe != nil {
		return CreateFallbackAttemptResult{}, platform.NewError(platform.CodeForbidden, false, "后台探测不能自动新增收费尝试")
	}
	if record.CancelRequested {
		return CreateFallbackAttemptResult{}, platform.NewError(platform.CodeCancelled, false, "已取消调用不能新增付费尝试")
	}
	if record.ActiveAttemptID != input.PreviousAttemptID {
		if len(record.Invocation.Attempts) > 0 {
			last := record.Invocation.Attempts[len(record.Invocation.Attempts)-1]
			if last.ModelId == input.NextModelID && string(last.RetryReason) == input.Reason {
				return CreateFallbackAttemptResult{Attempt: last}, nil
			}
		}
		return CreateFallbackAttemptResult{}, platform.NewError(platform.CodeConflict, false, "备用尝试的前驱已经变化")
	}
	previous, err := ReadExecutionAttempt(ctx, op, input.PreviousAttemptID)
	if err != nil {
		return CreateFallbackAttemptResult{}, err
	}
	var route gatewayruntime.FixedRoute
	if json.Unmarshal(record.Route, &route) != nil {
		return CreateFallbackAttemptResult{}, platform.NewError(platform.CodeIntegrityError, false, "备用路线损坏")
	}
	oldIndex, nextIndex := -1, -1
	for i, candidate := range route.Candidates {
		if candidate.Model.ModelId == previous.Model.ModelId {
			oldIndex = i
		}
		if candidate.Model.ModelId == input.NextModelID {
			nextIndex = i
		}
	}
	if input.Reason == "structured_retry" {
		if oldIndex != nextIndex {
			return CreateFallbackAttemptResult{}, platform.NewError(platform.CodeForbidden, false, "格式重试不能暗换模型")
		}
		for _, attempt := range record.Invocation.Attempts {
			if attempt.ModelId == input.NextModelID && attempt.RetryReason == "structured_retry" {
				return CreateFallbackAttemptResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "同模型最多一次独立格式重试")
			}
		}
	} else {
		if input.Reason != "fallback" || nextIndex != oldIndex+1 {
			return CreateFallbackAttemptResult{}, platform.NewError(platform.CodeForbidden, false, "备用模型必须是固定路线的直接后继")
		}
		for _, attempt := range record.Invocation.Attempts {
			if attempt.ModelId == input.NextModelID {
				return CreateFallbackAttemptResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "同备用模型不得再次付费尝试")
			}
		}
	}
	if nextIndex < 0 || len(record.Invocation.Attempts) >= len(route.Candidates)+1 {
		return CreateFallbackAttemptResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "固定路线已耗尽")
	}
	prepared, err := PrepareInvocationAttempt(ctx, op, record, nextIndex, input.Reason)
	if err != nil {
		return CreateFallbackAttemptResult{}, err
	}
	maximum, err := billing.ComputeUsageCost(prepared.Price, prepared.InputTokenBudget, prepared.OutputTokenBudget)
	if err != nil {
		return CreateFallbackAttemptResult{}, err
	}
	if _, err := billing.ReserveCost(ctx, op, billing.ReserveCostInput{InvocationID: input.InvocationID, AttemptID: prepared.Attempt.AttemptId, MaximumCost: maximum, PricingKnown: maximum != nil, IdempotencyKey: "reserve." + prepared.Attempt.AttemptId, PreparedAttempt: &prepared}); err != nil {
		return CreateFallbackAttemptResult{}, err
	}
	return CreateFallbackAttemptResult{Attempt: prepared.Attempt}, nil
}
