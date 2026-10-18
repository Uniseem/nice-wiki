package invocation

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/catalog"
	"nice-wiki/modules/model-gateway/internal/concurrency"
	"nice-wiki/modules/model-gateway/internal/gen"
	"nice-wiki/modules/model-gateway/internal/provider"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"nice-wiki/modules/model-gateway/internal/stream"
	"nice-wiki/modules/model-gateway/internal/tokens"
	"time"
)

type DispatchInvocationInput struct {
	InvocationID string `json:"invocationId"`
	LeaseID      string `json:"leaseId"`
}
type DispatchInvocationResult struct {
	Invocation  gen.Invocation           `json:"invocation"`
	Outcome     provider.ProviderOutcome `json:"-"`
	FailureCode string                   `json:"failureCode,omitempty"`
}

func DispatchInvocation(ctx context.Context, op OperationContext, input DispatchInvocationInput) (DispatchInvocationResult, error) {
	record, err := ReadExecutionRecord(ctx, op, input.InvocationID)
	if err != nil {
		return DispatchInvocationResult{}, err
	}
	attempt, err := ReadExecutionAttempt(ctx, op, record.ActiveAttemptID)
	if err != nil {
		return DispatchInvocationResult{}, err
	}
	failure := func(cause error) (DispatchInvocationResult, error) {
		return DispatchInvocationResult{Invocation: record.Invocation, Outcome: provider.ProviderOutcome{AcceptanceProof: "not_accepted", Code: ErrorCode(cause)}, FailureCode: ErrorCode(cause)}, nil
	}
	if record.LeaseID != input.LeaseID || record.LeaseUntilNS <= time.Now().UnixNano() || record.CancelRequested || attempt.DispatchedNS != 0 || attempt.Attempt.State != "reserved" {
		return failure(platform.NewError(platform.CodeStaleResult, false, "派发原租约或状态已变化"))
	}
	if err := gatewayruntime.CheckLanguageSuggestionApproval(ctx, op, record.Request); err != nil {
		return failure(err)
	}
	if err := ValidateInputModel(record.Request, attempt.Model, string(attempt.Provider.Adapter)); err != nil {
		return failure(err)
	}
	slot, err := concurrency.AcquireProviderSlot(ctx, op, concurrency.AcquireProviderSlotInput{ProviderID: attempt.Provider.ProviderId, InvocationID: input.InvocationID, LeaseSeconds: 30})
	if err != nil {
		return failure(err)
	}
	defer func() {
		_, _ = concurrency.ReleaseProviderSlot(ctx, op, concurrency.ReleaseProviderSlotInput{LeaseID: slot.LeaseID, InvocationID: input.InvocationID})
	}()
	if record.Request.ExpectedEmbeddingSpace != nil {
		space, err := catalog.ModelEmbeddingSpace(attempt.Model)
		if err != nil {
			return failure(err)
		}
		expected := record.Request.ExpectedEmbeddingSpace
		if expected.SpaceId != space.SpaceID || expected.Provider != space.Provider || expected.ModelVersion != space.ModelVersion || expected.Dimensions != space.Dimensions || expected.PreprocessingVersion != space.PreprocessingVersion {
			return failure(platform.NewError(platform.CodeStaleResult, false, "实际供应方发送前嵌入空间不匹配"))
		}
	}
	request, err := provider.BuildProviderRequest(ctx, op, provider.BuildProviderRequestInput{InvocationID: input.InvocationID, Attempt: attempt.Attempt.Ordinal})
	if err != nil {
		return failure(err)
	}
	tx, err := gatewayruntime.BeginExecution(ctx, op, true)
	if err != nil {
		return failure(err)
	}
	current, err := gatewayruntime.ReadInvocationRecord(tx, op, input.InvocationID)
	if err != nil {
		tx.Close()
		return DispatchInvocationResult{}, err
	}
	if current.LeaseID != input.LeaseID || current.LeaseUntilNS <= time.Now().UnixNano() || current.CancelRequested {
		tx.Close()
		return failure(platform.NewError(platform.CodeStaleResult, false, "供应方发送前租约已变化"))
	}
	active, err := gatewayruntime.ReadAttemptRecord(tx, current.ActiveAttemptID)
	if err != nil {
		tx.Close()
		return DispatchInvocationResult{}, err
	}
	if active.DispatchedNS != 0 || active.Attempt.AttemptId != attempt.Attempt.AttemptId {
		tx.Close()
		return DispatchInvocationResult{}, platform.NewError(platform.CodeConflict, false, "同尝试不能再次派发")
	}
	active.DispatchedNS = time.Now().UnixNano()
	active.Attempt.State = gen.AttemptState("dispatching")
	current.Invocation.State = gen.InvocationState("dispatching")
	for i := range current.Invocation.Attempts {
		if current.Invocation.Attempts[i].AttemptId == active.Attempt.AttemptId {
			current.Invocation.Attempts[i] = active.Attempt
		}
	}
	if err := gatewayruntime.SaveAttemptRecord(tx, active); err != nil {
		tx.Close()
		return DispatchInvocationResult{}, err
	}
	if err := gatewayruntime.SaveInvocationRecord(tx, current); err != nil {
		tx.Close()
		return DispatchInvocationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		tx.Close()
		return DispatchInvocationResult{}, err
	}
	tx.Close()
	outcome, sendErr := provider.SendProviderRequest(ctx, request, attempt.Provider, string(record.Request.Capability), record.Request.CandidateIds, func(delta string) error {
		_, err := stream.AppendStreamEvent(ctx, op, stream.AppendStreamEventInput{InvocationID: input.InvocationID, LeaseID: input.LeaseID, Event: gatewayruntime.StreamEvent{Event: "delta", Delta: &delta}})
		return err
	})
	code := ""
	if sendErr != nil {
		code = ErrorCode(sendErr)
	}
	if code == "" && record.Request.Capability == "embedding" {
		space, err := catalog.ModelEmbeddingSpace(attempt.Model)
		if err != nil {
			code = ErrorCode(err)
		} else if outcome.Output == nil || len(outcome.Output.Vectors) != len(record.Request.Texts) {
			code = platform.CodeIntegrityError
		} else {
			outcome.Output.EmbeddingSpace = &platform.EmbeddingSpace{SpaceID: space.SpaceID, Provider: space.Provider, ModelVersion: space.ModelVersion, Dimensions: int64(space.Dimensions), PreprocessingVersion: space.PreprocessingVersion}
			_, err = tokens.ValidateEmbeddingSpace(ctx, op, tokens.ValidateEmbeddingSpaceInput{Space: gen.EmbeddingSpace{SpaceId: space.SpaceID, Provider: space.Provider, ModelVersion: space.ModelVersion, Dimensions: space.Dimensions, PreprocessingVersion: space.PreprocessingVersion}, Vectors: outcome.Output.Vectors})
			if err != nil {
				code = ErrorCode(err)
			}
		}
	}
	if code != "" {
		outcome.Code = code
		if outcome.Completed && code == platform.CodeIntegrityError {
			outcome.Completed = false
			outcome.Output = nil
		}
	}
	final, err := gatewayruntime.BeginFinalization(ctx, op)
	if err != nil {
		return DispatchInvocationResult{}, err
	}
	defer final.Close()
	current, err = gatewayruntime.ReadInvocationRecord(final, op, input.InvocationID)
	if err != nil {
		return DispatchInvocationResult{}, err
	}
	if outcome.Output != nil {
		current.Output = outcome.Output
	}
	active, err = gatewayruntime.ReadAttemptRecord(final, attempt.Attempt.AttemptId)
	if err != nil {
		return DispatchInvocationResult{}, err
	}
	if outcome.Completed && code == "" {
		if current.Invocation.ResultAttemptId != nil && *current.Invocation.ResultAttemptId != active.Attempt.AttemptId {
			return DispatchInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "已有完整结果不能改写来源attempt")
		}
		id := active.Attempt.AttemptId
		current.Invocation.ResultAttemptId = &id
	}
	if outcome.RequestID != "" {
		active.Attempt.ProviderRequestId = &outcome.RequestID
		if current.ActiveAttemptID == active.Attempt.AttemptId {
			current.Invocation.ProviderRequestId = &outcome.RequestID
		}
	}
	if err := gatewayruntime.SaveAttemptRecord(final, active); err != nil {
		return DispatchInvocationResult{}, err
	}
	raw, _ := platform.CanonicalJSON(outcome)
	if _, err := final.Tx.ExecContext(final.Context, `INSERT INTO attempt_evidence(attempt_id,outcome_json,outcome_hash,recorded_ns) VALUES(?,?,?,?)`, active.Attempt.AttemptId, raw, platform.HashBytes(raw), time.Now().UnixNano()); err != nil {
		return DispatchInvocationResult{}, gatewayruntime.ExecutionStorageError(err)
	}
	if err := gatewayruntime.SaveInvocationRecord(final, current); err != nil {
		return DispatchInvocationResult{}, err
	}
	if err := final.Commit(); err != nil {
		return DispatchInvocationResult{}, err
	}
	return DispatchInvocationResult{Invocation: current.Invocation, Outcome: outcome, FailureCode: code}, nil
}
func ReadAttemptEvidence(tx *gatewayruntime.ExecutionTransaction, id string) (provider.ProviderOutcome, error) {
	var raw []byte
	var hash string
	var result provider.ProviderOutcome
	if err := tx.Tx.QueryRowContext(tx.Context, `SELECT outcome_json,outcome_hash FROM attempt_evidence WHERE attempt_id=?`, id).Scan(&raw, &hash); err == sql.ErrNoRows {
		return result, platform.NewError(platform.CodeNotFound, false, "原派发尚无完整持久响应证据")
	} else if err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	if platform.HashBytes(raw) != hash || json.Unmarshal(raw, &result) != nil {
		return result, platform.NewError(platform.CodeIntegrityError, false, "供应方脱敏证据损坏")
	}
	canonical, err := platform.CanonicalJSON(result)
	if err != nil || platform.HashBytes(canonical) != hash {
		return result, platform.NewError(platform.CodeIntegrityError, false, "供应方证据含未知或不完整字段，不能损失信息读取")
	}
	return result, nil
}
