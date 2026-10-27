package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"nice-wiki/libs/platform"
)

type RevokeAllSessionsInput struct {
	AccountID platform.Identifier `json:"accountId"`
	Reason    string              `json:"reason"`
}

type RevokeAllSessionsResult struct {
	Count int64 `json:"count"`
}

// RevokeAllSessions 在账户状态迁移的私有事务内或经超管授权后撤销全部登录。
// 同一operation重放读取原结果，不能把首次提交后建立的新会话一起撤销。
func RevokeAllSessions(ctx context.Context, op OperationContext, input RevokeAllSessionsInput) (RevokeAllSessionsResult, error) {
	externalTx, external := identityTransactionFromContext(ctx)
	if !external && (op.Actor.Kind != "admin" || !hasGrant(op.Actor.Grants, "superadmin")) {
		return RevokeAllSessionsResult{}, platform.NewError(platform.CodeForbidden, false, "撤销全部会话需要超管或身份私有事务")
	}
	tx := externalTx
	if !external {
		store, ok := identityStoreFromContext(ctx)
		if !ok {
			return RevokeAllSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话私有存储尚未装配")
		}
		var err error
		tx, err = store.db.BeginTx(ctx, nil)
		if err != nil {
			return RevokeAllSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销事务不可用")
		}
		defer tx.Rollback()
	}
	requestHash := platform.HashBytes(mustJSON(struct {
		Action    string                 `json:"action"`
		Caller    string                 `json:"caller"`
		DataEpoch platform.Identifier    `json:"dataEpoch"`
		Input     RevokeAllSessionsInput `json:"input"`
	}{"identity.RevokeAllSessions", op.CallerService, op.DataEpoch, input}))
	var err error
	if !external {
		var digest, resultHash, state string
		var resultJSON sql.NullString
		err = tx.QueryRowContext(ctx, `SELECT input_digest, result_hash, state, result_json
        FROM operation_receipts WHERE site_id = ? AND actor_id = ? AND operation_id = ?`,
			op.Actor.SiteID, op.Actor.SubjectID, op.OperationID).Scan(&digest, &resultHash, &state, &resultJSON)
		if err == nil {
			if digest != requestHash {
				return RevokeAllSessionsResult{}, platform.NewError(platform.CodeConflict, false, "幂等操作摘要不一致")
			}
			var result RevokeAllSessionsResult
			if state != "committed" || !resultJSON.Valid || platform.HashBytes([]byte(resultJSON.String)) != resultHash ||
				json.Unmarshal([]byte(resultJSON.String), &result) != nil {
				return RevokeAllSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "会话撤销回执不完整")
			}
			return result, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return RevokeAllSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销回执不可读")
		}
	}
	var permissionRevision, credentialRevision platform.Revision
	err = tx.QueryRowContext(ctx, `SELECT a.permission_revision, c.credential_revision
        FROM accounts a JOIN credentials c ON c.account_id = a.account_id
        WHERE a.account_id = ? AND a.site_id = ?`, input.AccountID, op.Actor.SiteID).
		Scan(&permissionRevision, &credentialRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return RevokeAllSessionsResult{}, platform.NewError(platform.CodeForbidden, false, "账户不属于当前站点")
	}
	if err != nil {
		return RevokeAllSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户权威记录不可读")
	}
	now := platform.Now()
	if _, err := tx.ExecContext(ctx, `UPDATE session_families SET revoked_at = ?, revision = revision + 1
        WHERE revoked_at IS NULL AND session_id IN (SELECT session_id FROM sessions WHERE account_id = ? AND site_id = ?)`,
		now, input.AccountID, op.Actor.SiteID); err != nil {
		return RevokeAllSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户会话族撤销失败")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = ?, revision = revision + 1
        WHERE account_id = ? AND site_id = ? AND revoked_at IS NULL`, now, input.AccountID, op.Actor.SiteID)
	if err != nil {
		return RevokeAllSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销写入失败")
	}
	count, err := changed.RowsAffected()
	if err != nil {
		return RevokeAllSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "会话撤销计数不可读")
	}
	reason := input.Reason
	switch reason {
	case "disabled", "deleted":
		reason = "status"
	case "admin_reset":
		reason = "password"
	}
	_, err = PublishIdentityEvent(WithIdentityTransaction(ctx, tx), op, PublishIdentityEventInput{
		EventID: platform.NewIdentifier(),
		Event: IdentityChanged{AccountID: input.AccountID, SiteID: op.Actor.SiteID,
			PermissionRevision: permissionRevision, CredentialRevision: credentialRevision,
			Reason: reason, OccurredAt: now},
	})
	if err != nil {
		return RevokeAllSessionsResult{}, err
	}
	result := RevokeAllSessionsResult{Count: count}
	// 嵌入调用时由外层用例统一登记幂等回执并提交。
	if external {
		return result, nil
	}
	raw, _ := json.Marshal(result)
	_, err = tx.ExecContext(ctx, `INSERT INTO operation_receipts
        (operation_id, site_id, actor_id, input_digest, result_hash, state, created_at, result_json)
        VALUES (?, ?, ?, ?, ?, 'committed', ?, ?)`, op.OperationID, op.Actor.SiteID, op.Actor.SubjectID,
		requestHash, platform.HashBytes(raw), now, string(raw))
	if err != nil {
		return RevokeAllSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销回执写入失败")
	}
	if err := tx.Commit(); err != nil {
		return RevokeAllSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销事务提交失败")
	}
	return result, nil
}

// RevokeSessionFamily 只在本域事务中撤销一个设备族；事务由外层用例提交。
func RevokeSessionFamily(ctx context.Context, op OperationContext, input RevokeSessionFamilyInput) (RevokeSessionFamilyResult, error) {
	tx, ok := identityTransactionFromContext(ctx)
	if !ok {
		return RevokeSessionFamilyResult{}, platform.NewError(platform.CodeForbidden, false, "会话族撤销仅允许身份私有事务")
	}
	var sessionID, accountID, siteID string
	var permissionRevision, credentialRevision platform.Revision
	var revoked sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT f.session_id, s.account_id, s.site_id,
        a.permission_revision, c.credential_revision, f.revoked_at
        FROM session_families f JOIN sessions s ON s.session_id = f.session_id
        JOIN accounts a ON a.account_id = s.account_id
        JOIN credentials c ON c.account_id = a.account_id
        WHERE f.family_id = ?`, input.FamilyID).Scan(&sessionID, &accountID, &siteID,
		&permissionRevision, &credentialRevision, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return RevokeSessionFamilyResult{}, platform.NewError(platform.CodeUnauthenticated, false, "会话族不存在")
	}
	if err != nil {
		return RevokeSessionFamilyResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话族权威不可读")
	}
	if revoked.Valid {
		return RevokeSessionFamilyResult{Count: 0}, nil
	}
	now := platform.Now()
	if _, err := tx.ExecContext(ctx, `UPDATE session_families SET revoked_at = ?, revision = revision + 1
        WHERE family_id = ? AND revoked_at IS NULL`, now, input.FamilyID); err != nil {
		return RevokeSessionFamilyResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话族撤销失败")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = ?, revision = revision + 1
        WHERE session_id = ? AND revoked_at IS NULL`, now, sessionID)
	if err != nil {
		return RevokeSessionFamilyResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销失败")
	}
	count, err := changed.RowsAffected()
	if err != nil {
		return RevokeSessionFamilyResult{}, platform.NewError(platform.CodeIntegrityError, false, "会话撤销计数不可读")
	}
	_, err = PublishIdentityEvent(ctx, op, PublishIdentityEventInput{EventID: platform.NewIdentifier(),
		Event: IdentityChanged{AccountID: accountID, SiteID: siteID,
			PermissionRevision: permissionRevision, CredentialRevision: credentialRevision,
			Reason: "session", OccurredAt: now}})
	if err != nil {
		return RevokeSessionFamilyResult{}, err
	}
	return RevokeSessionFamilyResult{Count: count}, nil
}

type RevokeSessionFamilyInput struct {
	FamilyID platform.Identifier `json:"familyId"`
	Reason   string              `json:"reason"`
}

type RevokeSessionFamilyResult struct {
	Count int64 `json:"count"`
}

type RevokeSessionInput struct {
	SessionID platform.Identifier `json:"sessionId"`
}
type RevokeSessionResult struct {
	Revoked bool `json:"revoked"`
}

// RevokeSession 撤销当前账户的指定设备，并保存不含敏感值的完整幂等回执。
func RevokeSession(ctx context.Context, op OperationContext, input RevokeSessionInput) (RevokeSessionResult, error) {
	if op.Actor.Kind != "admin" || !hasGrant(op.Actor.Grants, "account.self") {
		return RevokeSessionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "需要账户自身会话")
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return RevokeSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话私有存储尚未装配")
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return RevokeSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销事务不可用")
	}
	defer tx.Rollback()
	digest := platform.HashBytes(mustJSON(struct {
		Action    string              `json:"action"`
		Caller    string              `json:"caller"`
		DataEpoch platform.Identifier `json:"dataEpoch"`
		Input     RevokeSessionInput  `json:"input"`
	}{"identity.RevokeSession", op.CallerService, op.DataEpoch, input}))
	var previousDigest, resultHash, state string
	var resultJSON sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT input_digest, result_hash, state, result_json FROM operation_receipts
        WHERE site_id = ? AND actor_id = ? AND operation_id = ?`, op.Actor.SiteID, op.Actor.SubjectID, op.OperationID).
		Scan(&previousDigest, &resultHash, &state, &resultJSON)
	if err == nil {
		if previousDigest != digest {
			return RevokeSessionResult{}, platform.NewError(platform.CodeConflict, false, "幂等操作摘要不一致")
		}
		var result RevokeSessionResult
		if state != "committed" || !resultJSON.Valid || platform.HashBytes([]byte(resultJSON.String)) != resultHash ||
			json.Unmarshal([]byte(resultJSON.String), &result) != nil {
			return RevokeSessionResult{}, platform.NewError(platform.CodeIntegrityError, false, "会话撤销回执不完整")
		}
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return RevokeSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销回执不可读")
	}
	snapshot, err := readSessionSnapshot(ctx, tx, op.Actor.SessionID)
	if err != nil {
		return RevokeSessionResult{}, err
	}
	if snapshot.Session.AccountID != op.Actor.SubjectID || snapshot.Session.SiteID != op.Actor.SiteID || snapshot.Status != "active" ||
		op.Actor.SessionRevision <= 0 || snapshot.Session.Revision != op.Actor.SessionRevision {
		return RevokeSessionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "账户当前会话不匹配")
	}
	if err := validateSessionSnapshot(snapshot, op.DataEpoch, true); err != nil {
		return RevokeSessionResult{}, err
	}
	if snapshot.MustChangePassword && input.SessionID != op.Actor.SessionID {
		return RevokeSessionResult{}, platform.NewError(platform.CodeForbidden, false, "初始口令只允许退出当前会话")
	}

	var familyID sql.NullString
	var permissionRevision, credentialRevision platform.Revision
	err = tx.QueryRowContext(ctx, `SELECT f.family_id, a.permission_revision, c.credential_revision
        FROM sessions s JOIN accounts a ON a.account_id = s.account_id
        JOIN credentials c ON c.account_id = a.account_id
        LEFT JOIN session_families f ON f.session_id = s.session_id
        WHERE s.session_id = ? AND s.account_id = ? AND s.site_id = ?`,
		input.SessionID, op.Actor.SubjectID, op.Actor.SiteID).Scan(&familyID, &permissionRevision, &credentialRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return RevokeSessionResult{}, platform.NewError(platform.CodeForbidden, false, "会话不属于当前账户")
	}
	if err != nil {
		return RevokeSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话权威不可读")
	}
	if familyID.Valid {
		if _, err := RevokeSessionFamily(WithIdentityTransaction(ctx, tx), op, RevokeSessionFamilyInput{FamilyID: familyID.String, Reason: "logout"}); err != nil {
			return RevokeSessionResult{}, err
		}
	} else {
		now := platform.Now()
		changed, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = ?, revision = revision + 1
            WHERE session_id = ? AND revoked_at IS NULL`, now, input.SessionID)
		if err != nil {
			return RevokeSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销失败")
		}
		count, err := changed.RowsAffected()
		if err != nil {
			return RevokeSessionResult{}, platform.NewError(platform.CodeIntegrityError, false, "会话撤销计数不可读")
		}
		if count != 0 {
			_, err = PublishIdentityEvent(WithIdentityTransaction(ctx, tx), op, PublishIdentityEventInput{EventID: platform.NewIdentifier(),
				Event: IdentityChanged{AccountID: op.Actor.SubjectID, SiteID: op.Actor.SiteID,
					PermissionRevision: permissionRevision, CredentialRevision: credentialRevision, Reason: "session", OccurredAt: now}})
			if err != nil {
				return RevokeSessionResult{}, err
			}
		}
	}
	result := RevokeSessionResult{Revoked: true}
	raw, _ := json.Marshal(result)
	if _, err := tx.ExecContext(ctx, `INSERT INTO operation_receipts
        (operation_id, site_id, actor_id, input_digest, result_hash, state, created_at, result_json)
        VALUES (?, ?, ?, ?, ?, 'committed', ?, ?)`, op.OperationID, op.Actor.SiteID, op.Actor.SubjectID,
		digest, platform.HashBytes(raw), platform.Now(), string(raw)); err != nil {
		return RevokeSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销回执写入失败")
	}
	if err := tx.Commit(); err != nil {
		return RevokeSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销事务提交失败")
	}
	return result, nil
}

type RevokeOtherSessionsInput struct {
	AccountID     platform.Identifier `json:"accountId"`
	KeepSessionID platform.Identifier `json:"keepSessionId"`
}
type RevokeOtherSessionsResult struct {
	Count int64 `json:"count"`
}

// RevokeOtherSessions 在外层改密事务内保留当前设备，更新其凭据水位。
func RevokeOtherSessions(ctx context.Context, op OperationContext, input RevokeOtherSessionsInput) (RevokeOtherSessionsResult, error) {
	tx, ok := identityTransactionFromContext(ctx)
	if !ok || input.AccountID != op.Actor.SubjectID || input.KeepSessionID != op.Actor.SessionID {
		return RevokeOtherSessionsResult{}, platform.NewError(platform.CodeForbidden, false, "保留设备仅允许自身改密私有事务")
	}
	var revision, credentialRevision, permissionRevision platform.Revision
	var restricted bool
	var expiresAt platform.Timestamp
	var revoked sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT s.revision, c.credential_revision, a.permission_revision,
        a.must_change_password, s.absolute_expires_at, s.revoked_at
        FROM sessions s JOIN accounts a ON a.account_id = s.account_id
        JOIN credentials c ON c.account_id = a.account_id
        WHERE s.session_id = ? AND s.account_id = ? AND s.site_id = ? AND a.status = 'active'`,
		input.KeepSessionID, input.AccountID, op.Actor.SiteID).
		Scan(&revision, &credentialRevision, &permissionRevision, &restricted, &expiresAt, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return RevokeOtherSessionsResult{}, platform.NewError(platform.CodeUnauthenticated, false, "当前设备不可保留")
	}
	if err != nil {
		return RevokeOtherSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "当前设备权威不可读")
	}
	expiry, err := platform.ParseTimestamp(expiresAt)
	if err != nil {
		return RevokeOtherSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "会话期限损坏")
	}
	now := platform.Now()
	nowTime, _ := platform.ParseTimestamp(now)
	if revoked.Valid || !nowTime.Before(expiry) {
		return RevokeOtherSessionsResult{}, platform.NewError(platform.CodeUnauthenticated, false, "当前设备已失效")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE session_families SET revoked_at = ?, revision = revision + 1
        WHERE revoked_at IS NULL AND session_id IN
        (SELECT session_id FROM sessions WHERE account_id = ? AND site_id = ? AND session_id <> ?)`,
		now, input.AccountID, op.Actor.SiteID, input.KeepSessionID); err != nil {
		return RevokeOtherSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "其他会话族撤销失败")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = ?, revision = revision + 1
        WHERE account_id = ? AND site_id = ? AND session_id <> ? AND revoked_at IS NULL`,
		now, input.AccountID, op.Actor.SiteID, input.KeepSessionID)
	if err != nil {
		return RevokeOtherSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "其他会话撤销失败")
	}
	count, err := changed.RowsAffected()
	if err != nil || count > 10000 {
		return RevokeOtherSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "其他会话撤销数量超出合同")
	}
	changed, err = tx.ExecContext(ctx, `UPDATE sessions SET credential_revision = ?, permission_revision = ?,
        restricted_to_password_change = ?, revision = revision + 1 WHERE session_id = ? AND revision = ?`,
		credentialRevision, permissionRevision, restricted, input.KeepSessionID, revision)
	if err != nil {
		return RevokeOtherSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "当前设备水位写入失败")
	}
	affected, err := changed.RowsAffected()
	if err != nil || affected != 1 {
		return RevokeOtherSessionsResult{}, platform.NewError(platform.CodeConflict, false, "当前设备版本已变化")
	}
	return RevokeOtherSessionsResult{Count: count}, nil
}
