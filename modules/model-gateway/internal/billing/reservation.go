package billing

import (
	"context"
	"database/sql"
	"math/big"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"time"
)

type ReserveCostInput struct {
	InvocationID    string                        `json:"invocationId"`
	AttemptID       string                        `json:"attemptId"`
	MaximumCost     *gen.Money                    `json:"maximumCost,omitempty"`
	PricingKnown    bool                          `json:"pricingKnown"`
	IdempotencyKey  string                        `json:"idempotencyKey"`
	PreparedAttempt *gatewayruntime.AttemptRecord `json:"preparedAttempt,omitempty"`
}
type ReserveCostResult struct {
	Reservation gen.Reservation `json:"reservation"`
}
type ExtendReservationInput struct {
	ReservationID   string    `json:"reservationId"`
	AdditionalCost  gen.Money `json:"additionalCost"`
	ExpectedVersion int       `json:"expectedVersion"`
	IdempotencyKey  string    `json:"idempotencyKey"`
}
type ExtendReservationResult = ReserveCostResult

// ComputeUsageCost 对固定价目按两个token费率分别向上取整，再加固定费；缺价返回nil。
func ComputeUsageCost(price gen.PriceSnapshot, inputTokens, outputTokens int) (*gen.Money, error) {
	if !price.PricingKnown {
		return nil, nil
	}
	if price.FixedCost == nil || price.InputPerMillionCost == nil || price.OutputPerMillionCost == nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "已知价目缺少完整费率")
	}
	total := big.NewInt(int64(price.FixedCost.Micros))
	for _, part := range []struct{ Tokens, Rate int }{{inputTokens, price.InputPerMillionCost.Micros}, {outputTokens, price.OutputPerMillionCost.Micros}} {
		amount := new(big.Int).Mul(big.NewInt(int64(part.Tokens)), big.NewInt(int64(part.Rate)))
		amount.Add(amount, big.NewInt(999999))
		amount.Quo(amount, big.NewInt(1000000))
		total.Add(total, amount)
	}
	if total.Sign() < 0 || total.Cmp(big.NewInt(9007199254740991)) > 0 {
		return nil, platform.NewError(platform.CodeBudgetExceeded, false, "固定价目费用超出可表示范围")
	}
	return &gen.Money{Currency: "USD", Micros: int(total.Int64())}, nil
}
func ReadReservation(tx *gatewayruntime.ExecutionTransaction, id string) (gen.Reservation, error) {
	var result gen.Reservation
	var maximum, held sql.NullInt64
	var state string
	var known int
	err := tx.Tx.QueryRowContext(tx.Context, `SELECT reservation_id,invocation_id,state,maximum_micro_usd,held_micro_usd,version,pricing_known,unknown_usage_count FROM reservations WHERE reservation_id=?`, id).Scan(&result.ReservationId, &result.InvocationId, &state, &maximum, &held, &result.Version, &known, &result.UnknownUsageCount)
	if err == sql.ErrNoRows {
		return result, platform.NewError(platform.CodeNotFound, false, "费用预留不存在")
	}
	if err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	result.State = gen.ReservationState(state)
	result.PricingKnown = known == 1
	validState := state == "held" || state == "settled" || state == "released"
	if !validState || result.Version < 0 || result.Version > 9007199254740991 || result.UnknownUsageCount < 0 || result.UnknownUsageCount > 9007199254740991 || (maximum.Valid && (maximum.Int64 < 0 || maximum.Int64 > 9007199254740991)) || (held.Valid && (held.Int64 < 0 || held.Int64 > maximum.Int64)) {
		return result, platform.NewError(platform.CodeIntegrityError, false, "预留状态、金额或版本越界")
	}
	if known < 0 || known > 1 || maximum.Valid != result.PricingKnown || held.Valid != result.PricingKnown {
		return result, platform.NewError(platform.CodeIntegrityError, false, "费用预留金额与价目状态不一致")
	}
	if maximum.Valid {
		result.MaximumCost = &gen.Money{Currency: "USD", Micros: int(maximum.Int64)}
		result.HeldCost = &gen.Money{Currency: "USD", Micros: int(held.Int64)}
	}
	return result, nil
}

// CheckFeeBudgets 必须在调用方短写事务内执行，所有held含旧日未知继续占同scope。
func CheckFeeBudgets(tx *gatewayruntime.ExecutionTransaction, record gatewayruntime.InvocationRecord, additional *gen.Money, now time.Time) error {
	profile := record.Profile
	if profile.MaximumCost.Micros > 0 {
		if additional == nil {
			return platform.NewError(platform.CodeBudgetExceeded, false, "有限请求额度不能接受本次未知最大费用")
		}
		rows, err := tx.Tx.QueryContext(tx.Context, `SELECT f.amount_micro_usd FROM fee_commitments f JOIN reservations r ON r.reservation_id=f.reservation_id WHERE r.invocation_id=? AND f.state IN ('held','settled')`, record.Invocation.InvocationId)
		if err != nil {
			return gatewayruntime.ExecutionStorageError(err)
		}
		sum := big.NewInt(int64(additional.Micros))
		for rows.Next() {
			var amount sql.NullInt64
			if err := rows.Scan(&amount); err != nil {
				rows.Close()
				return gatewayruntime.ExecutionStorageError(err)
			}
			if !amount.Valid {
				rows.Close()
				return platform.NewError(platform.CodeBudgetExceeded, false, "原调用已有未知费用，不能证明请求额度足够")
			}
			if amount.Int64 < 0 || amount.Int64 > 9007199254740991 {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "费用投影金额越界")
			}
			sum.Add(sum, big.NewInt(amount.Int64))
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return gatewayruntime.ExecutionStorageError(err)
		}
		if sum.Cmp(big.NewInt(int64(profile.MaximumCost.Micros))) > 0 {
			return platform.NewError(platform.CodeBudgetExceeded, false, "原调用全部独立尝试超过档位请求费用上限")
		}
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).UnixNano()
	hour := now.Truncate(time.Hour).UnixNano()
	type budget struct {
		Limit int64
		Scope string
		Value string
		Start int64
	}
	budgets := []budget{}
	if record.Probe != nil {
		var daily int64
		var site, epoch string
		err := tx.Tx.QueryRowContext(tx.Context, `SELECT site_id,data_epoch,daily_probe_limit_micro_usd FROM fee_budget_policy WHERE singleton=1`).Scan(&site, &epoch, &daily)
		if err != nil && err != sql.ErrNoRows {
			return gatewayruntime.ExecutionStorageError(err)
		}
		if err == nil && (site != record.Operation.Actor.SiteID || epoch != record.Operation.DataEpoch) {
			return platform.NewError(platform.CodeStaleResult, false, "诊断费用策略站点/代际变化")
		}
		budgets = append(budgets, budget{daily, "probe_lane", platform.CostClassAdminProbe, day})
	} else {
		budgets = append(budgets, budget{int64(profile.DailyLimitCost.Micros), "profile_id", profile.ProfileId, day})
	}
	if record.Operation.CostClass == platform.CostClassQA {
		if record.Binding == nil {
			return platform.NewError(platform.CodeForbidden, false, "QA费用缺少已验证签名范围")
		}
		var enabled bool
		var sessionLimit, multiplier, daily int64
		var site, epoch string
		if err := tx.Tx.QueryRowContext(tx.Context, `SELECT site_id,data_epoch,hourly_limits_enabled,session_hourly_limit_micro_usd,network_multiplier,daily_qa_limit_micro_usd FROM fee_budget_policy WHERE singleton=1`).Scan(&site, &epoch, &enabled, &sessionLimit, &multiplier, &daily); err != nil && err != sql.ErrNoRows {
			return gatewayruntime.ExecutionStorageError(err)
		} else if err == nil && (site != record.Operation.Actor.SiteID || epoch != record.Operation.DataEpoch) {
			return platform.NewError(platform.CodeStaleResult, false, "费用策略站点或代际不匹配")
		}
		budgets = append(budgets, budget{daily, "cost_class", "qa", day})
		if enabled && sessionLimit > 0 {
			network := new(big.Int).Mul(big.NewInt(sessionLimit), big.NewInt(multiplier))
			if !network.IsInt64() {
				return platform.NewError(platform.CodeIntegrityError, false, "网络费用额度超出整数范围")
			}
			budgets = append(budgets, budget{sessionLimit, "session_scope", record.Binding.SessionBudgetScope, hour}, budget{network.Int64(), "network_scope", record.Binding.NetworkBudgetScope, hour})
		}
	}
	for _, bound := range budgets {
		if bound.Limit == 0 {
			continue
		}
		if additional == nil {
			return platform.NewError(platform.CodeBudgetExceeded, false, "有限作用域额度不能接受本次未知最大费用")
		}
		scope := bound.Scope + `=?`
		if bound.Scope == "probe_lane" {
			scope = `cost_class=? AND reservation_id IN (SELECT r.reservation_id FROM reservations r JOIN connection_probes p ON p.invocation_id=r.invocation_id)`
		}
		rows, err := tx.Tx.QueryContext(tx.Context, `SELECT amount_micro_usd FROM fee_commitments WHERE site_id=? AND data_epoch=? AND `+scope+` AND (state='held' OR (state='settled' AND created_ns>=?))`, record.Operation.Actor.SiteID, record.Operation.DataEpoch, bound.Value, bound.Start)
		if err != nil {
			return gatewayruntime.ExecutionStorageError(err)
		}
		total := big.NewInt(int64(additional.Micros))
		for rows.Next() {
			var amount sql.NullInt64
			if err := rows.Scan(&amount); err != nil {
				rows.Close()
				return gatewayruntime.ExecutionStorageError(err)
			}
			if !amount.Valid {
				rows.Close()
				return platform.NewError(platform.CodeBudgetExceeded, false, "费用作用域含未知金额，不能证明有限额度足够")
			}
			if amount.Int64 < 0 || amount.Int64 > 9007199254740991 {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "费用投影金额越界")
			}
			total.Add(total, big.NewInt(amount.Int64))
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return gatewayruntime.ExecutionStorageError(err)
		}
		if total.Cmp(big.NewInt(bound.Limit)) > 0 {
			return platform.NewError(platform.CodeBudgetExceeded, false, "费用预留超过当前作用域额度")
		}
	}
	return nil
}
func ReserveCost(ctx context.Context, op OperationContext, input ReserveCostInput) (ReserveCostResult, error) {
	tx, err := gatewayruntime.BeginExecution(ctx, op, true)
	if err != nil {
		return ReserveCostResult{}, err
	}
	defer tx.Close()
	var result ReserveCostResult
	replay, err := gatewayruntime.ReplayExecutionReceipt(tx, op, "model-gateway.ReserveCost", input.IdempotencyKey, input, &result)
	if err != nil || replay {
		return result, err
	}
	record, err := gatewayruntime.ReadInvocationRecord(tx, op, input.InvocationID)
	if err != nil {
		return result, err
	}
	var attempt gatewayruntime.AttemptRecord
	if input.PreparedAttempt != nil {
		attempt = *input.PreparedAttempt
		if attempt.Attempt.AttemptId != input.AttemptID || attempt.Attempt.InvocationId != input.InvocationID || attempt.Attempt.Ordinal != len(record.Invocation.Attempts)+1 || attempt.LeaseID != record.LeaseID || record.LeaseUntilNS <= time.Now().UnixNano() {
			return result, platform.NewError(platform.CodeStaleResult, false, "准备尝试与当前序列不一致")
		}
		record.ActiveAttemptID = input.AttemptID
	} else {
		attempt, err = gatewayruntime.ReadAttemptRecord(tx, input.AttemptID)
		if err != nil {
			return result, err
		}
	}
	if record.ActiveAttemptID != input.AttemptID || attempt.Attempt.InvocationId != input.InvocationID || string(attempt.Attempt.State) != "reserved" || record.CancelRequested {
		return result, platform.NewError(platform.CodeStaleResult, false, "预留不属于当前未取消尝试")
	}
	maximum, err := ComputeUsageCost(attempt.Price, attempt.InputTokenBudget, attempt.OutputTokenBudget)
	if err != nil {
		return result, err
	}
	if input.PricingKnown != (maximum != nil) || (maximum == nil) != (input.MaximumCost == nil) || (maximum != nil && *maximum != *input.MaximumCost) {
		return result, platform.NewError(platform.CodeIntegrityError, false, "费用预留与固定价目估算不一致")
	}
	if record.Operation.CostClass == platform.CostClassQA && record.Binding == nil {
		return result, platform.NewError(platform.CodeForbidden, false, "QA费用缺少已验证许可范围")
	}
	if err := CheckFeeBudgets(tx, record, maximum, time.Now().UTC()); err != nil {
		return result, err
	}
	if input.PreparedAttempt != nil {
		attemptRaw, _ := platform.CanonicalJSON(attempt.Attempt)
		modelRaw, _ := platform.CanonicalJSON(attempt.Model)
		providerRaw, _ := platform.CanonicalJSON(attempt.Provider)
		priceRaw, _ := platform.CanonicalJSON(attempt.Price)
		if _, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO invocation_attempts(attempt_id,invocation_id,ordinal,attempt_json,model_json,provider_json,credential_version,price_snapshot_json,lease_id,lease_until_ns,input_token_budget,output_token_budget) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, input.AttemptID, input.InvocationID, attempt.Attempt.Ordinal, attemptRaw, modelRaw, providerRaw, attempt.CredentialVersion, priceRaw, attempt.LeaseID, attempt.LeaseUntilNS, attempt.InputTokenBudget, attempt.OutputTokenBudget); err != nil {
			return result, gatewayruntime.ExecutionStorageError(err)
		}
		if _, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO credential_references(attempt_id,provider_id,credential_version,data_epoch) VALUES(?,?,?,?)`, input.AttemptID, attempt.Provider.ProviderId, attempt.CredentialVersion, op.DataEpoch); err != nil {
			return result, gatewayruntime.ExecutionStorageError(err)
		}
		record.Invocation.Attempts = append(record.Invocation.Attempts, attempt.Attempt)
	}
	id := "reservation." + input.AttemptID
	var amount any
	if maximum != nil {
		amount = maximum.Micros
	}
	_, err = tx.Tx.ExecContext(tx.Context, `INSERT INTO reservations(reservation_id,invocation_id,maximum_micro_usd,held_micro_usd,state,version,pricing_known,unknown_usage_count,attempt_id) VALUES(?,?,?,?,'held',1,?,?,?)`, id, input.InvocationID, amount, amount, maximum != nil, 0, input.AttemptID)
	if err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	session, network := "", ""
	if record.Binding != nil {
		session = record.Binding.SessionBudgetScope
		network = record.Binding.NetworkBudgetScope
	}
	_, err = tx.Tx.ExecContext(tx.Context, `INSERT INTO fee_commitments(reservation_id,attempt_id,site_id,data_epoch,cost_class,profile_id,root_operation_id,session_scope,network_scope,created_ns,state,amount_micro_usd,pricing_known) VALUES(?,?,?,?,?,?,?,?,?,?,'held',?,?)`, id, input.AttemptID, op.Actor.SiteID, op.DataEpoch, op.CostClass, record.Profile.ProfileId, op.RootOperationID, session, network, time.Now().UnixNano(), amount, maximum != nil)
	if err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	attempt.Attempt.ReservationId = id
	if err := gatewayruntime.SaveAttemptRecord(tx, attempt); err != nil {
		return result, err
	}
	record.Invocation.ReservationId = &id
	record.Invocation.PriceSnapshotId = attempt.Price.PriceSnapshotId
	priceJSON, _ := platform.CanonicalJSON(attempt.Price)
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE invocations SET price_snapshot_json=? WHERE invocation_id=?`, priceJSON, input.InvocationID); err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	record.Invocation.State = gen.InvocationState("reserved")
	for i := range record.Invocation.Attempts {
		if record.Invocation.Attempts[i].AttemptId == input.AttemptID {
			record.Invocation.Attempts[i] = attempt.Attempt
		}
	}
	if err := gatewayruntime.SaveInvocationRecord(tx, record); err != nil {
		return result, err
	}
	result.Reservation, err = ReadReservation(tx, id)
	if err != nil {
		return result, err
	}
	if err := gatewayruntime.SaveExecutionReceipt(tx, op, "model-gateway.ReserveCost", input.IdempotencyKey, input, result); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return ReserveCostResult{}, err
	}
	return result, nil
}
