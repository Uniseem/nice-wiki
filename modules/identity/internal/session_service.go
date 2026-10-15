package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform/outbox"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// Session 是不含令牌、摘要、网络地址的会话元数据。
type Session struct {
	SessionID                  platform.Identifier `json:"sessionId"`
	AccountID                  platform.Identifier `json:"accountId"`
	SiteID                     platform.Identifier `json:"siteId"`
	CredentialRevision         platform.Revision   `json:"credentialRevision"`
	PermissionRevision         platform.Revision   `json:"permissionRevision"`
	IssuedAt                   platform.Timestamp  `json:"issuedAt"`
	ExpiresAt                  platform.Timestamp  `json:"expiresAt"`
	AbsoluteExpiresAt          platform.Timestamp  `json:"absoluteExpiresAt"`
	DeviceLabel                string              `json:"deviceLabel"`
	RestrictedToPasswordChange bool                `json:"restrictedToPasswordChange"`
	RevokedAt                  *platform.Timestamp `json:"revokedAt"`
	Revision                   platform.Revision   `json:"revision"`
}

// SessionPageRequest 使用公共分页协议；零值 limit 按默认20处理。
type SessionPageRequest struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

// SessionPageInfo 保留公共分页响应的省略语义。
type SessionPageInfo struct {
	NextCursor string `json:"nextCursor,omitempty"`
	HasMore    bool   `json:"hasMore"`
}

type ReadSessionsInput struct {
	Page SessionPageRequest `json:"page"`
}

type ReadSessionsResult struct {
	Sessions []Session       `json:"sessions"`
	PageInfo SessionPageInfo `json:"pageInfo"`
}

// SessionPageCursor 是受签名保护的私有分页位置；不能直接作为网络参数使用。
type SessionPageCursor struct {
	IssuedAt  platform.Timestamp  `json:"issuedAt"`
	SessionID platform.Identifier `json:"sessionId"`
}

type sessionCursorKey struct{}

// WithSessionCursorKey 注入本服务游标签名密钥引用；签名与验签统一由平台处理。
func WithSessionCursorKey(ctx context.Context, key platform.SigningKey) context.Context {
	return context.WithValue(ctx, sessionCursorKey{}, key)
}

// ReadSessions 仅查询当前主体的会话，按签发时间、会话ID升序进行键集分页。
func ReadSessions(ctx context.Context, op OperationContext, input ReadSessionsInput) (ReadSessionsResult, error) {
	if op.Actor.Kind != "admin" || !hasGrant(op.Actor.Grants, "account.self") {
		return ReadSessionsResult{}, platform.NewError(platform.CodeUnauthenticated, false, "需要账户自身会话")
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return ReadSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话私有存储尚未装配")
	}
	readTx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ReadSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话列表快照不可用")
	}
	defer readTx.Rollback()
	snapshot, err := readSessionSnapshot(ctx, readTx, op.Actor.SessionID)
	if err != nil {
		return ReadSessionsResult{}, err
	}
	if snapshot.Session.AccountID != op.Actor.SubjectID || snapshot.Session.SiteID != op.Actor.SiteID || snapshot.Status != "active" ||
		op.Actor.SessionRevision <= 0 || snapshot.Session.Revision != op.Actor.SessionRevision {
		return ReadSessionsResult{}, platform.NewError(platform.CodeUnauthenticated, false, "账户当前会话不匹配")
	}
	if err := validateSessionSnapshot(snapshot, op.DataEpoch, true); err != nil {
		return ReadSessionsResult{}, err
	}
	if snapshot.MustChangePassword {
		return ReadSessionsResult{}, platform.NewError(platform.CodeForbidden, false, "初始口令必须先修改")
	}
	limit := input.Page.Limit
	if limit == 0 {
		limit = 20
	}
	// Identifier不含竖线，分隔后的作用域无歧义，并在平台信封中一并签名。
	scope := "identity.sessions|" + op.Actor.SiteID + "|" + op.Actor.SubjectID + "|" + op.DataEpoch
	cursor := SessionPageCursor{}
	key, signed := ctx.Value(sessionCursorKey{}).(platform.SigningKey)
	if input.Page.Cursor != "" {
		if !signed {
			return ReadSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话游标签名器尚未装配")
		}
		raw, err := serviceauth.VerifyCursor(key, scope, input.Page.Cursor)
		if err != nil {
			return ReadSessionsResult{}, err
		}
		var position SessionPageCursor
		if err := json.Unmarshal(raw, &position); err != nil {
			return ReadSessionsResult{}, platform.NewError(platform.CodeValidation, false, "会话游标载荷不合法")
		}

		cursor = position
	}
	rows, err := readTx.QueryContext(ctx, `
        SELECT session_id, account_id, site_id, credential_revision, permission_revision,
            issued_at, expires_at, absolute_expires_at, device_label,
            restricted_to_password_change, revoked_at, revision
        FROM sessions WHERE account_id = ? AND site_id = ?
            AND (issued_at > ? OR (issued_at = ? AND session_id > ?))
        ORDER BY issued_at, session_id LIMIT ?`,
		op.Actor.SubjectID, op.Actor.SiteID, cursor.IssuedAt, cursor.IssuedAt, cursor.SessionID, limit+1)
	if err != nil {
		return ReadSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话列表不可读")
	}
	defer rows.Close()
	result := ReadSessionsResult{Sessions: make([]Session, 0, limit)}
	for rows.Next() {
		var session Session
		var revoked sql.NullString
		if err := rows.Scan(&session.SessionID, &session.AccountID, &session.SiteID,
			&session.CredentialRevision, &session.PermissionRevision, &session.IssuedAt,
			&session.ExpiresAt, &session.AbsoluteExpiresAt, &session.DeviceLabel,
			&session.RestrictedToPasswordChange, &revoked, &session.Revision); err != nil {
			return ReadSessionsResult{}, platform.NewError(platform.CodeIntegrityError, false, "会话记录不可读")
		}
		if len(result.Sessions) == limit {
			result.PageInfo.HasMore = true
			continue
		}
		if revoked.Valid {
			session.RevokedAt = &revoked.String
		}
		result.Sessions = append(result.Sessions, session)
	}
	if err := rows.Err(); err != nil {
		return ReadSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话列表读取失败")
	}
	if result.PageInfo.HasMore {
		if !signed {
			return ReadSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话游标签名器尚未装配")
		}
		last := result.Sessions[len(result.Sessions)-1]
		cursor.IssuedAt, cursor.SessionID = last.IssuedAt, last.SessionID
		raw, _ := json.Marshal(cursor)
		result.PageInfo.NextCursor, err = serviceauth.SignCursor(key, scope, raw)
		if err != nil {
			return ReadSessionsResult{}, err
		}
	}
	return result, nil
}

// SessionRuntime 由启动装配注入签名引用、公钥表与固定公开Origin。
type SessionRuntime struct {
	Key          platform.SigningKey
	Anchors      platform.TrustAnchors
	PublicOrigin string
}

type sessionRuntimeContextKey struct{}

// WithSessionRuntime 只把受信进程配置放入本域上下文，不接受请求自报配置。
func WithSessionRuntime(ctx context.Context, runtime SessionRuntime) context.Context {
	return context.WithValue(ctx, sessionRuntimeContextKey{}, runtime)
}

func sessionRuntimeFromContext(ctx context.Context) (SessionRuntime, error) {
	runtime, ok := ctx.Value(sessionRuntimeContextKey{}).(SessionRuntime)
	if !ok {
		return SessionRuntime{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话运行配置尚未装配")
	}
	return runtime, nil
}

type IssueSessionTokensInput struct {
	Session Session              `json:"session"`
	Auth    platform.AuthContext `json:"auth"`
}

type IssueSessionTokensResult struct {
	Tokens SessionTokens `json:"tokens"`
}

// IssueSessionTokens 在外层私有事务内轮换独立秘密，只将摘要写入数据库。
func IssueSessionTokens(ctx context.Context, op OperationContext, input IssueSessionTokensInput) (IssueSessionTokensResult, error) {
	tx, ok := identityTransactionFromContext(ctx)
	if !ok {
		return IssueSessionTokensResult{}, platform.NewError(platform.CodeForbidden, false, "会话签发仅允许身份私有事务")
	}
	runtime, err := sessionRuntimeFromContext(ctx)
	if err != nil {
		return IssueSessionTokensResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_state (singleton, auth_epoch) VALUES (1, ?)
        ON CONFLICT (singleton) DO NOTHING`, platform.NewIdentifier()); err != nil {
		return IssueSessionTokensResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "认证代次初始化失败")
	}
	var authEpoch platform.Identifier
	if err := tx.QueryRowContext(ctx, `SELECT auth_epoch FROM auth_state WHERE singleton = 1`).Scan(&authEpoch); err != nil {
		return IssueSessionTokensResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "认证代次不可读")
	}
	absolute, err := platform.ParseTimestamp(input.Session.AbsoluteExpiresAt)
	if err != nil {
		return IssueSessionTokensResult{}, platform.NewError(platform.CodeIntegrityError, false, "会话绝对期限损坏")
	}
	issued, err := platform.ParseTimestamp(input.Session.IssuedAt)
	if err != nil {
		return IssueSessionTokensResult{}, platform.NewError(platform.CodeIntegrityError, false, "会话签发时间损坏")
	}
	if cap := issued.Add(7 * 24 * time.Hour); cap.Before(absolute) {
		absolute = cap
	}
	now := time.Now().UTC().Truncate(time.Second)
	expires := now.Add(15 * time.Minute)
	if absolute.Before(expires) {
		expires = absolute.Truncate(time.Second)
	}
	if !now.Before(expires) {
		return IssueSessionTokensResult{}, platform.NewError(platform.CodeUnauthenticated, false, "会话绝对期限已到")
	}
	secrets, err := serviceauth.GenerateSessionSecrets()
	if err != nil {
		return IssueSessionTokensResult{}, err
	}
	token, err := serviceauth.SignSessionAccess(runtime.Key, platform.SessionAccessClaims{
		SessionID: input.Session.SessionID, AccountID: input.Session.AccountID, SiteID: input.Session.SiteID,
		CredentialRevision: input.Session.CredentialRevision, PermissionRevision: input.Session.PermissionRevision,
		SessionRevision: input.Session.Revision, AuthEpoch: authEpoch, DataEpoch: op.DataEpoch,
		IssuedAt: now.Unix(), ExpiresAt: expires.Unix(), TokenID: secrets.AccessNonce,
	})
	if err != nil {
		return IssueSessionTokensResult{}, err
	}
	var familyID, oldDigest string
	var familyRevision platform.Revision
	var revoked sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT family_id, refresh_digest, revision, revoked_at
        FROM session_families WHERE session_id = ?`, input.Session.SessionID).
		Scan(&familyID, &oldDigest, &familyRevision, &revoked)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err := tx.ExecContext(ctx, `INSERT INTO session_families
            (family_id, session_id, auth_epoch, data_epoch, refresh_digest, access_digest, csrf_digest, revision)
            VALUES (?, ?, ?, ?, ?, ?, ?, 1)`, platform.NewIdentifier(), input.Session.SessionID, authEpoch, op.DataEpoch,
			platform.HashBytes([]byte(secrets.RefreshToken)), platform.HashBytes([]byte(token)), platform.HashBytes([]byte(secrets.CSRFToken))); err != nil {
			return IssueSessionTokensResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话秘密摘要写入失败")
		}
	case err != nil:
		return IssueSessionTokensResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话族不可读")
	default:
		if revoked.Valid {
			return IssueSessionTokensResult{}, platform.NewError(platform.CodeUnauthenticated, false, "会话族已撤销")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO consumed_refresh_tokens (refresh_digest, family_id, consumed_at)
            VALUES (?, ?, ?)`, oldDigest, familyID, platform.Now()); err != nil {
			return IssueSessionTokensResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "已消费会话摘要写入失败")
		}
		changed, err := tx.ExecContext(ctx, `UPDATE session_families SET refresh_digest = ?, access_digest = ?, csrf_digest = ?,
            auth_epoch = ?, data_epoch = ?, revision = revision + 1 WHERE family_id = ? AND revision = ? AND revoked_at IS NULL`,
			platform.HashBytes([]byte(secrets.RefreshToken)), platform.HashBytes([]byte(token)), platform.HashBytes([]byte(secrets.CSRFToken)),
			authEpoch, op.DataEpoch, familyID, familyRevision)
		if err != nil {
			return IssueSessionTokensResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话秘密轮换失败")
		}
		count, err := changed.RowsAffected()
		if err != nil || count != 1 {
			return IssueSessionTokensResult{}, platform.NewError(platform.CodeConflict, false, "会话族版本已变化")
		}
	}
	expiresAt := platform.Timestamp(expires.Format(time.RFC3339Nano))
	changed, err := tx.ExecContext(ctx, `UPDATE sessions SET expires_at = ? WHERE session_id = ? AND revision = ? AND revoked_at IS NULL`,
		expiresAt, input.Session.SessionID, input.Session.Revision)
	if err != nil {
		return IssueSessionTokensResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话访问期限写入失败")
	}
	count, err := changed.RowsAffected()
	if err != nil || count != 1 {
		return IssueSessionTokensResult{}, platform.NewError(platform.CodeConflict, false, "会话版本已变化")
	}
	auth := input.Auth
	auth.ExpiresAt = expiresAt
	auth.SessionRevision = input.Session.Revision
	return IssueSessionTokensResult{Tokens: SessionTokens{SessionID: input.Session.SessionID, AccessToken: token,
		RefreshToken: secrets.RefreshToken, CSRFToken: secrets.CSRFToken, Auth: auth,
		MustChangePassword: input.Session.RestrictedToPasswordChange, ExpiresAt: expiresAt}}, nil
}

type RefreshSessionInput struct {
	RefreshToken string `json:"refreshToken"`
}
type RefreshSessionResult struct {
	Tokens SessionTokens `json:"tokens"`
}

// RefreshSession 单次消费refresh；重用旧值须先提交族撤销，再返回未认证。
func RefreshSession(ctx context.Context, op OperationContext, input RefreshSessionInput) (RefreshSessionResult, error) {
	if op.CallerService != "portal" {
		return RefreshSessionResult{}, platform.NewError(platform.CodeForbidden, false, "会话刷新仅允许portal")
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return RefreshSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话私有存储尚未装配")
	}
	if err := outbox.EnsureSchema(ctx, store.db); err != nil {
		return RefreshSessionResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return RefreshSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话刷新事务不可用")
	}
	defer tx.Rollback()
	digest := platform.HashBytes([]byte(input.RefreshToken))
	var sessionID string
	err = tx.QueryRowContext(ctx, `SELECT session_id FROM session_families WHERE refresh_digest = ?`, digest).Scan(&sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		var familyID, accountID, siteID string
		err = tx.QueryRowContext(ctx, `SELECT c.family_id, s.account_id, s.site_id FROM consumed_refresh_tokens c
            JOIN session_families f ON f.family_id = c.family_id JOIN sessions s ON s.session_id = f.session_id
            WHERE c.refresh_digest = ?`, digest).Scan(&familyID, &accountID, &siteID)
		if errors.Is(err, sql.ErrNoRows) {
			return RefreshSessionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "会话刷新凭据无效")
		}
		if err != nil {
			return RefreshSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话刷新摘要不可读")
		}
		revokeOp := op
		revokeOp.Actor = platform.AuthContext{Kind: "service", SubjectID: accountID, SiteID: siteID}
		if _, err := RevokeSessionFamily(WithIdentityTransaction(ctx, tx), revokeOp,
			RevokeSessionFamilyInput{FamilyID: familyID, Reason: "refresh_reuse"}); err != nil {
			return RefreshSessionResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return RefreshSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "旧会话凭据撤销提交失败")
		}
		return RefreshSessionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "会话刷新凭据已被消费")
	}
	if err != nil {
		return RefreshSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话刷新权威不可读")
	}
	snapshot, err := readSessionSnapshot(ctx, tx, sessionID)
	if err != nil {
		return RefreshSessionResult{}, err
	}
	if err := validateSessionSnapshot(snapshot, op.DataEpoch, false); err != nil {
		return RefreshSessionResult{}, err
	}
	if snapshot.Status != "active" {
		return RefreshSessionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "账户不可登录")
	}
	auth, err := sessionAuthContext(ctx, tx, snapshot)
	if err != nil {
		return RefreshSessionResult{}, err
	}
	changed, err := tx.ExecContext(ctx, `UPDATE sessions SET permission_revision = ?, revision = revision + 1 WHERE session_id = ? AND revision = ? AND revoked_at IS NULL`,
		snapshot.CurrentPermissionRevision, snapshot.Session.SessionID, snapshot.Session.Revision)
	if err != nil {
		return RefreshSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话轮换水位写入失败")
	}
	count, err := changed.RowsAffected()
	if err != nil || count != 1 {
		return RefreshSessionResult{}, platform.NewError(platform.CodeConflict, false, "会话版本已变化")
	}
	snapshot.Session.Revision++
	snapshot.Session.PermissionRevision = snapshot.CurrentPermissionRevision
	issued, err := IssueSessionTokens(WithIdentityTransaction(ctx, tx), op,
		IssueSessionTokensInput{Session: snapshot.Session, Auth: auth})
	if err != nil {
		return RefreshSessionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return RefreshSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话刷新提交失败")
	}
	return RefreshSessionResult{Tokens: issued.Tokens}, nil
}
