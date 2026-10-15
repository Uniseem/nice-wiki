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
)

type Identifier = platform.Identifier

// 登录准入只接受已经过统一入口校验的HMAC，不持有其原始身份或网络信息。
type BeginLoginAttemptInput struct {
	RequestID   string   `json:"requestId"`
	AccountKey  string   `json:"accountKey"`
	NetworkKeys []string `json:"networkKeys"`
	Action      string   `json:"action"`
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
	if err := checkLoginScope(ctx, t, "account", in.AccountKey, 5, 1); err != nil {
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
	var lockedUntil int64
	err := t.tx.QueryRowContext(ctx, `SELECT locked_until_ns FROM login_locks WHERE data_epoch=? AND scope_kind=? AND scope_key=?`, t.store.openedDataEpoch, kind, key).Scan(&lockedUntil)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return loginDBError()
	}
	var failures int
	if err := t.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM login_failures
        WHERE data_epoch=? AND scope_kind=? AND scope_key=? AND failed_at_ns>?`, t.store.openedDataEpoch, kind, key, t.now.Add(-loginFailureWindow).UnixNano()).Scan(&failures); err != nil {
		return loginDBError()
	}
	if lockedUntil > t.now.UnixNano() || failures >= failureLimit {
		return platform.NewError(platform.CodeRateLimited, false, "登录失败保护暂时锁定")
	}
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
