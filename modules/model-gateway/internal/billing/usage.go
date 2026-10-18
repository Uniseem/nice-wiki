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

type RecordUsageInput struct {
	InvocationID      string     `json:"invocationId"`
	AttemptID         string     `json:"attemptId"`
	ProviderRequestID string     `json:"providerRequestId"`
	InputTokens       int        `json:"inputTokens"`
	OutputTokens      int        `json:"outputTokens"`
	ReportedCost      *gen.Money `json:"reportedCost,omitempty"`
	UsageHash         string     `json:"usageHash"`
	PricingComplete   bool       `json:"pricingComplete"`
}
type RecordUsageResult struct {
	Accepted  bool `json:"accepted"`
	Duplicate bool `json:"duplicate"`
}

// UsageDigest 明确用量证据摘要，仅含真实供应方收费ID/token/金额，不含正文或密钥。
func UsageDigest(input RecordUsageInput) string {
	raw, _ := platform.CanonicalJSON(struct {
		RequestID       string     `json:"providerRequestId"`
		InputTokens     int        `json:"inputTokens"`
		OutputTokens    int        `json:"outputTokens"`
		Cost            *gen.Money `json:"reportedCost,omitempty"`
		PricingComplete bool       `json:"pricingComplete"`
	}{input.ProviderRequestID, input.InputTokens, input.OutputTokens, input.ReportedCost, input.PricingComplete})
	return platform.HashBytes(raw)
}
func RecordUsage(ctx context.Context, op OperationContext, input RecordUsageInput) (RecordUsageResult, error) {
	if input.UsageHash != UsageDigest(input) {
		return RecordUsageResult{}, platform.NewError(platform.CodeIntegrityError, false, "供应方用量摘要与脱敏证据不一致")
	}
	tx, err := gatewayruntime.BeginFinalization(ctx, op)
	if err != nil {
		return RecordUsageResult{}, err
	}
	defer tx.Close()
	record, err := gatewayruntime.ReadInvocationRecord(tx, op, input.InvocationID)
	if err != nil {
		return RecordUsageResult{}, err
	}
	attempt, err := gatewayruntime.ReadAttemptRecord(tx, input.AttemptID)
	if err != nil {
		return RecordUsageResult{}, err
	}
	if attempt.Attempt.InvocationId != record.Invocation.InvocationId || attempt.DispatchedNS == 0 {
		return RecordUsageResult{}, platform.NewError(platform.CodeForbidden, false, "未派发尝试不能接受供应方用量")
	}
	var previousHash, previousAttempt string
	var previous []byte
	err = tx.Tx.QueryRowContext(tx.Context, `SELECT usage_hash,attempt_id,usage_json FROM invocation_usage WHERE provider_id=? AND provider_request_id=?`, attempt.Provider.ProviderId, input.ProviderRequestID).Scan(&previousHash, &previousAttempt, &previous)
	if err == nil {
		if previousHash == input.UsageHash && previousAttempt == input.AttemptID {
			return RecordUsageResult{Accepted: true, Duplicate: true}, nil
		}
		if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE invocation_usage SET conflict_hash=? WHERE provider_id=? AND provider_request_id=?`, input.UsageHash, attempt.Provider.ProviderId, input.ProviderRequestID); err != nil {
			return RecordUsageResult{}, gatewayruntime.ExecutionStorageError(err)
		}
		if err := tx.Commit(); err != nil {
			return RecordUsageResult{}, err
		}
		return RecordUsageResult{}, platform.NewError(platform.CodeIntegrityError, false, "相同供应方收费ID存在冲突用量，需人工对账")
	}
	if err != sql.ErrNoRows {
		return RecordUsageResult{}, gatewayruntime.ExecutionStorageError(err)
	}
	if attempt.Attempt.ProviderRequestId != nil && *attempt.Attempt.ProviderRequestId != input.ProviderRequestID {
		return RecordUsageResult{}, platform.NewError(platform.CodeIntegrityError, false, "供应方收费ID与本次尝试不一致")
	}
	raw, _ := platform.CanonicalJSON(input)
	_, err = tx.Tx.ExecContext(tx.Context, `INSERT INTO invocation_usage(provider_id,provider_request_id,invocation_id,attempt_id,usage_hash,usage_json,created_ns) VALUES(?,?,?,?,?,?,?)`, attempt.Provider.ProviderId, input.ProviderRequestID, input.InvocationID, input.AttemptID, input.UsageHash, raw, time.Now().UnixNano())
	if err != nil {
		return RecordUsageResult{}, gatewayruntime.ExecutionStorageError(err)
	}
	attempt.Attempt.ProviderRequestId = &input.ProviderRequestID
	attempt.UsageHash = input.UsageHash
	if err := gatewayruntime.SaveAttemptRecord(tx, attempt); err != nil {
		return RecordUsageResult{}, err
	}
	for i := range record.Invocation.Attempts {
		if record.Invocation.Attempts[i].AttemptId == input.AttemptID {
			record.Invocation.Attempts[i] = attempt.Attempt
		}
	}
	if record.ActiveAttemptID == input.AttemptID {
		record.Invocation.ProviderRequestId = &input.ProviderRequestID
	}
	if err := gatewayruntime.SaveInvocationRecord(tx, record); err != nil {
		return RecordUsageResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return RecordUsageResult{}, err
	}
	return RecordUsageResult{Accepted: true}, nil
}
func ReadUsage(tx *gatewayruntime.ExecutionTransaction, attempt gatewayruntime.AttemptRecord, hash string) (RecordUsageInput, error) {
	var raw []byte
	var conflict sql.NullString
	err := tx.Tx.QueryRowContext(tx.Context, `SELECT usage_json,conflict_hash FROM invocation_usage WHERE attempt_id=? AND usage_hash=?`, attempt.Attempt.AttemptId, hash).Scan(&raw, &conflict)
	if err == sql.ErrNoRows {
		return RecordUsageInput{}, platform.NewError(platform.CodeNotFound, false, "指定尝试的真实用量证据不存在")
	}
	if err != nil {
		return RecordUsageInput{}, gatewayruntime.ExecutionStorageError(err)
	}
	var result RecordUsageInput
	if conflict.Valid || json.Unmarshal(raw, &result) != nil || result.UsageHash != UsageDigest(result) || result.AttemptID != attempt.Attempt.AttemptId {
		return result, platform.NewError(platform.CodeIntegrityError, false, "用量证据冲突或持久记录损坏")
	}
	return result, nil
}
