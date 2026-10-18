package billing

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"time"
)

type SettleInvocationInput struct {
	InvocationID   string `json:"invocationId"`
	AttemptID      string `json:"attemptId"`
	UsageHash      string `json:"usageHash"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type SettleInvocationResult struct {
	Entry       gen.LedgerEntry `json:"entry"`
	Reservation gen.Reservation `json:"reservation"`
}

func SettleInvocation(ctx context.Context, op OperationContext, input SettleInvocationInput) (SettleInvocationResult, error) {
	tx, err := gatewayruntime.BeginExecution(ctx, op, true)
	if err != nil {
		return SettleInvocationResult{}, err
	}
	defer tx.Close()
	var result SettleInvocationResult
	replay, err := gatewayruntime.ReplayExecutionReceipt(tx, op, "model-gateway.SettleInvocation", input.IdempotencyKey, input, &result)
	if err != nil || replay {
		return result, err
	}
	record, err := gatewayruntime.ReadInvocationRecord(tx, op, input.InvocationID)
	if err != nil {
		return result, err
	}
	attempt, err := gatewayruntime.ReadAttemptRecord(tx, input.AttemptID)
	if err != nil {
		return result, err
	}
	if attempt.Attempt.InvocationId != input.InvocationID {
		return result, platform.NewError(platform.CodeForbidden, false, "结算尝试不属于本调用")
	}
	usage, err := ReadUsage(tx, attempt, input.UsageHash)
	if err != nil {
		return result, err
	}
	reservation, err := ReadReservation(tx, attempt.Attempt.ReservationId)
	if err != nil {
		return result, err
	}
	if reservation.State != "held" {
		return result, platform.NewError(platform.CodeConflict, false, "此尝试已经结算或凭明确证明释放")
	}
	actual := usage.ReportedCost
	if actual == nil {
		actual, err = ComputeUsageCost(attempt.Price, usage.InputTokens, usage.OutputTokens)
		if err != nil {
			return result, err
		}
	}
	entry := gen.LedgerEntry{EntryId: "ledger." + input.AttemptID, InvocationId: input.InvocationID, AttemptId: input.AttemptID, ProviderRequestId: usage.ProviderRequestID, InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens, PriceSnapshot: attempt.Price, ActualCost: actual, PricingKnown: actual != nil, Status: gen.LedgerEntryStatus("settled"), CreatedAt: time.Now().UTC()}
	if actual == nil {
		entry.Status = gen.LedgerEntryStatus("price_unknown")
	}
	if err := InsertLedgerEntry(tx, entry, usage.UsageHash, attempt.Price, ""); err != nil {
		return result, err
	}
	var zero, amount any
	unknown := 0
	if reservation.PricingKnown {
		zero = 0
	}
	if actual != nil {
		amount = actual.Micros
	}
	if !attempt.Price.PricingKnown {
		unknown = 1
	}
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE reservations SET state='settled',held_micro_usd=?,version=version+1,unknown_usage_count=unknown_usage_count+? WHERE reservation_id=?`, zero, unknown, reservation.ReservationId); err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	// 已知价预留即使供应方最终金额未知也仍保持其原pricingKnown结构，held归零；账单未知单独记录。
	if reservation.PricingKnown {
		if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE reservations SET held_micro_usd=0 WHERE reservation_id=?`, reservation.ReservationId); err != nil {
			return result, gatewayruntime.ExecutionStorageError(err)
		}
	}
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE fee_commitments SET state='settled',amount_micro_usd=?,pricing_known=? WHERE reservation_id=?`, amount, actual != nil, reservation.ReservationId); err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	payload := struct {
		InvocationID string     `json:"invocationId"`
		AttemptID    string     `json:"attemptId"`
		Root         string     `json:"rootOperationId"`
		CostClass    string     `json:"costClass"`
		EntryID      string     `json:"entryId"`
		Actual       *gen.Money `json:"actualCost,omitempty"`
		Known        bool       `json:"pricingKnown"`
		Epoch        string     `json:"dataEpoch"`
	}{input.InvocationID, input.AttemptID, record.Invocation.RootOperationId, string(record.Invocation.CostClass), entry.EntryId, actual, actual != nil, op.DataEpoch}
	if err := gatewayruntime.EnqueueExecutionEvent(tx, record.Operation, "invocation.settled", input.InvocationID, len(record.Invocation.Attempts), payload); err != nil {
		return result, err
	}
	result.Entry = entry
	result.Reservation, err = ReadReservation(tx, reservation.ReservationId)
	if err != nil {
		return result, err
	}
	if err := gatewayruntime.SaveExecutionReceipt(tx, op, "model-gateway.SettleInvocation", input.IdempotencyKey, input, result); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return SettleInvocationResult{}, err
	}
	return result, nil
}
func InsertLedgerEntry(tx *gatewayruntime.ExecutionTransaction, entry gen.LedgerEntry, hash string, price gen.PriceSnapshot, supersedes string) error {
	var amount, previous any
	if entry.ActualCost != nil {
		amount = entry.ActualCost.Micros
	}
	if supersedes != "" {
		previous = supersedes
	}
	raw, _ := platform.CanonicalJSON(price)
	_, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO ledger_entries(entry_id,invocation_id,provider_request_id,actual_micro_usd,input_tokens,output_tokens,status,created_at,attempt_id,pricing_known,usage_hash,supersedes_entry_id,price_snapshot_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, entry.EntryId, entry.InvocationId, entry.ProviderRequestId, amount, entry.InputTokens, entry.OutputTokens, entry.Status, entry.CreatedAt.UTC().Format(time.RFC3339Nano), entry.AttemptId, entry.PricingKnown, hash, previous, raw)
	return gatewayruntime.ExecutionStorageError(err)
}
func ReadAttemptLedger(tx *gatewayruntime.ExecutionTransaction, attemptID string) (gen.LedgerEntry, error) {
	var entry gen.LedgerEntry
	var amount sql.NullInt64
	var created, status string
	var priceRaw []byte
	var known bool
	err := tx.Tx.QueryRowContext(tx.Context, `SELECT l.entry_id,l.invocation_id,l.provider_request_id,l.actual_micro_usd,l.input_tokens,l.output_tokens,l.status,l.created_at,l.attempt_id,l.pricing_known,l.price_snapshot_json FROM ledger_entries l WHERE l.attempt_id=? AND NOT EXISTS(SELECT 1 FROM ledger_entries n WHERE n.supersedes_entry_id=l.entry_id)`, attemptID).Scan(&entry.EntryId, &entry.InvocationId, &entry.ProviderRequestId, &amount, &entry.InputTokens, &entry.OutputTokens, &status, &created, &entry.AttemptId, &known, &priceRaw)
	if err == sql.ErrNoRows {
		return entry, platform.NewError(platform.CodeNotFound, false, "独立尝试尚无账单")
	}
	if err != nil {
		return entry, gatewayruntime.ExecutionStorageError(err)
	}
	parsed, err := platform.ParseTimestamp(created)
	if err != nil {
		return entry, platform.NewError(platform.CodeIntegrityError, false, "账单时间损坏")
	}
	if json.Unmarshal(priceRaw, &entry.PriceSnapshot) != nil {
		return entry, platform.NewError(platform.CodeIntegrityError, false, "账单固定价目损坏")
	}
	entry.CreatedAt = parsed
	entry.Status = gen.LedgerEntryStatus(status)
	entry.PricingKnown = known
	if known != amount.Valid {
		return entry, platform.NewError(platform.CodeIntegrityError, false, "账单金额与价目状态矛盾")
	}
	if amount.Valid {
		entry.ActualCost = &gen.Money{Currency: "USD", Micros: int(amount.Int64)}
	}
	return entry, nil
}
