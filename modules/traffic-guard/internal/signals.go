package trafficguard

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
	"time"
)

type LoadSignalsInput struct {
	SubjectKey string
	Action     string
	NowMs      int64
}
type LoadSignalsResult struct {
	Denials10m           int64
	ChallengePassed      bool
	ActiveBan            bool
	ExactRepeatCount     int64
	SimilarityScore      *int64
	RegularIntervalCount int64
	ModelVerdict         string
}

// LoadSignals 在同一快照中读取匿名信号；已过期ban不依赖异步sweeper才失效。
func LoadSignals(ctx context.Context, op OperationContext, in LoadSignalsInput) (LoadSignalsResult, error) {
	t, err := beginGuardTransaction(ctx, op, false)
	if err != nil {
		return LoadSignalsResult{}, err
	}
	defer t.close()
	var session, verdict string
	var passedUntil, suspiciousUntil int64
	var count, level int
	err = t.tx.QueryRowContext(t.ctx, `SELECT session_key,model_verdict,challenge_passed_until_ns,questions_since_challenge,challenge_level,suspicious_until_ns
  FROM guard_subjects WHERE site_id=? AND data_epoch=? AND subject_key=?`, t.site, op.DataEpoch, in.SubjectKey).Scan(&session, &verdict, &passedUntil, &count, &level, &suspiciousUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return LoadSignalsResult{}, platform.NewError(platform.CodeNotFound, false, "匿名主体不存在")
	}
	if err != nil {
		return LoadSignalsResult{}, loginDBError()
	}
	now := time.UnixMilli(in.NowMs)
	if suspiciousUntil > now.UnixNano() {
		verdict = "suspicious"
	}
	if verdict == "suspicious" && suspiciousUntil <= now.UnixNano() {
		verdict = "normal"
	}
	required := 18
	if verdict == "suspicious" {
		required = 22
	}
	result := LoadSignalsResult{ModelVerdict: verdict, ChallengePassed: passedUntil > now.UnixNano() && level >= required && (verdict != "suspicious" || count < 5)}
	var bans int
	err = t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM guard_bans WHERE site_id=? AND data_epoch=? AND scope='ai' AND state='active'
  AND starts_at_ns<=? AND expires_at_ns>? AND network_key IN
  (SELECT network_key FROM guard_network_aliases WHERE site_id=? AND data_epoch=? AND subject_key=?)`, t.site, op.DataEpoch, now.UnixNano(), now.UnixNano(), t.site, op.DataEpoch, in.SubjectKey).Scan(&bans)
	if err != nil {
		return LoadSignalsResult{}, loginDBError()
	}
	result.ActiveBan = bans > 0
	if verdict == "abuse" && !result.ActiveBan {
		result.ModelVerdict = "normal"
	}
	err = t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM guard_risk_events WHERE site_id=? AND data_epoch=? AND action=? AND occurred_at_ns>?
  AND network_key IN (SELECT network_key FROM guard_network_aliases WHERE site_id=? AND data_epoch=? AND subject_key=?)`, t.site, op.DataEpoch, in.Action, now.Add(-10*time.Minute).UnixNano(), t.site, op.DataEpoch, in.SubjectKey).Scan(&result.Denials10m)
	if err != nil {
		return LoadSignalsResult{}, loginDBError()
	}
	var similarity sql.NullInt64
	err = t.tx.QueryRowContext(t.ctx, `SELECT MAX(similarity_score) FROM guard_question_signals WHERE site_id=? AND data_epoch=? AND session_key=? AND occurred_at_ns>?`, t.site, op.DataEpoch, session, now.Add(-10*time.Minute).UnixNano()).Scan(&similarity)
	if err != nil {
		return LoadSignalsResult{}, loginDBError()
	}
	if similarity.Valid {
		value := similarity.Int64
		result.SimilarityScore = &value
	}
	for _, metric := range []struct {
		column string
		target *int64
	}{{"question_hash", &result.ExactRepeatCount}, {"interval_ms", &result.RegularIntervalCount}} {
		// 列名只取本文件固定两项，不拼入调用方字段或任意SQL。
		query := `SELECT COALESCE(MAX(n),0) FROM (SELECT COUNT(*) n FROM guard_question_signals WHERE site_id=? AND data_epoch=? AND session_key=? AND occurred_at_ns>? AND ` + metric.column + ` IS NOT NULL GROUP BY ` + metric.column + `)`
		if t.tx.QueryRowContext(t.ctx, query, t.site, op.DataEpoch, session, now.Add(-10*time.Minute).UnixNano()).Scan(metric.target) != nil {
			return LoadSignalsResult{}, loginDBError()
		}
	}
	return result, nil
}
