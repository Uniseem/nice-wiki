package trafficguard

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
	"time"
)

type ReportQuestionSignalsInput struct {
	SessionKey      string `json:"sessionKey"`
	NetworkKey      string `json:"networkKey"`
	QuestionHash    string `json:"questionHash"`
	SimilarityScore *int64 `json:"similarityScore,omitempty"`
	IntervalMs      *int64 `json:"intervalMs,omitempty"`
	ModelVerdict    string `json:"modelVerdict"`
	DataEpoch       string `json:"-"`
}
type ReportQuestionSignalsResult struct {
	Decision     string `json:"decision"`
	BanExpiresAt string `json:"banExpiresAt,omitempty"`
}

// ReportQuestionSignals 只接收QA主Agent的滥用信号，不重新调用模型，不持久问题正文。
func ReportQuestionSignals(ctx context.Context, op OperationContext, in ReportQuestionSignalsInput) (ReportQuestionSignalsResult, error) {
	if op.CallerService != "qa" {
		return ReportQuestionSignalsResult{}, platform.NewError(platform.CodeForbidden, false, "仅QA可提交既有风险信号")
	}
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return ReportQuestionSignalsResult{}, err
	}
	defer t.close()
	originalOperation := t.op.OperationID
	t.op.OperationID = op.RootOperationID
	var result ReportQuestionSignalsResult
	found, err := readGuardReceipt(t, "signals.report", in, &result)
	t.op.OperationID = originalOperation
	if err != nil {
		return ReportQuestionSignalsResult{}, err
	}
	if found {
		return result, nil
	}
	var subject, ip string
	err = t.tx.QueryRowContext(t.ctx, `SELECT subject_key,ip FROM guard_permits WHERE site_id=? AND data_epoch=? AND owner_caller='qa' AND action='qa.ask' AND request_id=? AND session_key=? AND network_key=? AND ip<>''`, t.site, op.DataEpoch, op.RootOperationID, in.SessionKey, in.NetworkKey).Scan(&subject, &ip)
	if errors.Is(err, sql.ErrNoRows) {
		return ReportQuestionSignalsResult{}, platform.NewError(platform.CodeNotFound, false, "已验证匿名会话与网络引用不存在")
	}
	if err != nil {
		return ReportQuestionSignalsResult{}, loginDBError()
	}
	digest, _ := platform.CanonicalJSON(in)
	_, err = t.tx.ExecContext(t.ctx, `INSERT INTO guard_question_signals(signal_id,site_id,data_epoch,question_operation_id,input_hash,session_key,network_key,question_hash,similarity_score,interval_ms,model_verdict,occurred_at_ns,ip)
  VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, platform.NewIdentifier(), t.site, op.DataEpoch, op.RootOperationID, platform.HashBytes(digest), in.SessionKey, in.NetworkKey, in.QuestionHash, in.SimilarityScore, in.IntervalMs, in.ModelVerdict, t.now.UnixNano(), ip)
	if err != nil {
		return ReportQuestionSignalsResult{}, loginDBError()
	}
	suspiciousUntil := int64(0)
	if in.ModelVerdict != "normal" {
		suspiciousUntil = t.now.Add(10 * time.Minute).UnixNano()
	}
	_, err = t.tx.ExecContext(t.ctx, `UPDATE guard_subjects SET model_verdict=?,
  suspicious_until_ns=MAX(suspicious_until_ns,?),updated_at_ns=? WHERE site_id=? AND data_epoch=? AND subject_key=?`, in.ModelVerdict, suspiciousUntil, t.now.UnixNano(), t.site, op.DataEpoch, subject)
	if err != nil {
		return ReportQuestionSignalsResult{}, loginDBError()
	}
	result = ReportQuestionSignalsResult{Decision: "allow", BanExpiresAt: ""}
	var attacks int
	if err = t.tx.QueryRowContext(t.ctx, `SELECT COUNT(DISTINCT question_operation_id) FROM guard_question_signals WHERE site_id=? AND data_epoch=? AND ip=? AND model_verdict='abuse' AND occurred_at_ns>?`, t.site, op.DataEpoch, ip, t.now.Add(-10*time.Minute).UnixNano()).Scan(&attacks); err != nil {
		return ReportQuestionSignalsResult{}, loginDBError()
	}
	if in.ModelVerdict == "abuse" && attacks >= 3 {
		if _, err = createIPBan(t, ip, 600, "abuse"); err != nil {
			return ReportQuestionSignalsResult{}, err
		}
		result.Decision = "deny"
		result.BanExpiresAt = t.now.Add(10 * time.Minute).Format(time.RFC3339Nano)
	}
	signals, err := LoadSignals(t.ctx, op, LoadSignalsInput{subject, "qa.ask", t.now.UnixMilli()})
	if err != nil {
		return ReportQuestionSignalsResult{}, err
	}
	policy, _, err := readGuardPolicy(t)
	if err != nil {
		return ReportQuestionSignalsResult{}, err
	}
	// 站点没开挑战时没有可通过的挑战，视同已通过（与准入判定一致），否则每个问题都会被要求验证。
	verdict := signals.ModelVerdict
	if verdict == "abuse" && attacks < 3 {
		verdict = "suspicious"
		if _, err = t.tx.ExecContext(t.ctx, `UPDATE guard_subjects SET model_verdict='suspicious' WHERE site_id=? AND data_epoch=? AND subject_key=?`, t.site, op.DataEpoch, subject); err != nil {
			return ReportQuestionSignalsResult{}, loginDBError()
		}
	}
	risk, err := ComputeRisk(t.ctx, op, ComputeRiskInput{Denials10m: signals.Denials10m, ChallengePassed: signals.ChallengePassed || !policy.ChallengeEnabled, ActiveBan: signals.ActiveBan, ExactRepeatCount: signals.ExactRepeatCount, SimilarityScore: signals.SimilarityScore, RegularIntervalCount: signals.RegularIntervalCount, ModelVerdict: verdict})
	if err != nil {
		return ReportQuestionSignalsResult{}, err
	}
	if result.BanExpiresAt == "" {
		result.Decision = risk.Decision
	}
	if signals.ActiveBan {
		var expires int64
		if t.tx.QueryRowContext(t.ctx, `SELECT MAX(expires_at_ns) FROM guard_bans WHERE site_id=? AND data_epoch=? AND state='active' AND expires_at_ns>? AND network_key IN (SELECT network_key FROM guard_network_aliases WHERE site_id=? AND data_epoch=? AND subject_key=?)`, t.site, op.DataEpoch, t.now.UnixNano(), t.site, op.DataEpoch, subject).Scan(&expires) != nil {
			return ReportQuestionSignalsResult{}, loginDBError()
		}
		result.BanExpiresAt = time.Unix(0, expires).UTC().Format(time.RFC3339Nano)
	}
	_, err = t.tx.ExecContext(t.ctx, `DELETE FROM guard_question_signals WHERE site_id=? AND data_epoch=? AND session_key=?
  AND signal_id NOT IN (SELECT signal_id FROM guard_question_signals WHERE site_id=? AND data_epoch=? AND session_key=? AND occurred_at_ns>?
  ORDER BY occurred_at_ns DESC,signal_id DESC LIMIT 8)`, t.site, op.DataEpoch, in.SessionKey, t.site, op.DataEpoch, in.SessionKey, t.now.Add(-10*time.Minute).UnixNano())
	if err != nil {
		return ReportQuestionSignalsResult{}, loginDBError()
	}
	if in.ModelVerdict != "normal" {
		_, err = t.tx.ExecContext(t.ctx, `INSERT INTO guard_risk_events(event_id,site_id,data_epoch,action,decision,network_key,occurred_at_ns,ip) VALUES(?,?,?,'qa.agent',?,?,?,?)`, platform.NewIdentifier(), t.site, op.DataEpoch, result.Decision, in.NetworkKey, t.now.UnixNano(), ip)
		if err != nil {
			return ReportQuestionSignalsResult{}, loginDBError()
		}
	}
	t.op.OperationID = op.RootOperationID
	if err = saveGuardReceipt(t, "signals.report", in, result); err != nil {
		return ReportQuestionSignalsResult{}, err
	}
	t.op.OperationID = originalOperation
	if err = t.commit(); err != nil {
		return ReportQuestionSignalsResult{}, err
	}
	return result, nil
}
