package trafficguard

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"time"
)

type IssueChallengeInput struct {
	SubjectKey string
	RequestID  string
	Method     string
	DataEpoch  string
}
type IssueChallengeResult struct {
	ChallengeID string `json:"challengeId"`
	Nonce       string `json:"nonce"`
	Difficulty  int    `json:"difficulty"`
	ExpiresAt   string `json:"expiresAt"`
	Method      string `json:"method"`
	SiteKey     string `json:"siteKey"`
	Interactive bool   `json:"interactive"`
}
type VerifyChallengeInput struct {
	ChallengeID string
	SubjectKey  string
	Proof       string
	DataEpoch   string
	SessionID   string
}
type VerifyChallengeResult struct {
	PassedToken string `json:"passedToken"`
	ExpiresAt   string `json:"expiresAt"`
}
type challengeRecord struct {
	id, subject, session, request, inputHash, method, nonceHash, state string
	difficulty, attempts                                               int
	expires, created                                                   int64
}

// IssueChallenge 只保存nonce哈希；相同请求重构原nonce，不新增活跃挑战。
func IssueChallenge(ctx context.Context, op OperationContext, in IssueChallengeInput) (IssueChallengeResult, error) {
	if op.CallerService != "portal" {
		return IssueChallengeResult{}, platform.NewError(platform.CodeForbidden, false, "挑战仅接受可信入口")
	}
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return IssueChallengeResult{}, err
	}
	defer t.close()
	policy, _, err := readGuardPolicy(t)
	if err != nil {
		return IssueChallengeResult{}, err
	}
	switch in.Method {
	case "pow":
		if !policy.ChallengeEnabled {
			return IssueChallengeResult{}, platform.NewError(platform.CodeValidation, false, "工作量挑战未启用")
		}
	case "turnstile":
		if !policy.ExternalChallengeEnabled {
			return IssueChallengeResult{}, platform.NewError(platform.CodeValidation, false, "外部挑战未启用")
		}
		if t.runtime.Turnstile.SiteKey == "" || t.runtime.Turnstile.SecretRef == "" || t.runtime.Turnstile.ExpectedHostname == "" {
			return IssueChallengeResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "外部挑战配置未装配")
		}
	default:
		return IssueChallengeResult{}, platform.NewError(platform.CodeValidation, false, "未知挑战方式")
	}
	encoded, _ := platform.CanonicalJSON(in)
	digest := platform.HashBytes(encoded)
	var id string
	err = t.tx.QueryRowContext(t.ctx, `SELECT challenge_id FROM guard_challenges WHERE site_id=? AND data_epoch=? AND subject_key=? AND request_id=?`, t.site, op.DataEpoch, in.SubjectKey, in.RequestID).Scan(&id)
	if err == nil {
		row, err := readChallengeRecord(t, id)
		if err != nil {
			return IssueChallengeResult{}, err
		}
		if row.inputHash != digest {
			return IssueChallengeResult{}, platform.NewError(platform.CodeConflict, false, "同挑战请求已绑定其他方式")
		}
		if err = checkIssuedChallenge(row, t.now); err != nil {
			return IssueChallengeResult{}, err
		}
		nonce, err := challengeNonce(t.runtime, t.site, op.DataEpoch, row)
		if err != nil {
			return IssueChallengeResult{}, err
		}
		return challengeResult(t.runtime, row, nonce), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return IssueChallengeResult{}, loginDBError()
	}
	signals, err := LoadSignals(t.ctx, op, LoadSignalsInput{SubjectKey: in.SubjectKey, Action: "qa.ask", NowMs: t.now.UnixMilli()})
	if err != nil {
		return IssueChallengeResult{}, err
	}
	risk, err := ComputeRisk(t.ctx, op, ComputeRiskInput{ChallengePassed: signals.ChallengePassed, ActiveBan: signals.ActiveBan, ModelVerdict: signals.ModelVerdict})
	if err != nil {
		return IssueChallengeResult{}, err
	}
	if risk.Decision == "deny" {
		return IssueChallengeResult{}, platform.NewError(platform.CodeForbidden, false, "该网络AI功能暂被限制")
	}
	var recent, active int
	if t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM guard_challenges WHERE site_id=? AND data_epoch=? AND subject_key=? AND created_at_ns>?`, t.site, op.DataEpoch, in.SubjectKey, t.now.Add(-time.Minute).UnixNano()).Scan(&recent) != nil {
		return IssueChallengeResult{}, loginDBError()
	}
	// active上限包含前一分钟创建且仍在120秒有效期内的挑战。
	if t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM guard_challenges WHERE site_id=? AND data_epoch=? AND subject_key=? AND state='issued' AND expires_at_ns>?`, t.site, op.DataEpoch, in.SubjectKey, t.now.UnixNano()).Scan(&active) != nil {
		return IssueChallengeResult{}, loginDBError()
	}
	if recent >= 6 || active >= 2 {
		return IssueChallengeResult{}, platform.NewError(platform.CodeRateLimited, true, "挑战请求过于频繁")
	}
	var session string
	if t.tx.QueryRowContext(t.ctx, `SELECT session_key FROM guard_subjects WHERE site_id=? AND data_epoch=? AND subject_key=?`, t.site, op.DataEpoch, in.SubjectKey).Scan(&session) != nil {
		return IssueChallengeResult{}, loginDBError()
	}
	row := challengeRecord{id: platform.NewIdentifier(), subject: in.SubjectKey, session: session, request: in.RequestID, inputHash: digest, method: in.Method, difficulty: risk.Difficulty, expires: t.now.Add(120 * time.Second).UnixNano(), created: t.now.UnixNano(), state: "issued"}
	nonce, err := challengeNonce(t.runtime, t.site, op.DataEpoch, row)
	if err != nil {
		return IssueChallengeResult{}, err
	}
	row.nonceHash = platform.HashBytes([]byte(nonce))
	_, err = t.tx.ExecContext(t.ctx, `INSERT INTO guard_challenges(challenge_id,site_id,data_epoch,subject_key,session_key,request_id,input_hash,method,difficulty,nonce_hash,expires_at_ns,created_at_ns,state)
  VALUES(?,?,?,?,?,?,?,?,?,?,?,?,'issued')`, row.id, t.site, op.DataEpoch, row.subject, row.session, row.request, row.inputHash, row.method, row.difficulty, row.nonceHash, row.expires, row.created)
	if err != nil {
		return IssueChallengeResult{}, loginDBError()
	}
	if err = t.commit(); err != nil {
		return IssueChallengeResult{}, err
	}
	return challengeResult(t.runtime, row, nonce), nil
}

// VerifyChallenge 在短事务外调用供应方；提交issued→consumed后才返回通过证明。
func VerifyChallenge(ctx context.Context, op OperationContext, in VerifyChallengeInput) (VerifyChallengeResult, error) {
	if op.CallerService != "portal" {
		return VerifyChallengeResult{}, platform.NewError(platform.CodeForbidden, false, "挑战仅接受可信入口")
	}
	row, runtime, site, err := readChallengeSnapshot(ctx, op, in.ChallengeID)
	if err != nil {
		return VerifyChallengeResult{}, err
	}
	if row.subject != in.SubjectKey {
		return VerifyChallengeResult{}, platform.NewError(platform.CodeForbidden, false, "挑战不属于当前匿名主体")
	}
	if err = checkIssuedChallenge(row, time.Now().UTC()); err != nil {
		return VerifyChallengeResult{}, err
	}
	nonce, err := challengeNonce(runtime, site, op.DataEpoch, row)
	if err != nil {
		return VerifyChallengeResult{}, err
	}
	valid := false
	if row.method == "pow" {
		proof, err := VerifyPoW(ctx, op, VerifyPoWInput{nonce, in.Proof, row.difficulty})
		if err != nil {
			return VerifyChallengeResult{}, err
		}
		valid = proof.Valid
	} else {
		proof, err := VerifyExternalChallenge(ctx, op, VerifyExternalChallengeInput{ResponseToken: in.Proof, ExpectedAction: "traffic_guard", SubjectKey: in.SubjectKey, ChallengeID: row.id})
		if err != nil {
			return VerifyChallengeResult{}, err
		}
		valid = proof.Valid
	}
	// 通过后的会话ID只存在内存及签名cookie；新匿名浏览器各获独立会话，不共享网络会话。
	sessionID := in.SessionID
	subject, session := row.subject, row.session
	if valid {
		expected := row.subject
		if sessionID == "" {
			sessionID = platform.NewIdentifier()
			expected = ""
		}
		subject, session, err = deriveSessionScopes(runtime, site, sessionID, expected)
		if err != nil {
			return VerifyChallengeResult{}, err
		}
	}
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return VerifyChallengeResult{}, err
	}
	defer t.close()
	current, err := readChallengeRecord(t, row.id)
	if err != nil {
		return VerifyChallengeResult{}, err
	}
	if err = checkIssuedChallenge(current, t.now); err != nil {
		return VerifyChallengeResult{}, err
	}
	if !valid {
		_, err = t.tx.ExecContext(t.ctx, `UPDATE guard_challenges SET attempts=attempts+1 WHERE challenge_id=? AND site_id=? AND data_epoch=? AND state='issued' AND attempts=?`, row.id, t.site, op.DataEpoch, current.attempts)
		if err != nil {
			return VerifyChallengeResult{}, loginDBError()
		}
		if err = t.commit(); err != nil {
			return VerifyChallengeResult{}, err
		}
		return VerifyChallengeResult{}, platform.NewError(platform.CodeValidation, false, "挑战证明不正确")
	}
	// 外部调用期间可能产生ban，成功证明不能绕过新AI禁令。
	signals, err := LoadSignals(t.ctx, op, LoadSignalsInput{SubjectKey: row.subject, Action: "qa.ask", NowMs: t.now.UnixMilli()})
	if err != nil {
		return VerifyChallengeResult{}, err
	}
	if signals.ActiveBan {
		return VerifyChallengeResult{}, platform.NewError(platform.CodeForbidden, false, "该网络AI功能暂被限制")
	}
	expires := t.now.Add(10 * time.Minute)
	_, err = t.tx.ExecContext(t.ctx, `UPDATE guard_challenges SET state='consumed' WHERE challenge_id=? AND site_id=? AND data_epoch=? AND state='issued' AND attempts=?`, row.id, t.site, op.DataEpoch, current.attempts)
	if err != nil {
		return VerifyChallengeResult{}, loginDBError()
	}
	_, err = t.tx.ExecContext(t.ctx, `INSERT INTO guard_subjects(site_id,data_epoch,subject_key,session_key,network_key,challenge_passed_until_ns,model_verdict,questions_since_challenge,challenge_level,suspicious_until_ns,updated_at_ns)
  SELECT site_id,data_epoch,?,?,network_key,?,model_verdict,0,?,suspicious_until_ns,? FROM guard_subjects WHERE site_id=? AND data_epoch=? AND subject_key=?
  ON CONFLICT(site_id,data_epoch,subject_key) DO UPDATE SET challenge_passed_until_ns=excluded.challenge_passed_until_ns,questions_since_challenge=0,challenge_level=excluded.challenge_level,updated_at_ns=excluded.updated_at_ns`, subject, session, expires.UnixNano(), row.difficulty, t.now.UnixNano(), t.site, op.DataEpoch, row.subject)
	if err != nil {
		return VerifyChallengeResult{}, loginDBError()
	}
	if subject != row.subject {
		_, err = t.tx.ExecContext(t.ctx, `INSERT INTO guard_network_aliases(site_id,data_epoch,subject_key,network_key,last_seen_at_ns)
   SELECT site_id,data_epoch,?,network_key,? FROM guard_network_aliases WHERE site_id=? AND data_epoch=? AND subject_key=?`, subject, t.now.UnixNano(), t.site, op.DataEpoch, row.subject)
		if err != nil {
			return VerifyChallengeResult{}, loginDBError()
		}
	}
	expiry := expires.Format(time.RFC3339Nano)
	token, err := serviceauth.SignGuardSession(t.runtime.SigningKey, platform.GuardSessionClaims{SubjectID: subject, SiteID: t.site, DataEpoch: op.DataEpoch, ExpiresAt: expiry, SessionID: sessionID})
	if err != nil {
		return VerifyChallengeResult{}, err
	}
	if err = t.commit(); err != nil {
		return VerifyChallengeResult{}, err
	}
	return VerifyChallengeResult{PassedToken: token, ExpiresAt: expiry}, nil
}

func readChallengeRecord(t *guardTransaction, id string) (challengeRecord, error) {
	var row challengeRecord
	err := t.tx.QueryRowContext(t.ctx, `SELECT challenge_id,subject_key,session_key,request_id,input_hash,method,difficulty,nonce_hash,expires_at_ns,created_at_ns,state,attempts FROM guard_challenges WHERE challenge_id=? AND site_id=? AND data_epoch=?`, id, t.site, t.op.DataEpoch).Scan(&row.id, &row.subject, &row.session, &row.request, &row.inputHash, &row.method, &row.difficulty, &row.nonceHash, &row.expires, &row.created, &row.state, &row.attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return challengeRecord{}, platform.NewError(platform.CodeNotFound, false, "挑战不存在")
	}
	if err != nil {
		return challengeRecord{}, loginDBError()
	}
	return row, nil
}
func readChallengeSnapshot(ctx context.Context, op OperationContext, id string) (challengeRecord, GuardRuntime, string, error) {
	t, err := beginGuardTransaction(ctx, op, false)
	if err != nil {
		return challengeRecord{}, GuardRuntime{}, "", err
	}
	defer t.close()
	row, err := readChallengeRecord(t, id)
	if err != nil {
		return challengeRecord{}, GuardRuntime{}, "", err
	}
	return row, t.runtime, t.site, nil
}
func checkIssuedChallenge(row challengeRecord, now time.Time) error {
	if row.state == "consumed" {
		return platform.NewError(platform.CodeConflict, false, "挑战已经消费")
	}
	if row.state != "issued" || row.expires <= now.UnixNano() {
		return platform.NewError(platform.CodeStaleResult, false, "挑战已过期")
	}
	if row.attempts >= 5 {
		return platform.NewError(platform.CodeConflict, false, "挑战失败次数已满，请取得新挑战")
	}
	return nil
}
func challengeNonce(runtime GuardRuntime, site, epoch string, row challengeRecord) (string, error) {
	payload, _ := platform.CanonicalJSON(map[string]string{"challengeId": row.id, "siteId": site, "dataEpoch": epoch, "subjectKey": row.subject, "requestId": row.request, "method": row.method})
	keys := append([]platform.SigningKey{runtime.BudgetKey}, runtime.PreviousBudgetKeys...)
	for _, key := range keys {
		nonce, err := serviceauth.PrivateOperationDigest(key, "guard.challenge.v1", payload)
		if err != nil {
			return "", err
		}
		if row.nonceHash == "" || platform.HashBytes([]byte(nonce)) == row.nonceHash {
			return nonce, nil
		}
	}
	return "", platform.NewError(platform.CodeVersionMismatch, false, "挑战预算key过渡材料缺失")
}
func challengeResult(runtime GuardRuntime, row challengeRecord, nonce string) IssueChallengeResult {
	siteKey := ""
	if row.method == "turnstile" {
		siteKey = runtime.Turnstile.SiteKey
	}
	return IssueChallengeResult{ChallengeID: row.id, Nonce: nonce, Difficulty: row.difficulty, ExpiresAt: time.Unix(0, row.expires).UTC().Format(time.RFC3339Nano), Method: row.method, SiteKey: siteKey, Interactive: row.difficulty == 22}
}
