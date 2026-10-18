package analytics

import (
	"context"
	"database/sql"
	"nice-wiki/libs/platform"
	"time"
)

// QueryUsageStatistics只汇总本域已有已知账项投影，水位与未对账状态明确返回。
func QueryUsageStatistics(ctx context.Context, op OperationContext, in QueryUsageStatisticsInput) (QueryUsageStatisticsResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return QueryUsageStatisticsResult{}, err
	}
	var complete int
	if err = rt.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM aggregation_watermarks WHERE source='usage-v2-complete'`).Scan(&complete); err != nil {
		return QueryUsageStatisticsResult{}, storageError(err)
	}
	if complete != 1 {
		return QueryUsageStatisticsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "完整费用上界及未入账负债尚未投影，不把已入账部分或0当全站费用")
	}
	interval, err := ValidateDateRange(ctx, op, ValidateDateRangeInput{From: in.From, To: in.To, MaxDays: 366})
	if err != nil {
		return QueryUsageStatisticsResult{}, err
	}
	begin, _ := time.Parse(time.RFC3339Nano, interval.FromUTC)
	end, _ := time.Parse(time.RFC3339Nano, interval.ToUTC)
	tx, err := rt.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return QueryUsageStatisticsResult{}, storageError(err)
	}
	defer tx.Rollback()
	result := QueryUsageStatisticsResult{Entries: []UsageEntry{}}
	if err = tx.QueryRowContext(ctx, `SELECT coalesce((SELECT revision FROM aggregation_watermarks WHERE source='usage'),0),coalesce((SELECT updated_at FROM aggregation_watermarks WHERE source='usage-reconciled'),'')`).Scan(&result.Watermark, &result.LastReconciledAt); err != nil {
		return QueryUsageStatisticsResult{}, storageError(err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT ledger_entry_id,ledger_revision,operation_id,cost_class,amount_micros,currency,state,occurred_at FROM usage_projection WHERE occurred_at>=? AND occurred_at<? AND (?='' OR cost_class=?) AND data_epoch=? ORDER BY occurred_at,ledger_entry_id LIMIT 10001`, begin.Format(analyticsTimeLayout), end.Format(analyticsTimeLayout), in.CostClass, in.CostClass, rt.Epoch)
	if err != nil {
		return QueryUsageStatisticsResult{}, storageError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var entry UsageEntry
		if err = rows.Scan(&entry.LedgerEntryID, &entry.LedgerRevision, &entry.OperationID, &entry.CostClass, &entry.Amount.Micros, &entry.Amount.Currency, &entry.State, &entry.OccurredAt); err != nil {
			return QueryUsageStatisticsResult{}, storageError(err)
		}
		if entry.Amount.Currency != "USD" || entry.Amount.Micros < 0 || entry.State == "price_unknown" || entry.State == "charge_unknown" {
			return QueryUsageStatisticsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "现有投影含未知费用，不能用0或已知部分充总额")
		}
		if result.AmountMicros > 9007199254740991-entry.Amount.Micros {
			return QueryUsageStatisticsResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "费用投影超过精确整数预算")
		}
		result.AmountMicros += entry.Amount.Micros
		result.Entries = append(result.Entries, entry)
	}
	if err = rows.Err(); err != nil {
		return QueryUsageStatisticsResult{}, storageError(err)
	}
	if len(result.Entries) > 10000 {
		return QueryUsageStatisticsResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "费用投影超过响应预算")
	}
	return result, nil
}
