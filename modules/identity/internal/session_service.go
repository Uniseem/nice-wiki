package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/identity/internal/gen"
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

type CreateSessionInput struct {
	AccountName    AccountName `json:"accountName"`
	Password       string      `json:"password"`
	DeviceLabel    string      `json:"deviceLabel"`
	ChallengeID    string      `json:"challengeId,omitempty"`
	ChallengeProof string      `json:"challengeProof,omitempty"`
}
type CreateSessionResult struct {
	Tokens    SessionTokens `json:"tokens"`
	NextRoute string        `json:"nextRoute"`
}

// LoginRuntime 只持有按版本化合同签名调用的风控客户端，不接收浏览器伪造的作用域。
type LoginRuntime struct{ Client *gen.TrafficGuardClient }
type loginRuntimeContextKey struct{}
type loginAttempt struct {
	AttemptID string             `json:"attemptId"`
	ExpiresAt platform.Timestamp `json:"expiresAt"`
	Revision  platform.Revision  `json:"revision"`
}

func WithLoginRuntime(ctx context.Context, runtime LoginRuntime) context.Context {
	return context.WithValue(ctx, loginRuntimeContextKey{}, runtime)
}

// challengeID/challengeProof 是账号近期多次失败后风控要求的 login-pow 证明，平时为空。
func beginIdentityLogin(ctx context.Context, op OperationContext, accountName, action, challengeID, challengeProof string) (loginAttempt, error) {
	runtime, ok := ctx.Value(loginRuntimeContextKey{}).(LoginRuntime)
	if !ok || runtime.Client == nil {
		return loginAttempt{}, platform.NewError(platform.CodeDependencyUnavailable, true, "登录风控尚未装配")
	}
	if op.CallerService != "portal" || len(op.LoginNetworkScopes) < 1 || len(op.LoginNetworkScopes) > 2 {
		return loginAttempt{}, platform.NewError(platform.CodeForbidden, false, "登录缺少受信网络作用域")
	}
	session, err := sessionRuntimeFromContext(ctx)
	if err != nil {
		return loginAttempt{}, err
	}
	accountKey, err := serviceauth.PrivateOperationDigest(session.Key, "login-account/v1", []byte(accountName))
	if err != nil {
		return loginAttempt{}, err
	}
	body := struct {
		RequestID      string          `json:"requestId"`
		AccountKey     string          `json:"accountKey"`
		NetworkKeys    []platform.Hash `json:"networkKeys"`
		Action         string          `json:"action"`
		ChallengeID    string          `json:"challengeId,omitempty"`
		ChallengeProof string          `json:"challengeProof,omitempty"`
	}{
		op.OperationID, accountKey, op.LoginNetworkScopes, action, challengeID, challengeProof}
	raw, err := runtime.Client.BeginLoginAttempt(platform.WithOperationContext(ctx, op), body, op.OperationID)
	if err != nil {
		return loginAttempt{}, err
	}
	var attempt loginAttempt
	if json.Unmarshal(raw, &attempt) != nil || !platform.ValidIdentifier(attempt.AttemptID) || attempt.Revision < 1 {
		return loginAttempt{}, platform.NewError(platform.CodeDependencyUnavailable, true, "登录风控响应不可用")
	}
	expires, err := platform.ParseTimestamp(attempt.ExpiresAt)
	if err != nil || !time.Now().UTC().Before(expires) {
		return loginAttempt{}, platform.NewError(platform.CodeUnauthenticated, false, "登录准入已到期")
	}
	return attempt, nil
}

func finishIdentityLogin(ctx context.Context, op OperationContext, attempt loginAttempt, outcome string) error {
	runtime, ok := ctx.Value(loginRuntimeContextKey{}).(LoginRuntime)
	if !ok || runtime.Client == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "登录风控尚未装配")
	}
	body := struct {
		Outcome string `json:"outcome"`
	}{outcome}
	raw, err := runtime.Client.FinishLoginAttempt(platform.WithOperationContext(ctx, op), attempt.AttemptID, body, op.OperationID)
	if err != nil {
		return err
	}
	var response struct {
		Accepted  bool   `json:"accepted"`
		AttemptID string `json:"attemptId"`
	}
	if json.Unmarshal(raw, &response) != nil || !response.Accepted || response.AttemptID != attempt.AttemptID {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "登录风控终局响应不可用")
	}
	return nil
}

func insertSession(ctx context.Context, tx *sql.Tx, account Account, credentialRevision platform.Revision, deviceLabel string) (Session, platform.AuthContext, error) {
	now := time.Now().UTC()
	session := Session{SessionID: platform.NewIdentifier(), AccountID: account.AccountID, SiteID: account.SiteID,
		CredentialRevision: credentialRevision, PermissionRevision: account.PermissionRevision,
		IssuedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(15 * time.Minute).Format(time.RFC3339Nano),
		AbsoluteExpiresAt: now.Add(7 * 24 * time.Hour).Format(time.RFC3339Nano), DeviceLabel: deviceLabel, RestrictedToPasswordChange: account.MustChangePassword, Revision: 1}
	_, err := tx.ExecContext(ctx, `INSERT INTO sessions(session_id,account_id,site_id,credential_revision,permission_revision,
        issued_at,expires_at,absolute_expires_at,device_label,restricted_to_password_change,revision) VALUES(?,?,?,?,?,?,?,?,?,?,1)`,
		session.SessionID, session.AccountID, session.SiteID, session.CredentialRevision, session.PermissionRevision,
		session.IssuedAt, session.ExpiresAt, session.AbsoluteExpiresAt, session.DeviceLabel, session.RestrictedToPasswordChange)
	if err != nil {
		return Session{}, platform.AuthContext{}, identityDBError()
	}
	mapped, err := MapFunctionGrants(MapFunctionGrantsInput{Permissions: toFunctionPermissions(account.Permissions), IsSuperadmin: account.IsSuperadmin})
	if err != nil {
		return Session{}, platform.AuthContext{}, err
	}
	grants := mapped.Grants
	if account.MustChangePassword {
		grants = []string{"account.self"}
	}
	auth := platform.AuthContext{SubjectID: account.AccountID, Kind: "admin", SiteID: account.SiteID, SessionID: session.SessionID,
		SessionRevision: 1, PermissionRevision: account.PermissionRevision, Grants: grants, ExpiresAt: session.ExpiresAt}
	return session, auth, nil
}

// CreateSession 先完成账号与网络的风控终局，再原子签发本次唯一敏感结果。
func CreateSession(ctx context.Context, op OperationContext, input CreateSessionInput) (CreateSessionResult, error) {
	ctx, op, err := anonymousIdentityOperation(ctx, op)
	if err != nil {
		return CreateSessionResult{}, err
	}
	normalized, err := NormalizeAccountName(NormalizeAccountNameInput{AccountName: input.AccountName})
	if err != nil {
		return CreateSessionResult{}, err
	}
	attempt, err := beginIdentityLogin(ctx, op, normalized.Normalized, "login", input.ChallengeID, input.ChallengeProof)
	if err != nil {
		return CreateSessionResult{}, err
	}
	outcomeSent := false
	defer func() {
		if !outcomeSent {
			_ = finishIdentityLogin(ctx, op, attempt, "aborted")
		}
	}()
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return CreateSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话存储尚未装配")
	}
	digest, keyID, err := privateIdentityDigest(ctx, op, "createSession", input)
	if err != nil {
		return CreateSessionResult{}, err
	}
	readTx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return CreateSessionResult{}, identityDBError()
	}
	var receipt sensitiveOperationReceipt
	found, err := readIdentityReceipt(ctx, op, readTx, digest, keyID, &receipt)
	if err != nil {
		readTx.Rollback()
		return CreateSessionResult{}, err
	}
	if found {
		readTx.Rollback()
		outcomeSent = true
		return CreateSessionResult{}, sensitiveReplayError()
	}
	var accountID, siteID, passwordHash, status string
	var credentialRevision platform.Revision
	err = readTx.QueryRowContext(ctx, `SELECT a.account_id,a.site_id,c.password_hash,c.credential_revision,a.status FROM accounts a JOIN credentials c ON c.account_id=a.account_id WHERE a.account_name=?`, normalized.Normalized).
		Scan(&accountID, &siteID, &passwordHash, &credentialRevision, &status)
	readTx.Rollback()
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := VerifyPassword(VerifyPasswordInput{Password: input.Password, EncodedHash: dummyPasswordHash}); err != nil {
			return CreateSessionResult{}, err
		}
		outcomeSent = true
		if err := finishIdentityLogin(ctx, op, attempt, "rejected"); err != nil {
			return CreateSessionResult{}, err
		}
		return CreateSessionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "账号或口令不正确")
	}
	if err != nil {
		return CreateSessionResult{}, identityDBError()
	}
	verified, err := VerifyPassword(VerifyPasswordInput{Password: input.Password, EncodedHash: passwordHash})
	if err != nil {
		return CreateSessionResult{}, err
	}
	if !verified.Valid || status != "active" {
		outcomeSent = true
		if err := finishIdentityLogin(ctx, op, attempt, "rejected"); err != nil {
			return CreateSessionResult{}, err
		}
		return CreateSessionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "账号或口令不正确")
	}
	rehashed := ""
	if verified.NeedsRehash {
		hash, err := HashPassword(ctx, op, HashPasswordInput{Password: input.Password})
		if err != nil {
			return CreateSessionResult{}, err
		}
		rehashed = hash.EncodedHash
	}
	runtime := registrationRuntimeFromContext(ctx)
	if runtime == nil {
		return CreateSessionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "初始化状态客户端尚未装配")
	}
	initialization, err := runtime.ReadInitializationStatus(ctx, op)
	if err != nil {
		return CreateSessionResult{}, err
	}
	if !isHomeSite(siteID) && initialization.SiteID != nil && *initialization.SiteID != siteID {
		return CreateSessionResult{}, platform.NewError(platform.CodeIntegrityError, false, "站点配置与账号归属不一致")
	}
	outcomeSent = true
	if err := finishIdentityLogin(ctx, op, attempt, "authenticated"); err != nil {
		return CreateSessionResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return CreateSessionResult{}, identityDBError()
	}
	defer tx.Rollback()
	found, err = readIdentityReceipt(ctx, op, tx, digest, keyID, &receipt)
	if err != nil {
		return CreateSessionResult{}, err
	}
	if found {
		return CreateSessionResult{}, sensitiveReplayError()
	}
	account, err := readAccount(ctx, tx, accountID, siteID)
	if err != nil {
		return CreateSessionResult{}, err
	}
	var currentRevision platform.Revision
	if err := tx.QueryRowContext(ctx, `SELECT credential_revision FROM credentials WHERE account_id=?`, accountID).Scan(&currentRevision); err != nil {
		return CreateSessionResult{}, identityDBError()
	}
	if account.Status != AccountStatusActive || currentRevision != credentialRevision {
		return CreateSessionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "账号或口令不正确")
	}
	if rehashed != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE credentials SET password_hash=? WHERE account_id=? AND credential_revision=? AND password_hash=?`, rehashed, accountID, credentialRevision, passwordHash); err != nil {
			return CreateSessionResult{}, identityDBError()
		}
	}
	session, auth, err := insertSession(ctx, tx, account, credentialRevision, input.DeviceLabel)
	if err != nil {
		return CreateSessionResult{}, err
	}
	issued, err := IssueSessionTokens(WithIdentityTransaction(ctx, tx), op, IssueSessionTokensInput{Session: session, Auth: auth})
	if err != nil {
		return CreateSessionResult{}, err
	}
	receipt = sensitiveOperationReceipt{AccountID: accountID, SessionID: session.SessionID, CredentialRevision: credentialRevision}
	if err := saveIdentityReceipt(ctx, op, tx, digest, keyID, receipt); err != nil {
		return CreateSessionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CreateSessionResult{}, identityDBError()
	}
	next := "/admin"
	if account.MustChangePassword {
		next = "/admin/me"
	} else if !initialization.Initialized {
		next = "/admin/setup"
	}
	return CreateSessionResult{Tokens: issued.Tokens, NextRoute: next}, nil
}

// anonymousIdentityOperation 固定匿名凭证动作的幂等主体，不因可选cookie或委托变化。
func anonymousIdentityOperation(ctx context.Context, op OperationContext) (context.Context, OperationContext, error) {
	if op.CallerService != "portal" {
		return nil, OperationContext{}, platform.NewError(platform.CodeForbidden, false, "匿名凭证入口仅允许portal")
	}
	op.Actor = platform.AuthContext{Kind: "service", SubjectID: "portal", SiteID: "system", Grants: []string{}, ExpiresAt: op.Deadline}
	ctx = platform.WithPortalProof(ctx, "")
	return platform.WithOperationContext(ctx, op), op, nil
}
