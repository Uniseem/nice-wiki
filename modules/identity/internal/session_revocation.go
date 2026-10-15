package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
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
		if err := outbox.EnsureSchema(ctx, store.db); err != nil {
			return RevokeAllSessionsResult{}, err
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
	changed, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = ?, revision = revision + 1
        WHERE account_id = ? AND site_id = ? AND revoked_at IS NULL`, now, input.AccountID, op.Actor.SiteID)
	if err != nil {
		return RevokeAllSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话撤销写入失败")
	}
	count, err := changed.RowsAffected()
	if err != nil || count > 10000 {
		return RevokeAllSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "会话撤销数量超出合同")
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
