package trafficguard

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
)

type Identifier = platform.Identifier

// 登录准入只接受已经过统一入口校验的HMAC，不持有其原始身份或网络信息。
type BeginLoginAttemptInput struct {
	RequestID      string   `json:"requestId"`
	AccountKey     string   `json:"accountKey"`
	NetworkKeys    []string `json:"networkKeys"`
	Action         string   `json:"action"`
	ChallengeID    string   `json:"challengeId,omitempty"`
	ChallengeProof string   `json:"challengeProof,omitempty"`
}
type BeginLoginAttemptResult struct {
	AttemptID string             `json:"attemptId"`
	ExpiresAt platform.Timestamp `json:"expiresAt"`
	Revision  platform.Revision  `json:"revision"`
}
type FinishLoginAttemptInput struct {
	Outcome string `json:"outcome"`
}
type FinishLoginAttemptResult struct {
	Accepted  bool   `json:"accepted"`
	AttemptID string `json:"attemptId"`
}

type loginTransaction struct {
	store *PolicyStore
	tx    *sql.Tx
	now   time.Time
}
type loginAttemptRecord struct {
	accountKey  string
	networkKeys []string
	outcome     string
	expiresAt   int64
}

const loginFailureWindow = 15 * time.Minute
const loginLockDuration = 15 * time.Minute
const loginReservationDuration = 30 * time.Second

// BeginLoginAttempt 原子取得账号和所有网络作用域的30秒核验预留。
func BeginLoginAttempt(ctx context.Context, op OperationContext, in BeginLoginAttemptInput) (BeginLoginAttemptResult, error) {
	t, err := beginLoginTransaction(ctx, op)
	if err != nil {
		return BeginLoginAttemptResult{}, err
	}
	defer t.close()
	if in.RequestID != op.OperationID {
		return BeginLoginAttemptResult{}, platform.NewError(platform.CodeConflict, false, "登录请求与操作引用不一致")
	}
	inputHash, networks := loginInputHash(in)
	var originalHash string
	var result BeginLoginAttemptResult
	var expires int64
	err = t.tx.QueryRowContext(ctx, `SELECT input_hash, attempt_id, expires_at_ns, creation_revision
        FROM login_attempts WHERE data_epoch=? AND caller=? AND request_id=?`,
		op.DataEpoch, op.CallerService, in.RequestID).Scan(&originalHash, &result.AttemptID, &expires, &result.Revision)
	if err == nil {
		if originalHash != inputHash {
			return BeginLoginAttemptResult{}, platform.NewError(platform.CodeConflict, false, "登录请求已绑定其他输入")
		}
		result.ExpiresAt = time.Unix(0, expires).UTC().Format(time.RFC3339Nano)
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return BeginLoginAttemptResult{}, loginDBError()
	}
	// 账号维度连错后不硬锁：否则任何知道账号名的人每 15 分钟试几次，就能把唯一的超管一直锁在门外。
	// 改为此后每次登录尝试都要附一份新解出的工作量证明，一次挑战只换一次尝试；网络维度仍硬锁。
	limited, err := loginScopeLimited(ctx, t, "account", in.AccountKey, 5)
	if err != nil {
		return BeginLoginAttemptResult{}, err
	}
	if limited {
		if in.ChallengeID == "" {
			return BeginLoginAttemptResult{}, platform.NewError(platform.CodeChallengeRequired, false, "账号近期多次登录失败，请先完成人机验证")
		}
		if err := consumeLoginChallenge(ctx, t, op, in.ChallengeID, in.ChallengeProof); err != nil {
			return BeginLoginAttemptResult{}, err
		}
	}
	if err := checkLoginConcurrency(ctx, t, "account", in.AccountKey, 1); err != nil {
		return BeginLoginAttemptResult{}, err
	}
	for _, key := range networks {
		if err := checkLoginScope(ctx, t, "network", key, 20, 8); err != nil {
			return BeginLoginAttemptResult{}, err
		}
	}
	expiry := t.now.Add(loginReservationDuration)
	result = BeginLoginAttemptResult{AttemptID: platform.NewIdentifier(), ExpiresAt: expiry.Format(time.RFC3339Nano), Revision: 1}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO login_attempts
        (attempt_id,data_epoch,caller,request_id,input_hash,account_key,action,created_at_ns,expires_at_ns,creation_revision,outcome)
        VALUES(?,?,?,?,?,?,?,?,?,1,'pending')`, result.AttemptID, op.DataEpoch, op.CallerService,
		in.RequestID, inputHash, in.AccountKey, in.Action, t.now.UnixNano(), expiry.UnixNano())
	if err != nil {
		return BeginLoginAttemptResult{}, loginDBError()
	}
	for _, key := range networks {
		if _, err := t.tx.ExecContext(ctx, `INSERT INTO login_attempt_networks(data_epoch,attempt_id,network_key) VALUES(?,?,?)`, op.DataEpoch, result.AttemptID, key); err != nil {
			return BeginLoginAttemptResult{}, loginDBError()
		}
	}
	if err := t.tx.Commit(); err != nil {
		return BeginLoginAttemptResult{}, loginDBError()
	}
	return result, nil
}

// FinishLoginAttempt 保存核验事实；原终局不能被新operation或重放改写。
func FinishLoginAttempt(ctx context.Context, op OperationContext, attemptId Identifier, in FinishLoginAttemptInput) (FinishLoginAttemptResult, error) {
	t, err := beginLoginTransaction(ctx, op)
	if err != nil {
		return FinishLoginAttemptResult{}, err
	}
	defer t.close()
	var receiptAttempt, receiptOutcome string
	err = t.tx.QueryRowContext(ctx, `SELECT attempt_id,outcome FROM login_finish_receipts
        WHERE data_epoch=? AND caller=? AND operation_id=?`, op.DataEpoch, op.CallerService, op.OperationID).Scan(&receiptAttempt, &receiptOutcome)
	if err == nil {
		if receiptAttempt != attemptId || receiptOutcome != in.Outcome {
			return FinishLoginAttemptResult{}, platform.NewError(platform.CodeConflict, false, "登录终局操作已绑定其他输入")
		}
		return FinishLoginAttemptResult{Accepted: true, AttemptID: attemptId}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return FinishLoginAttemptResult{}, loginDBError()
	}
	attempt, err := loadLoginAttempt(ctx, t, attemptId)
	if err != nil {
		return FinishLoginAttemptResult{}, err
	}
	if attempt.outcome != "pending" && attempt.outcome != in.Outcome {
		return FinishLoginAttemptResult{}, platform.NewError(platform.CodeConflict, false, "登录核验终局不可更改")
	}
	if attempt.outcome == "pending" {
		if in.Outcome == "authenticated" && !t.now.Before(time.Unix(0, attempt.expiresAt)) {
			return FinishLoginAttemptResult{}, platform.NewError(platform.CodeConflict, false, "登录核验预留已到期")
		}
		switch in.Outcome {
		case "authenticated":
			if _, err := t.tx.ExecContext(ctx, `DELETE FROM login_failures WHERE data_epoch=? AND scope_kind='account' AND scope_key=?`, op.DataEpoch, attempt.accountKey); err != nil {
				return FinishLoginAttemptResult{}, loginDBError()
			}
			if _, err := t.tx.ExecContext(ctx, `DELETE FROM login_locks WHERE data_epoch=? AND scope_kind='account' AND scope_key=?`, op.DataEpoch, attempt.accountKey); err != nil {
				return FinishLoginAttemptResult{}, loginDBError()
			}
		case "rejected":
			if err := recordLoginFailure(ctx, t, attemptId, "account", attempt.accountKey, 5); err != nil {
				return FinishLoginAttemptResult{}, err
			}
			for _, key := range attempt.networkKeys {
				if err := recordLoginFailure(ctx, t, attemptId, "network", key, 20); err != nil {
					return FinishLoginAttemptResult{}, err
				}
			}
		case "aborted":
			// 中止只释放预留，不改变已有失败或锁。
		}
		if _, err := t.tx.ExecContext(ctx, `UPDATE login_attempts SET outcome=?,finished_at_ns=?
            WHERE data_epoch=? AND attempt_id=? AND outcome='pending'`, in.Outcome, t.now.UnixNano(), op.DataEpoch, attemptId); err != nil {
			return FinishLoginAttemptResult{}, loginDBError()
		}
	}
	if _, err := t.tx.ExecContext(ctx, `INSERT INTO login_finish_receipts(data_epoch,caller,operation_id,attempt_id,outcome,created_at_ns)
        VALUES(?,?,?,?,?,?)`, op.DataEpoch, op.CallerService, op.OperationID, attemptId, in.Outcome, t.now.UnixNano()); err != nil {
		return FinishLoginAttemptResult{}, loginDBError()
	}
	if err := t.tx.Commit(); err != nil {
		return FinishLoginAttemptResult{}, loginDBError()
	}
	return FinishLoginAttemptResult{Accepted: true, AttemptID: attemptId}, nil
}

func beginLoginTransaction(ctx context.Context, op OperationContext) (*loginTransaction, error) {
	if op.CallerService != "identity" || op.Actor.Kind != "service" {
		return nil, platform.NewError(platform.CodeForbidden, false, "登录保护仅接受identity纯服务调用")
	}
	store, ok := ctx.Value(policyStoreContextKey{}).(*PolicyStore)
	if !ok || store == nil {
		return nil, loginDBError()
	}
	store.mu.RLock()
	if store.db == nil {
		store.mu.RUnlock()
		return nil, loginDBError()
	}
	if op.DataEpoch != store.openedDataEpoch {
		store.mu.RUnlock()
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "登录保护数据代次已变化")
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		store.mu.RUnlock()
		return nil, loginDBError()
	}
	return &loginTransaction{store: store, tx: tx, now: time.Now().UTC()}, nil
}

func (t *loginTransaction) close() {
	_ = t.tx.Rollback()
	t.store.mu.RUnlock()
}

func loginInputHash(in BeginLoginAttemptInput) (string, []string) {
	keys := append([]string(nil), in.NetworkKeys...)
	sort.Strings(keys)
	unique := keys[:0]
	for _, key := range keys {
		if len(unique) == 0 || unique[len(unique)-1] != key {
			unique = append(unique, key)
		}
	}
	in.NetworkKeys = unique
	canonical, _ := json.Marshal(in)
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), unique
}

func checkLoginScope(ctx context.Context, t *loginTransaction, kind string, key string, failureLimit int, concurrencyLimit int) error {
	limited, err := loginScopeLimited(ctx, t, kind, key, failureLimit)
	if err != nil {
		return err
	}
	if limited {
		return platform.NewError(platform.CodeRateLimited, false, "登录失败保护暂时锁定")
	}
	return checkLoginConcurrency(ctx, t, kind, key, concurrencyLimit)
}

// loginScopeLimited 报告该作用域是否在锁定期内或近期失败已达上限。
func loginScopeLimited(ctx context.Context, t *loginTransaction, kind string, key string, failureLimit int) (bool, error) {
	var lockedUntil int64
	err := t.tx.QueryRowContext(ctx, `SELECT locked_until_ns FROM login_locks WHERE data_epoch=? AND scope_kind=? AND scope_key=?`, t.store.openedDataEpoch, kind, key).Scan(&lockedUntil)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, loginDBError()
	}
	var failures int
	if err := t.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM login_failures
        WHERE data_epoch=? AND scope_kind=? AND scope_key=? AND failed_at_ns>?`, t.store.openedDataEpoch, kind, key, t.now.Add(-loginFailureWindow).UnixNano()).Scan(&failures); err != nil {
		return false, loginDBError()
	}
	return lockedUntil > t.now.UnixNano() || failures >= failureLimit, nil
}

// consumeLoginChallenge 核验并在本登录事务里消费一份 login-pow 挑战；登录尝试没开成，消费也随事务回滚。
func consumeLoginChallenge(ctx context.Context, t *loginTransaction, op OperationContext, challengeID, proof string) error {
	runtime, ok := ctx.Value(guardRuntimeKey{}).(GuardRuntime)
	if !ok {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "风控运行依赖未装配")
	}
	var row challengeRecord
	var site string
	err := t.tx.QueryRowContext(ctx, `SELECT challenge_id,site_id,subject_key,session_key,request_id,input_hash,method,difficulty,nonce_hash,expires_at_ns,created_at_ns,state,attempts FROM guard_challenges WHERE challenge_id=? AND data_epoch=?`, challengeID, t.store.openedDataEpoch).Scan(&row.id, &site, &row.subject, &row.session, &row.request, &row.inputHash, &row.method, &row.difficulty, &row.nonceHash, &row.expires, &row.created, &row.state, &row.attempts)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && row.method != loginChallengeMethod) {
		return platform.NewError(platform.CodeChallengeRequired, false, "人机验证无效，请重新验证")
	}
	if err != nil {
		return loginDBError()
	}
	if checkIssuedChallenge(row, t.now) != nil {
		return platform.NewError(platform.CodeChallengeRequired, false, "人机验证已过期或已用过，请重新验证")
	}
	nonce, err := challengeNonce(runtime, site, t.store.openedDataEpoch, row)
	if err != nil {
		return err
	}
	checked, err := VerifyPoW(ctx, op, VerifyPoWInput{nonce, proof, row.difficulty})
	if err != nil {
		return err
	}
	if !checked.Valid {
		return platform.NewError(platform.CodeChallengeRequired, false, "人机验证未通过，请重新验证")
	}
	consumed, err := t.tx.ExecContext(ctx, `UPDATE guard_challenges SET state='consumed' WHERE challenge_id=? AND data_epoch=? AND state='issued'`, challengeID, t.store.openedDataEpoch)
	if err != nil {
		return loginDBError()
	}
	if count, err := consumed.RowsAffected(); err != nil || count != 1 {
		return platform.NewError(platform.CodeChallengeRequired, false, "人机验证已用过，请重新验证")
	}
	return nil
}

// checkLoginConcurrency 限制同一作用域同时进行中的登录核验数。
func checkLoginConcurrency(ctx context.Context, t *loginTransaction, kind string, key string, concurrencyLimit int) error {
	var pending int
	query := `SELECT COUNT(*) FROM login_attempts WHERE data_epoch=? AND account_key=? AND outcome='pending' AND expires_at_ns>?`
	if kind == "network" {
		query = `SELECT COUNT(*) FROM login_attempt_networks n JOIN login_attempts a
            ON a.data_epoch=n.data_epoch AND a.attempt_id=n.attempt_id
            WHERE n.data_epoch=? AND n.network_key=? AND a.outcome='pending' AND a.expires_at_ns>?`
	}
	if err := t.tx.QueryRowContext(ctx, query, t.store.openedDataEpoch, key, t.now.UnixNano()).Scan(&pending); err != nil {
		return loginDBError()
	}
	if pending >= concurrencyLimit {
		return platform.NewError(platform.CodeRateLimited, false, "登录核验并发已满")
	}
	return nil
}

func loadLoginAttempt(ctx context.Context, t *loginTransaction, attemptID Identifier) (loginAttemptRecord, error) {
	var record loginAttemptRecord
	var epoch, caller string
	err := t.tx.QueryRowContext(ctx, `SELECT data_epoch,caller,account_key,outcome,expires_at_ns FROM login_attempts WHERE attempt_id=?`, attemptID).Scan(&epoch, &caller, &record.accountKey, &record.outcome, &record.expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return loginAttemptRecord{}, platform.NewError(platform.CodeNotFound, false, "登录核验尝试不存在")
	}
	if err != nil {
		return loginAttemptRecord{}, loginDBError()
	}
	if epoch != t.store.openedDataEpoch {
		return loginAttemptRecord{}, platform.NewError(platform.CodeVersionMismatch, false, "登录核验尝试属于其他数据代次")
	}
	if caller != "identity" {
		return loginAttemptRecord{}, platform.NewError(platform.CodeForbidden, false, "登录核验尝试来源不匹配")
	}
	rows, err := t.tx.QueryContext(ctx, `SELECT network_key FROM login_attempt_networks WHERE data_epoch=? AND attempt_id=? ORDER BY network_key`, epoch, attemptID)
	if err != nil {
		return loginAttemptRecord{}, loginDBError()
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return loginAttemptRecord{}, loginDBError()
		}
		record.networkKeys = append(record.networkKeys, key)
	}
	if rows.Err() != nil || len(record.networkKeys) == 0 {
		return loginAttemptRecord{}, loginDBError()
	}
	return record, nil
}

func recordLoginFailure(ctx context.Context, t *loginTransaction, attemptID Identifier, kind string, key string, threshold int) error {
	epoch := t.store.openedDataEpoch
	if _, err := t.tx.ExecContext(ctx, `INSERT INTO login_failures(data_epoch,scope_kind,scope_key,attempt_id,failed_at_ns) VALUES(?,?,?,?,?)`, epoch, kind, key, attemptID, t.now.UnixNano()); err != nil {
		return loginDBError()
	}
	var failures int
	if err := t.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM login_failures WHERE data_epoch=? AND scope_kind=? AND scope_key=? AND failed_at_ns>?`, epoch, kind, key, t.now.Add(-loginFailureWindow).UnixNano()).Scan(&failures); err != nil {
		return loginDBError()
	}
	if failures >= threshold {
		_, err := t.tx.ExecContext(ctx, `INSERT INTO login_locks(data_epoch,scope_kind,scope_key,locked_until_ns) VALUES(?,?,?,?)
            ON CONFLICT(data_epoch,scope_kind,scope_key) DO UPDATE SET locked_until_ns=excluded.locked_until_ns
            WHERE login_locks.locked_until_ns<=?`, epoch, kind, key, t.now.Add(loginLockDuration).UnixNano(), t.now.UnixNano())
		if err != nil {
			return loginDBError()
		}
	}
	return nil
}

func loginDBError() error {
	return platform.NewError(platform.CodeDependencyUnavailable, true, "登录保护私有存储不可用")
}

type EvaluateAdmissionInput struct {
	RequestID  string `json:"requestId"`
	SubjectKey string `json:"subjectKey"`
	Action     string `json:"action"`
	DataEpoch  string `json:"-"`
}
type EvaluateAdmissionResult struct {
	Decision     string                    `json:"decision"`
	AdmissionID  *string                   `json:"admissionId"`
	RetryAfterMs int64                     `json:"retryAfterMs"`
	ChallengeID  *string                   `json:"challengeId"`
	ReasonCode   string                    `json:"reasonCode"`
	Permit       *platform.AdmissionPermit `json:"permit"`
}

// EvaluateAdmission 只分配持久元数据；只有同根消费后的QA行才可签模型token。
func EvaluateAdmission(ctx context.Context, op OperationContext, in EvaluateAdmissionInput) (EvaluateAdmissionResult, error) {
	if in.RequestID != op.RootOperationID {
		return EvaluateAdmissionResult{}, platform.NewError(platform.CodeValidation, false, "准入必须绑定原逻辑请求根")
	}
	classification, err := ClassifyAction(ctx, op, ClassifyActionInput{Action: in.Action})
	if err != nil {
		return EvaluateAdmissionResult{}, err
	}
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return EvaluateAdmissionResult{}, err
	}
	defer t.close()
	t.op.OperationID = op.RootOperationID
	var binding struct{}
	found, err := readGuardReceipt(t, "admission.input", in, &binding)
	if err != nil {
		return EvaluateAdmissionResult{}, err
	}
	if !found {
		if err = saveGuardReceipt(t, "admission.input", in, binding); err != nil {
			return EvaluateAdmissionResult{}, err
		}
	}
	policy, policyRevision, err := readGuardPolicy(t)
	if err != nil {
		return EvaluateAdmissionResult{}, err
	}
	var network string
	err = t.tx.QueryRowContext(t.ctx, `SELECT network_key FROM guard_subjects WHERE site_id=? AND data_epoch=? AND subject_key=?`, t.site, op.DataEpoch, in.SubjectKey).Scan(&network)
	if errors.Is(err, sql.ErrNoRows) {
		return EvaluateAdmissionResult{}, platform.NewError(platform.CodeNotFound, false, "匿名主体尚未验证")
	}
	if err != nil {
		return EvaluateAdmissionResult{}, loginDBError()
	}
	if classification.RequiresAIProof {
		signals, err := LoadSignals(t.ctx, op, LoadSignalsInput{in.SubjectKey, in.Action, t.now.UnixMilli()})
		if err != nil {
			return EvaluateAdmissionResult{}, err
		}
		risk, err := ComputeRisk(t.ctx, op, ComputeRiskInput{Denials10m: signals.Denials10m, ChallengePassed: signals.ChallengePassed || !policy.ChallengeEnabled, ActiveBan: signals.ActiveBan, ExactRepeatCount: signals.ExactRepeatCount, SimilarityScore: signals.SimilarityScore, RegularIntervalCount: signals.RegularIntervalCount, ModelVerdict: signals.ModelVerdict})
		if err != nil {
			return EvaluateAdmissionResult{}, err
		}
		if risk.Decision != "allow" {
			reason := platform.CodeForbidden
			if risk.Decision == "challenge" {
				reason = platform.CodeChallengeRequired
			}
			if err = recordAdmissionDenial(t, in.RequestID, in.Action, reason, policyRevision, network); err != nil {
				return EvaluateAdmissionResult{}, err
			}
			if err = t.commit(); err != nil {
				return EvaluateAdmissionResult{}, err
			}
			return EvaluateAdmissionResult{Decision: risk.Decision, ReasonCode: reason}, nil
		}
	}
	// 成功分配只保留一份；同根重放只交回原准入，已过期/终止的也不重新分配。
	// 原准入已终止时照样交回：拥有服务按原回执返回结果，没有回执时它核验准入会拒绝，不会借此复活。
	var existingID string
	err = t.tx.QueryRowContext(t.ctx, `SELECT admission_id FROM guard_permits WHERE site_id=? AND data_epoch=? AND owner_caller=? AND request_id=?`, t.site, op.DataEpoch, classification.OwnerCaller, in.RequestID).Scan(&existingID)
	if err == nil {
		row, err := readPermitRecord(t, existingID)
		if err != nil {
			return EvaluateAdmissionResult{}, err
		}
		if row.subject != in.SubjectKey || row.action != in.Action {
			return EvaluateAdmissionResult{}, platform.NewError(platform.CodeConflict, false, "原根已绑定其他匿名主体或动作")
		}
		permit := permitMetadata(row, op.DataEpoch)
		if err = t.commit(); err != nil {
			return EvaluateAdmissionResult{}, err
		}
		return EvaluateAdmissionResult{Decision: "allow", AdmissionID: &row.admissionID, Permit: &permit}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return EvaluateAdmissionResult{}, loginDBError()
	}
	if _, err = t.tx.ExecContext(t.ctx, `SAVEPOINT guard_quota`); err != nil {
		return EvaluateAdmissionResult{}, loginDBError()
	}
	buckets, err := ConsumeBuckets(t.ctx, op, ConsumeBucketsInput{SubjectKey: in.SubjectKey, Action: in.Action, Cost: 1, NowMs: t.now.UnixMilli(), DataEpoch: op.DataEpoch})
	if err != nil {
		return EvaluateAdmissionResult{}, err
	}
	var allocation AcquireConcurrencyResult
	if buckets.Allowed {
		allocation, err = AcquireConcurrency(t.ctx, op, AcquireConcurrencyInput{in.SubjectKey, in.Action, in.RequestID, op.DataEpoch})
	}
	var limited *platform.Error
	concurrencyLimited := errors.As(err, &limited) && limited.Code == platform.CodeRateLimited
	if err != nil && !concurrencyLimited {
		return EvaluateAdmissionResult{}, err
	}
	if !buckets.Allowed || concurrencyLimited {
		if _, err = t.tx.ExecContext(t.ctx, `ROLLBACK TO guard_quota`); err != nil {
			return EvaluateAdmissionResult{}, loginDBError()
		}
		if _, err = t.tx.ExecContext(t.ctx, `RELEASE guard_quota`); err != nil {
			return EvaluateAdmissionResult{}, loginDBError()
		}
		if err = recordAdmissionDenial(t, in.RequestID, in.Action, platform.CodeRateLimited, policyRevision, network); err != nil {
			return EvaluateAdmissionResult{}, err
		}
		if err = t.commit(); err != nil {
			return EvaluateAdmissionResult{}, err
		}
		decision := "deny"
		if concurrencyLimited {
			decision = "queue"
		}
		return EvaluateAdmissionResult{Decision: decision, ReasonCode: platform.CodeRateLimited, RetryAfterMs: buckets.RetryAfterMs}, nil
	}
	if _, err = t.tx.ExecContext(t.ctx, `RELEASE guard_quota`); err != nil {
		return EvaluateAdmissionResult{}, loginDBError()
	}
	row, err := readPermitRecord(t, allocation.AdmissionID)
	if err != nil {
		return EvaluateAdmissionResult{}, err
	}
	permit := permitMetadata(row, op.DataEpoch)
	if err = t.commit(); err != nil {
		return EvaluateAdmissionResult{}, err
	}
	return EvaluateAdmissionResult{Decision: "allow", AdmissionID: &row.admissionID, Permit: &permit}, nil
}

// recordAdmissionDenial 对原根/原因/策略去重，只广播安全决定引用，不广播匿名网络key。
func recordAdmissionDenial(t *guardTransaction, request, action, reason string, revision int64, network string) error {
	identity, _ := platform.CanonicalJSON(map[string]any{"site": t.site, "epoch": t.op.DataEpoch, "root": request, "reason": reason, "policy": revision})
	eventID := platform.HashBytes(identity)
	payload, _ := json.Marshal(map[string]any{"admissionId": eventID, "action": action, "reasonCode": reason, "policyRevision": revision})
	decision := "deny"
	if reason == platform.CodeChallengeRequired {
		decision = "challenge"
	}
	_, err := t.tx.ExecContext(t.ctx, `INSERT INTO guard_risk_events(event_id,site_id,data_epoch,action,decision,network_key,occurred_at_ns)
  VALUES(?,?,?,?,?,?,?) ON CONFLICT(event_id) DO NOTHING`, eventID, t.site, t.op.DataEpoch, action, decision, network, t.now.UnixNano())
	if err != nil {
		return loginDBError()
	}
	return outbox.EnqueueEvent(t.tx, platform.EventEnvelope{EventID: eventID, EventType: "traffic-guard.admission-denied.v1", EventVersion: 1, Producer: "traffic-guard", SubjectID: eventID, Revision: revision, OccurredAt: t.now.Format(time.RFC3339Nano), DataEpoch: t.op.DataEpoch, Payload: payload})
}
