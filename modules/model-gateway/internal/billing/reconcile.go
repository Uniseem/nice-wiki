package billing

import (
	"context"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"slices"
	"time"
)

type MarkUnknownChargeInput struct {
	InvocationID      string  `json:"invocationId"`
	AttemptID         string  `json:"attemptId,omitempty"`
	Reason            string  `json:"reason"`
	ProviderRequestID *string `json:"providerRequestId,omitempty"`
}
type MarkUnknownChargeResult struct {
	Invocation gen.Invocation `json:"invocation"`
}
type ReconcileChargeInput struct {
	InvocationID      string    `json:"invocationId"`
	AttemptID         *string   `json:"attemptId,omitempty"`
	ProofKind         string    `json:"proofKind"`
	ActualCost        gen.Money `json:"actualCost"`
	ProofHash         string    `json:"proofHash"`
	InputTokens       *int      `json:"inputTokens,omitempty"`
	OutputTokens      *int      `json:"outputTokens,omitempty"`
	IdempotencyKey    string    `json:"idempotencyKey"`
	ProviderRequestID *string   `json:"providerRequestId,omitempty"`
}
type ReconcileChargeResult struct {
	Entry       *gen.LedgerEntry `json:"entry,omitempty"`
	Reservation gen.Reservation  `json:"reservation"`
}

func MarkUnknownCharge(ctx context.Context, op OperationContext, input MarkUnknownChargeInput) (MarkUnknownChargeResult, error) {
	tx, err := gatewayruntime.BeginFinalization(ctx, op)
	if err != nil {
		return MarkUnknownChargeResult{}, err
	}
	defer tx.Close()
	record, err := gatewayruntime.ReadInvocationRecord(tx, op, input.InvocationID)
	if err != nil {
		return MarkUnknownChargeResult{}, err
	}
	id := input.AttemptID
	if id == "" {
		id = record.ActiveAttemptID
	}
	attempt, err := gatewayruntime.ReadAttemptRecord(tx, id)
	if err != nil {
		return MarkUnknownChargeResult{}, err
	}
	if attempt.Attempt.InvocationId != input.InvocationID || attempt.DispatchedNS == 0 {
		return MarkUnknownChargeResult{}, platform.NewError(platform.CodeConflict, false, "未派发尝试不能归为收费未知")
	}
	reservation, err := ReadReservation(tx, attempt.Attempt.ReservationId)
	if err != nil {
		return MarkUnknownChargeResult{}, err
	}
	if reservation.State != "held" {
		return MarkUnknownChargeResult{}, platform.NewError(platform.CodeConflict, false, "已结算或明确释放的尝试不能重标未知")
	}
	if input.ProviderRequestID != nil {
		if attempt.Attempt.ProviderRequestId != nil && *attempt.Attempt.ProviderRequestId != *input.ProviderRequestID {
			return MarkUnknownChargeResult{}, platform.NewError(platform.CodeIntegrityError, false, "未知费用证据更换了供应方收费ID")
		}
		attempt.Attempt.ProviderRequestId = input.ProviderRequestID
	}
	attempt.Attempt.State = gen.AttemptState("charge_unknown")
	if err := gatewayruntime.SaveAttemptRecord(tx, attempt); err != nil {
		return MarkUnknownChargeResult{}, err
	}
	for i := range record.Invocation.Attempts {
		if record.Invocation.Attempts[i].AttemptId == id {
			record.Invocation.Attempts[i] = attempt.Attempt
		}
	}
	if id == record.ActiveAttemptID && record.TerminalNS == 0 {
		record.Invocation.State = gen.InvocationState("charge_unknown")
	}
	if err := gatewayruntime.SaveInvocationRecord(tx, record); err != nil {
		return MarkUnknownChargeResult{}, err
	}
	payload := struct {
		InvocationID  string `json:"invocationId"`
		AttemptID     string `json:"attemptId"`
		Root          string `json:"rootOperationId"`
		ReservationID string `json:"reservationId"`
		Reason        string `json:"reason"`
		Epoch         string `json:"dataEpoch"`
	}{input.InvocationID, id, record.Invocation.RootOperationId, reservation.ReservationId, input.Reason, op.DataEpoch}
	if err := gatewayruntime.EnqueueExecutionEvent(tx, record.Operation, "invocation.charge_unknown", input.InvocationID, attempt.Attempt.Ordinal, payload); err != nil {
		return MarkUnknownChargeResult{}, err
	}
	reconcile := struct {
		InvocationID  string  `json:"invocationId"`
		AttemptID     string  `json:"attemptId"`
		RequestID     *string `json:"providerRequestId,omitempty"`
		ReservationID string  `json:"reservationId"`
		Root          string  `json:"rootOperationId"`
		Epoch         string  `json:"dataEpoch"`
	}{input.InvocationID, id, attempt.Attempt.ProviderRequestId, reservation.ReservationId, record.Invocation.RootOperationId, op.DataEpoch}
	if err := gatewayruntime.EnqueueExecutionEvent(tx, record.Operation, "reconciliation.requested", input.InvocationID, attempt.Attempt.Ordinal, reconcile); err != nil {
		return MarkUnknownChargeResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return MarkUnknownChargeResult{}, err
	}
	return MarkUnknownChargeResult{Invocation: record.Invocation}, nil
}

// ReconcileCharge 仅处理当前超管明确提交的证明；不能凭网络超时推导未收费。
func ReconcileCharge(ctx context.Context, op OperationContext, input ReconcileChargeInput) (ReconcileChargeResult, error) {
	if op.CallerService != "portal" || op.Actor.Kind != "admin" || !slices.Contains(op.Actor.Grants, "superadmin") {
		return ReconcileChargeResult{}, platform.NewError(platform.CodeForbidden, false, "费用人工证明只允许当前超管")
	}
	if (input.InputTokens == nil) != (input.OutputTokens == nil) {
		return ReconcileChargeResult{}, platform.NewError(platform.CodeValidation, false, "人工用量的输入与输出token必须同时给出")
	}
	if input.ProofKind == "not_charged" && input.ActualCost.Micros != 0 {
		return ReconcileChargeResult{}, platform.NewError(platform.CodeValidation, false, "未收费证明不能带非零金额")
	}
	tx, err := gatewayruntime.BeginFinalization(ctx, op)
	if err != nil {
		return ReconcileChargeResult{}, err
	}
	defer tx.Close()
	var result ReconcileChargeResult
	replay, err := gatewayruntime.ReplayExecutionReceipt(tx, op, "model-gateway.ReconcileCharge", input.IdempotencyKey, input, &result)
	if err != nil || replay {
		return result, err
	}
	record, err := gatewayruntime.ReadInvocationRecord(tx, op, input.InvocationID)
	if err != nil {
		return result, err
	}
	candidates := make([]string, 0)
	for _, attempt := range record.Invocation.Attempts {
		var pending bool
		err := tx.Tx.QueryRowContext(tx.Context, `SELECT EXISTS(SELECT 1 FROM reservations WHERE attempt_id=? AND state='held') OR EXISTS(SELECT 1 FROM ledger_entries l WHERE l.attempt_id=? AND l.pricing_known=0 AND NOT EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id)) OR EXISTS(SELECT 1 FROM invocation_usage u WHERE u.attempt_id=? AND u.conflict_hash IS NOT NULL)`, attempt.AttemptId, attempt.AttemptId, attempt.AttemptId).Scan(&pending)
		if err != nil {
			return result, gatewayruntime.ExecutionStorageError(err)
		}
		if pending && (attempt.State == "charge_unknown" || attempt.State == "failed" || attempt.State == "succeeded") {
			candidates = append(candidates, attempt.AttemptId)
		}
	}
	id := ""
	if input.AttemptID != nil {
		id = *input.AttemptID
	} else if len(candidates) == 1 {
		id = candidates[0]
	} else {
		return result, platform.NewError(platform.CodeConflict, false, "请明确唯一待对账attemptId")
	}
	if !slices.Contains(candidates, id) {
		return result, platform.NewError(platform.CodeConflict, false, "指定尝试没有待对账费用")
	}
	attempt, err := gatewayruntime.ReadAttemptRecord(tx, id)
	if err != nil {
		return result, err
	}
	reservation, err := ReadReservation(tx, attempt.Attempt.ReservationId)
	if err != nil {
		return result, err
	}
	var old gen.LedgerEntry
	old, err = ReadAttemptLedger(tx, id)
	if err != nil {
		var failure *platform.Error
		if !errors.As(err, &failure) || failure.Code != platform.CodeNotFound {
			return result, err
		}
	}
	if input.ProofKind == "charged" {
		if input.ProviderRequestID != nil {
			if attempt.Attempt.ProviderRequestId != nil && *attempt.Attempt.ProviderRequestId != *input.ProviderRequestID {
				return result, platform.NewError(platform.CodeConflict, false, "人工证明不能改写已知供应方收费ID")
			}
			attempt.Attempt.ProviderRequestId = input.ProviderRequestID
		}
		if attempt.Attempt.ProviderRequestId == nil {
			return result, platform.NewError(platform.CodeValidation, false, "原收费ID未知，已收费证明必须显式提供真实供应方ID")
		}
		inputTokens, outputTokens := input.InputTokens, input.OutputTokens
		if inputTokens == nil || outputTokens == nil {
			usage, err := ReadUsage(tx, attempt, attempt.UsageHash)
			if err != nil {
				return result, platform.NewError(platform.CodeValidation, false, "原用量未知，已收费证明必须显式给出真实inputTokens和outputTokens")
			}
			inputTokens = &usage.InputTokens
			outputTokens = &usage.OutputTokens
		}
		entry := gen.LedgerEntry{EntryId: "reconcile." + platform.HashBytes([]byte(id+":"+input.ProofHash)), InvocationId: input.InvocationID, AttemptId: id, ProviderRequestId: *attempt.Attempt.ProviderRequestId, InputTokens: *inputTokens, OutputTokens: *outputTokens, PriceSnapshot: attempt.Price, ActualCost: &input.ActualCost, PricingKnown: true, Status: gen.LedgerEntryStatus("reconciled"), CreatedAt: time.Now().UTC()}
		if err := InsertLedgerEntry(tx, entry, input.ProofHash, attempt.Price, old.EntryId); err != nil {
			return result, err
		}
		result.Entry = &entry
	} else if input.ProofKind != "not_charged" {
		return result, platform.NewError(platform.CodeValidation, false, "未知人工费用证明类型")
	}
	if input.ProofKind == "not_charged" && old.EntryId != "" {
		if _, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO ledger_voids(entry_id,proof_hash,actor_id,created_ns) VALUES(?,?,?,?)`, old.EntryId, input.ProofHash, op.Actor.SubjectID, time.Now().UnixNano()); err != nil {
			return result, gatewayruntime.ExecutionStorageError(err)
		}
	}
	state := "settled"
	amount := input.ActualCost.Micros
	if input.ProofKind == "not_charged" {
		state = "released"
	}
	var held any
	if reservation.PricingKnown {
		held = 0
	}
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE reservations SET state=?,held_micro_usd=?,version=version+1 WHERE reservation_id=?`, state, held, reservation.ReservationId); err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE fee_commitments SET state=?,amount_micro_usd=?,pricing_known=1 WHERE reservation_id=?`, state, amount, reservation.ReservationId); err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	proof, _ := platform.CanonicalJSON(input)
	if _, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO reconciliation_proofs(attempt_id,proof_hash,actor_id,proof_json,created_ns) VALUES(?,?,?,?,?)`, id, input.ProofHash, op.Actor.SubjectID, proof, time.Now().UnixNano()); err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	attempt.Attempt.State = gen.AttemptState("reconciled")
	if err := gatewayruntime.SaveAttemptRecord(tx, attempt); err != nil {
		return result, err
	}
	unresolved := false
	for i := range record.Invocation.Attempts {
		if record.Invocation.Attempts[i].AttemptId == id {
			record.Invocation.Attempts[i] = attempt.Attempt
		}
		if record.Invocation.Attempts[i].State == "charge_unknown" {
			unresolved = true
		}
	}
	if !unresolved && record.Invocation.State == "charge_unknown" {
		record.Invocation.State = gen.InvocationState("reconciled")
	}
	if err := gatewayruntime.SaveInvocationRecord(tx, record); err != nil {
		return result, err
	}
	result.Reservation, err = ReadReservation(tx, reservation.ReservationId)
	if err != nil {
		return result, err
	}
	var sourceEpoch *string
	if record.Operation.DataEpoch != op.DataEpoch {
		source := record.Operation.DataEpoch
		sourceEpoch = &source
	}
	var entryID *string
	var actual *gen.Money
	if result.Entry != nil {
		entry := result.Entry.EntryId
		entryID = &entry
		actual = result.Entry.ActualCost
	}
	payload := struct {
		InvocationID string     `json:"invocationId"`
		AttemptID    string     `json:"attemptId"`
		Root         string     `json:"rootOperationId"`
		CostClass    string     `json:"costClass"`
		ProofHash    string     `json:"proofHash"`
		ProofKind    string     `json:"proofKind"`
		EntryID      *string    `json:"entryId,omitempty"`
		ActualCost   *gen.Money `json:"actualCost,omitempty"`
		Epoch        string     `json:"dataEpoch"`
		SourceEpoch  *string    `json:"sourceDataEpoch,omitempty"`
	}{input.InvocationID, id, record.Invocation.RootOperationId, string(record.Invocation.CostClass), input.ProofHash, input.ProofKind, entryID, actual, op.DataEpoch, sourceEpoch}
	if err := gatewayruntime.EnqueueExecutionEvent(tx, op, "invocation.reconciled", input.InvocationID, len(record.Invocation.Attempts), payload); err != nil {
		return result, err
	}
	if err := gatewayruntime.ReleaseResolvedReferences(tx, record); err != nil {
		return result, err
	}
	if err := gatewayruntime.SaveExecutionReceipt(tx, op, "model-gateway.ReconcileCharge", input.IdempotencyKey, input, result); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return ReconcileChargeResult{}, err
	}
	return result, nil
}
