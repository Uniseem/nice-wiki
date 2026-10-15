package identity

import (
	"context"
	"database/sql"
	"encoding/json"

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
	Scope     string              `json:"scope"`
	SiteID    platform.Identifier `json:"siteId"`
	AccountID platform.Identifier `json:"accountId"`
	DataEpoch platform.Identifier `json:"dataEpoch"`
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
	limit := input.Page.Limit
	if limit == 0 {
		limit = 20
	}
	cursor := SessionPageCursor{
		Scope: "identity.sessions", SiteID: op.Actor.SiteID,
		AccountID: op.Actor.SubjectID, DataEpoch: op.DataEpoch,
	}
	key, signed := ctx.Value(sessionCursorKey{}).(platform.SigningKey)
	if input.Page.Cursor != "" {
		if !signed {
			return ReadSessionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话游标签名器尚未装配")
		}
		raw, err := serviceauth.VerifyCursor(key, cursor.Scope, input.Page.Cursor)
		if err != nil {
			return ReadSessionsResult{}, err
		}
		var position SessionPageCursor
		if err := json.Unmarshal(raw, &position); err != nil {
			return ReadSessionsResult{}, platform.NewError(platform.CodeValidation, false, "会话游标载荷不合法")
		}
		if position.Scope != cursor.Scope || position.SiteID != cursor.SiteID ||
			position.AccountID != cursor.AccountID || position.DataEpoch != cursor.DataEpoch {
			return ReadSessionsResult{}, platform.NewError(platform.CodeValidation, false, "会话游标作用域不一致")
		}
		cursor = position
	}
	rows, err := store.db.QueryContext(ctx, `
        SELECT session_id, account_id, site_id, credential_revision, permission_revision,
            issued_at, expires_at, absolute_expires_at, device_label,
            restricted_to_password_change, revoked_at, revision
        FROM sessions WHERE account_id = ? AND site_id = ?
            AND (issued_at > ? OR (issued_at = ? AND session_id > ?))
        ORDER BY issued_at, session_id LIMIT ?`,
		cursor.AccountID, cursor.SiteID, cursor.IssuedAt, cursor.IssuedAt, cursor.SessionID, limit+1)
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
		result.PageInfo.NextCursor, err = serviceauth.SignCursor(key, cursor.Scope, raw)
		if err != nil {
			return ReadSessionsResult{}, err
		}
	}
	return result, nil
}
