package identity

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// AuthorizationRequest 的access和已验签委托两种来源不能混用。
type AuthorizationRequest struct {
	AccessToken                string               `json:"accessToken,omitempty"`
	SubjectID                  platform.Identifier  `json:"subjectId,omitempty"`
	SessionID                  *platform.Identifier `json:"sessionId,omitempty"`
	SiteID                     platform.Identifier  `json:"siteId,omitempty"`
	Permission                 string               `json:"permission"`
	Action                     string               `json:"action,omitempty"`
	ResourceID                 *platform.Identifier `json:"resourceId,omitempty"`
	ExpectedPermissionRevision *platform.Revision   `json:"expectedPermissionRevision,omitempty"`
}

type AuthorizationDecision struct {
	AccountName       AccountName          `json:"accountName"`
	Authorized        bool                 `json:"authorized"`
	Auth              platform.AuthContext `json:"auth"`
	EvaluatedRevision platform.Revision    `json:"evaluatedRevision"`
	Reason            string               `json:"reason"`
}

type sessionQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type sessionSnapshot struct {
	AccountName               AccountName
	Session                   Session
	Status                    string
	IsSuperadmin              bool
	MustChangePassword        bool
	CurrentCredentialRevision platform.Revision
	CurrentPermissionRevision platform.Revision
	FamilyAuthEpoch           platform.Identifier
	FamilyDataEpoch           platform.Identifier
	CurrentAuthEpoch          platform.Identifier
	AccessDigest              platform.Hash
	CSRFDigest                platform.Hash
	FamilyRevoked             bool
}

// readSessionSnapshot 从一个数据库快照同时读取会话、账户、族与认证代次权威。
func readSessionSnapshot(ctx context.Context, query sessionQueryer, sessionID platform.Identifier) (sessionSnapshot, error) {
	var result sessionSnapshot
	var sessionRevoked, familyRevoked sql.NullString
	s := &result.Session
	err := query.QueryRowContext(ctx, `SELECT s.session_id, s.account_id, s.site_id, s.credential_revision,
        s.permission_revision, s.issued_at, s.expires_at, s.absolute_expires_at, s.device_label,
        s.restricted_to_password_change, s.revoked_at, s.revision, a.status, a.is_superadmin,
        a.must_change_password, c.credential_revision, a.permission_revision,
        f.auth_epoch, f.data_epoch, f.access_digest, f.csrf_digest, f.revoked_at, au.auth_epoch, a.account_name
        FROM sessions s JOIN accounts a ON a.account_id = s.account_id AND a.site_id = s.site_id
        JOIN credentials c ON c.account_id = s.account_id JOIN session_families f ON f.session_id = s.session_id
        JOIN auth_state au ON au.singleton = 1 WHERE s.session_id = ?`, sessionID).
		Scan(&s.SessionID, &s.AccountID, &s.SiteID, &s.CredentialRevision, &s.PermissionRevision,
			&s.IssuedAt, &s.ExpiresAt, &s.AbsoluteExpiresAt, &s.DeviceLabel, &s.RestrictedToPasswordChange,
			&sessionRevoked, &s.Revision, &result.Status, &result.IsSuperadmin, &result.MustChangePassword,
			&result.CurrentCredentialRevision, &result.CurrentPermissionRevision, &result.FamilyAuthEpoch,
			&result.FamilyDataEpoch, &result.AccessDigest, &result.CSRFDigest, &familyRevoked, &result.CurrentAuthEpoch, &result.AccountName)
	if errors.Is(err, sql.ErrNoRows) {
		return sessionSnapshot{}, platform.NewError(platform.CodeUnauthenticated, false, "会话不存在或尚未签发")
	}
	if err != nil {
		return sessionSnapshot{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话授权权威不可读")
	}
	if sessionRevoked.Valid {
		s.RevokedAt = &sessionRevoked.String
	}
	result.FamilyRevoked = familyRevoked.Valid
	return result, nil
}

// validateSessionSnapshot 核验权威水位与撤销；refresh允许访问过期与旧权限版本，由同事务更新权限。
func validateSessionSnapshot(snapshot sessionSnapshot, dataEpoch platform.Identifier, requireAccess bool) error {
	s := snapshot.Session
	if s.RevokedAt != nil || snapshot.FamilyRevoked || snapshot.FamilyAuthEpoch != snapshot.CurrentAuthEpoch ||
		snapshot.FamilyDataEpoch != dataEpoch || s.CredentialRevision != snapshot.CurrentCredentialRevision ||
		(requireAccess && s.PermissionRevision != snapshot.CurrentPermissionRevision) || s.RestrictedToPasswordChange != snapshot.MustChangePassword {
		return platform.NewError(platform.CodeUnauthenticated, false, "会话已撤销或授权水位已变化")
	}
	absolute, err := platform.ParseTimestamp(s.AbsoluteExpiresAt)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "会话绝对期限损坏")
	}
	now := time.Now().UTC()
	if !now.Before(absolute) {
		return platform.NewError(platform.CodeUnauthenticated, false, "会话绝对期限已到")
	}
	if requireAccess {
		expires, err := platform.ParseTimestamp(s.ExpiresAt)
		if err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "会话访问期限损坏")
		}
		if !now.Before(expires) {
			return platform.NewError(platform.CodeUnauthenticated, false, "会话访问期限已到")
		}
	}
	return nil
}

// sessionAuthContext 只从当前账户的功能表派生权限，不接收客户端grants。
func sessionAuthContext(ctx context.Context, query sessionQueryer, snapshot sessionSnapshot) (platform.AuthContext, error) {
	rows, err := query.QueryContext(ctx, `SELECT permission FROM account_permissions WHERE account_id = ? ORDER BY permission`, snapshot.Session.AccountID)
	if err != nil {
		return platform.AuthContext{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户功能权限不可读")
	}
	permissions := make(PermissionSelection, 0)
	for rows.Next() {
		var permission FunctionPermission
		if err := rows.Scan(&permission); err != nil {
			rows.Close()
			return platform.AuthContext{}, platform.NewError(platform.CodeIntegrityError, false, "账户功能权限损坏")
		}
		permissions = append(permissions, permission)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return platform.AuthContext{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户功能权限读取失败")
	}
	mapped, err := MapFunctionGrants(MapFunctionGrantsInput{Permissions: permissions, IsSuperadmin: snapshot.IsSuperadmin})
	if err != nil {
		return platform.AuthContext{}, platform.NewError(platform.CodeIntegrityError, false, "账户功能权限与登记不一致")
	}
	grants := mapped.Grants
	if snapshot.MustChangePassword {
		grants = []string{"account.self"}
	}
	if snapshot.Status != "active" {
		grants = []string{}
	}
	return platform.AuthContext{SubjectID: snapshot.Session.AccountID, SiteID: snapshot.Session.SiteID,
		Kind: "admin", SessionID: snapshot.Session.SessionID, SessionRevision: snapshot.Session.Revision, PermissionRevision: snapshot.CurrentPermissionRevision,
		Grants: grants, ExpiresAt: snapshot.Session.ExpiresAt}, nil
}

// verifySessionAccess 同时检查平台签名与本域摘要、水位；签名有效不代替即时撤销。
func verifySessionAccess(ctx context.Context, query sessionQueryer, runtime SessionRuntime, dataEpoch platform.Identifier, token string) (sessionSnapshot, error) {
	claims, err := serviceauth.VerifySessionAccess(runtime.Anchors, token)
	if err != nil {
		return sessionSnapshot{}, err
	}
	snapshot, err := readSessionSnapshot(ctx, query, claims.SessionID)
	if err != nil {
		return sessionSnapshot{}, err
	}
	s := snapshot.Session
	digest := platform.HashBytes([]byte(token))
	if subtle.ConstantTimeCompare([]byte(snapshot.AccessDigest), []byte(digest)) != 1 ||
		claims.AccountID != s.AccountID || claims.SiteID != s.SiteID || claims.CredentialRevision != s.CredentialRevision ||
		claims.PermissionRevision != s.PermissionRevision || claims.SessionRevision != s.Revision ||
		claims.AuthEpoch != snapshot.CurrentAuthEpoch || claims.DataEpoch != dataEpoch {
		return sessionSnapshot{}, platform.NewError(platform.CodeUnauthenticated, false, "会话令牌与当前权威不一致")
	}
	if err := validateSessionSnapshot(snapshot, dataEpoch, true); err != nil {
		return sessionSnapshot{}, err
	}
	return snapshot, nil
}

// CheckAuthorization 从固定access或可信委托解析主体，以当前功能权威作单次判断。
func CheckAuthorization(ctx context.Context, op OperationContext, input AuthorizationRequest) (AuthorizationDecision, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return AuthorizationDecision{}, platform.NewError(platform.CodeDependencyUnavailable, true, "授权私有存储尚未装配")
	}
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return AuthorizationDecision{}, platform.NewError(platform.CodeDependencyUnavailable, true, "授权读取事务不可用")
	}
	defer tx.Rollback()
	var snapshot sessionSnapshot
	if input.AccessToken != "" {
		if op.CallerService != "portal" {
			return AuthorizationDecision{}, platform.NewError(platform.CodeForbidden, false, "access解析仅允许portal")
		}
		runtime, err := sessionRuntimeFromContext(ctx)
		if err != nil {
			return AuthorizationDecision{}, err
		}
		snapshot, err = verifySessionAccess(ctx, tx, runtime, op.DataEpoch, input.AccessToken)
		if err != nil {
			return AuthorizationDecision{}, err
		}
	} else {
		if op.Actor.Kind != "admin" || input.SubjectID != op.Actor.SubjectID || input.SiteID != op.Actor.SiteID ||
			input.SessionID == nil || *input.SessionID != op.Actor.SessionID {
			return AuthorizationDecision{}, platform.NewError(platform.CodeForbidden, false, "授权主体必须来自可信用户委托")
		}
		snapshot, err = readSessionSnapshot(ctx, tx, *input.SessionID)
		if err != nil {
			return AuthorizationDecision{}, err
		}
		if snapshot.Session.AccountID != input.SubjectID ||
			snapshot.CurrentPermissionRevision != op.Actor.PermissionRevision || op.Actor.SessionRevision <= 0 ||
			snapshot.Session.Revision != op.Actor.SessionRevision {
			return AuthorizationDecision{}, platform.NewError(platform.CodeUnauthenticated, false, "委托会话或授权版本已变化")
		}
		if err := validateSessionSnapshot(snapshot, op.DataEpoch, true); err != nil {
			return AuthorizationDecision{}, err
		}
	}
	if input.ExpectedPermissionRevision != nil && *input.ExpectedPermissionRevision != snapshot.CurrentPermissionRevision {
		return AuthorizationDecision{}, platform.NewError(platform.CodeUnauthenticated, false, "观察权限版本已失效")
	}
	auth, err := sessionAuthContext(ctx, tx, snapshot)
	if err != nil {
		return AuthorizationDecision{}, err
	}
	// 访问的站点与会话家账户不同：按成员角色（站主按全部权限）即时派生该站点的权限。
	siteDenied := ""
	if input.SiteID != "" && input.SiteID != snapshot.Session.SiteID && snapshot.Status == "active" && !snapshot.MustChangePassword {
		auth, siteDenied, err = targetSiteAuth(ctx, tx, snapshot, auth, input.SiteID)
		if err != nil {
			return AuthorizationDecision{}, err
		}
	}
	decision := AuthorizationDecision{AccountName: snapshot.AccountName, Auth: auth, EvaluatedRevision: snapshot.CurrentPermissionRevision, Reason: "allowed"}
	switch {
	case siteDenied != "":
		decision.Reason = siteDenied
	case snapshot.Status == "disabled":
		decision.Reason = "disabled"
	case snapshot.Status == "deleted":
		decision.Reason = "deleted"
	case snapshot.MustChangePassword && (input.Permission != "account.self" ||
		(input.Action != "identity:changePassword" && input.Action != "identity:revokeSession" && input.Action != "identity:readSelf")):
		decision.Reason = "must_change_password"
	case !snapshot.IsSuperadmin && hasGrant(platformOnlyGrants, input.Permission):
		decision.Reason = "superadmin_only"
	case !hasGrant(auth.Grants, input.Permission):
		decision.Reason = "missing_function"
	default:
		decision.Authorized = true
	}
	return decision, nil
}

// targetSiteAuth 在站点成员表里为当前用户派生目标站点的权限上下文；
// 返回的第二个值非空表示该访问被拒绝（原因写进决定的 reason）。
func targetSiteAuth(ctx context.Context, q sessionQueryer, snapshot sessionSnapshot, home platform.AuthContext, siteID string) (platform.AuthContext, string, error) {
	var status string
	err := q.QueryRowContext(ctx, `SELECT status FROM sites WHERE site_id = ?`, siteID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return home, "unknown_site", nil
	}
	if err != nil {
		return home, "", platform.NewError(platform.CodeDependencyUnavailable, true, "站点权威不可读")
	}
	if status == "deleted" {
		return home, "site_deleted", nil
	}
	auth := home
	auth.SiteID = siteID
	if snapshot.IsSuperadmin {
		mapped, err := MapFunctionGrants(MapFunctionGrantsInput{IsSuperadmin: true})
		if err != nil {
			return home, "", err
		}
		auth.Grants = mapped.Grants
		return auth, "", nil
	}
	var role string
	err = q.QueryRowContext(ctx, `SELECT m.role FROM site_members m JOIN users u ON u.user_id = m.user_id WHERE m.site_id = ? AND u.account_id = ? AND u.status = 'active'`,
		siteID, snapshot.Session.AccountID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return home, "not_member", nil
	}
	if err != nil {
		return home, "", platform.NewError(platform.CodeDependencyUnavailable, true, "站点成员权威不可读")
	}
	if status != "active" {
		return home, "site_suspended", nil
	}
	grants, err := roleGrants(role)
	if err != nil {
		return home, "", err
	}
	auth.Grants = grants
	return auth, "", nil
}
