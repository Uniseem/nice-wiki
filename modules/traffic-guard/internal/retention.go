package trafficguard

import (
	"context"
	"time"
)

type PurgeAnonymousSignalsInput struct{ NowMs int64 }
type PurgeAnonymousSignalsResult struct {
	DeletedCount int64 `json:"deletedCount"`
}

// PurgeAnonymousSignals 最多清理500个主体记录（租约的网络关联不另计主体），只使用本文件固定SQL，不删除活跃禁令或租约。
func PurgeAnonymousSignals(ctx context.Context, op OperationContext, in PurgeAnonymousSignalsInput) (PurgeAnonymousSignalsResult, error) {
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return PurgeAnonymousSignalsResult{}, err
	}
	defer t.close()
	now := time.UnixMilli(in.NowMs)
	remaining := int64(500)
	operations := []struct {
		query  string
		cutoff int64
	}{
		{`DELETE FROM guard_challenges WHERE rowid IN (SELECT rowid FROM guard_challenges WHERE site_id=? AND data_epoch=? AND expires_at_ns<=? ORDER BY expires_at_ns LIMIT ?)`, now.UnixNano()},
		{`DELETE FROM guard_question_signals WHERE rowid IN (SELECT rowid FROM guard_question_signals WHERE site_id=? AND data_epoch=? AND occurred_at_ns<=? ORDER BY occurred_at_ns LIMIT ?)`, now.Add(-10 * time.Minute).UnixNano()},
		{`DELETE FROM guard_risk_events WHERE rowid IN (SELECT rowid FROM guard_risk_events WHERE site_id=? AND data_epoch=? AND occurred_at_ns<=? ORDER BY occurred_at_ns LIMIT ?)`, now.Add(-30 * 24 * time.Hour).UnixNano()},
		{`DELETE FROM guard_buckets WHERE rowid IN (SELECT rowid FROM guard_buckets WHERE site_id=? AND data_epoch=? AND updated_at_ms<=? ORDER BY updated_at_ms LIMIT ?)`, now.Add(-24 * time.Hour).UnixMilli()},
		{`DELETE FROM guard_bans WHERE rowid IN (SELECT rowid FROM guard_bans WHERE site_id=? AND data_epoch=? AND state<>'active' AND expires_at_ns<=? ORDER BY expires_at_ns LIMIT ?)`, now.Add(-30 * 24 * time.Hour).UnixNano()},
	}
	for _, operation := range operations {
		if remaining == 0 {
			break
		}
		changed, err := t.tx.ExecContext(t.ctx, operation.query, t.site, op.DataEpoch, operation.cutoff, remaining)
		if err != nil {
			return PurgeAnonymousSignalsResult{}, loginDBError()
		}
		count, err := changed.RowsAffected()
		if err != nil {
			return PurgeAnonymousSignalsResult{}, loginDBError()
		}
		remaining -= count
	}
	// 先删除租约子表，再删终局行；活跃租约永远不在候选集合内。
	if remaining > 0 {
		_, err = t.tx.ExecContext(t.ctx, `DELETE FROM guard_permit_networks WHERE admission_id IN
   (SELECT admission_id FROM guard_permits WHERE site_id=? AND data_epoch=? AND state<>'active' AND expires_at_ns<=? ORDER BY expires_at_ns LIMIT ?)`, t.site, op.DataEpoch, now.Add(-7*24*time.Hour).UnixNano(), remaining)
		if err != nil {
			return PurgeAnonymousSignalsResult{}, loginDBError()
		}
		changed, err := t.tx.ExecContext(t.ctx, `DELETE FROM guard_permits WHERE admission_id IN
   (SELECT admission_id FROM guard_permits WHERE site_id=? AND data_epoch=? AND state<>'active' AND expires_at_ns<=? ORDER BY expires_at_ns LIMIT ?)`, t.site, op.DataEpoch, now.Add(-7*24*time.Hour).UnixNano(), remaining)
		if err != nil {
			return PurgeAnonymousSignalsResult{}, loginDBError()
		}
		count, err := changed.RowsAffected()
		if err != nil {
			return PurgeAnonymousSignalsResult{}, loginDBError()
		}
		remaining -= count
	}
	if err = t.commit(); err != nil {
		return PurgeAnonymousSignalsResult{}, err
	}
	return PurgeAnonymousSignalsResult{500 - remaining}, nil
}
