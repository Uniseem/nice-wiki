package identity

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/identity/internal/gen"
)

// ReadAuthOptions 给登录页：能不能注册、是否要邀请码、有哪些第三方登录。
func ReadAuthOptions(ctx context.Context, op OperationContext, _ struct{}) (gen.ReadAuthOptionsResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.ReadAuthOptionsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return gen.ReadAuthOptionsResult{}, err
	}
	result := gen.ReadAuthOptionsResult{RegistrationOpen: settings.RegistrationOpen, EmailRegistration: settings.EmailRegistration,
		InviteRequired: settings.InviteRequired && !settings.RegistrationOpen, Providers: []gen.AuthProvider{}}
	for _, name := range allProviders {
		conf := settings.Providers[name]
		if conf.Enabled && conf.ClientID != "" && hasProviderSecret(ctx, store.db, name) {
			result.Providers = append(result.Providers, gen.AuthProvider{Provider: gen.AuthProviderProvider(name), Label: providerLabels[name]})
		}
	}
	return result, nil
}

type newUser struct {
	Email, DisplayName, Avatar, AccountName, PasswordHash string
}

// createUserTx 在事务里建用户、家账户（无任何功能权限）和默认额度。
func createUserTx(ctx context.Context, tx *sql.Tx, settings platformSettings, nu newUser) (Account, string, error) {
	userID, accountID, now := platform.NewIdentifier(), platform.NewIdentifier(), platform.Now()
	account := Account{AccountID: accountID, SiteID: homeSiteID(userID), AccountName: nu.AccountName, Permissions: []string{},
		Status: AccountStatusActive, Revision: 1, PermissionRevision: 1, CreatedAt: now, UpdatedAt: now}
	if err := insertAccount(ctx, tx, account, nu.PasswordHash); err != nil {
		return Account{}, "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO users(user_id,account_id,email,display_name,avatar_url,status,created_at,updated_at) VALUES(?,?,?,?,?,'active',?,?)`,
		userID, accountID, nu.Email, nu.DisplayName, nu.Avatar, now, now); err != nil {
		if stringsContains(err.Error(), "UNIQUE constraint failed") {
			return Account{}, "", platform.NewError(platform.CodeConflict, false, "邮箱已注册")
		}
		return Account{}, "", identityDBError()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_limits(user_id,max_sites,storage_bytes,updated_at) VALUES(?,?,?,?)`,
		userID, settings.DefaultMaxSites, settings.DefaultStorageBytes, now); err != nil {
		return Account{}, "", identityDBError()
	}
	return account, userID, nil
}

// issueSessionTx 给已存在的家账户签发会话（口令已验证或第三方身份已验证之后）。
func issueSessionTx(ctx context.Context, op OperationContext, tx *sql.Tx, account Account) (SessionTokens, error) {
	var credentialRevision int64
	if err := tx.QueryRowContext(ctx, `SELECT credential_revision FROM credentials WHERE account_id=?`, account.AccountID).Scan(&credentialRevision); err != nil {
		return SessionTokens{}, identityDBError()
	}
	session, auth, err := insertSession(ctx, tx, account, platform.Revision(credentialRevision), "")
	if err != nil {
		return SessionTokens{}, err
	}
	inner := WithIdentityTransaction(ctx, tx)
	issued, err := IssueSessionTokens(inner, op, IssueSessionTokensInput{Session: session, Auth: auth})
	if err != nil {
		return SessionTokens{}, err
	}
	return issued.Tokens, nil
}

func publishUserCreated(ctx context.Context, op OperationContext, tx *sql.Tx, account Account) error {
	inner := WithIdentityTransaction(ctx, tx)
	eventOp := op
	eventOp.Actor = platform.AuthContext{Kind: "service", SubjectID: account.AccountID, SiteID: account.SiteID}
	_, err := PublishIdentityEvent(inner, eventOp, PublishIdentityEventInput{EventID: platform.NewIdentifier(), Event: IdentityChanged{
		AccountID: account.AccountID, SiteID: account.SiteID, PermissionRevision: 1, CredentialRevision: 1, Reason: "permissions", OccurredAt: account.UpdatedAt}})
	return err
}

// consumeInvite 原子地用掉一次注册邀请码；没有可用次数时拒绝。
func consumeInvite(ctx context.Context, tx *sql.Tx, code string) error {
	result, err := tx.ExecContext(ctx, `UPDATE registration_invites SET used=used+1 WHERE code_hash=? AND revoked_at IS NULL AND used<max_uses
        AND (expires_at IS NULL OR expires_at>?)`, hashCode(strings.TrimSpace(code)), nowRFC())
	if err != nil {
		return identityDBError()
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return platform.NewError(platform.CodeForbidden, false, "注册邀请码无效、已用完或已过期")
	}
	return nil
}

// RegisterUser 邮箱注册（不验证邮箱）：站主未开放公开注册时需要注册邀请码。
func RegisterUser(ctx context.Context, op OperationContext, input gen.RegisterUserInput) (RegisterFirstSuperadminResult, error) {
	ctx, op, err := anonymousIdentityOperation(ctx, op)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if input.Password != input.PasswordConfirmation || len(input.Password) < 8 {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeValidation, false, "两次口令不一致或不足8位")
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" || len([]rune(displayName)) > 100 {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeValidation, false, "昵称不合法")
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if !settings.EmailRegistration {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeForbidden, false, "未开放邮箱注册")
	}
	needInvite := settings.InviteRequired && !settings.RegistrationOpen
	if needInvite && (input.InviteCode == nil || strings.TrimSpace(*input.InviteCode) == "") {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeForbidden, false, "注册需要邀请码")
	}
	attempt, err := beginIdentityLogin(ctx, op, email, "login", "", "")
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	outcomeSent := false
	defer func() {
		if !outcomeSent {
			_ = finishIdentityLogin(ctx, op, attempt, "aborted")
		}
	}()
	hashed, err := HashPassword(ctx, op, HashPasswordInput{Password: input.Password})
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return RegisterFirstSuperadminResult{}, identityDBError()
	}
	defer tx.Rollback()
	if needInvite {
		if err := consumeInvite(ctx, tx, *input.InviteCode); err != nil {
			return RegisterFirstSuperadminResult{}, err
		}
	}
	account, _, err := createUserTx(ctx, tx, settings, newUser{Email: email, DisplayName: displayName, AccountName: email, PasswordHash: hashed.EncodedHash})
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	tokens, err := issueSessionTx(ctx, op, tx, account)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if err := publishUserCreated(ctx, op, tx, account); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	outcomeSent = true
	if err := finishIdentityLogin(ctx, op, attempt, "authenticated"); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return RegisterFirstSuperadminResult{}, identityDBError()
	}
	return RegisterFirstSuperadminResult{Account: account, Tokens: tokens, NextRoute: "/admin"}, nil
}

// ---- 第三方登录 ----

type oauthEndpoints struct {
	Authorize, Token, User, Scope string
	BasicAuth                     bool
}

var oauthProviders = map[string]oauthEndpoints{
	"github":  {Authorize: "https://github.com/login/oauth/authorize", Token: "https://github.com/login/oauth/access_token", User: "https://api.github.com/user", Scope: "read:user user:email"},
	"google":  {Authorize: "https://accounts.google.com/o/oauth2/v2/auth", Token: "https://oauth2.googleapis.com/token", User: "https://openidconnect.googleapis.com/v1/userinfo", Scope: "openid email profile"},
	"linuxdo": {Authorize: "https://connect.linux.do/oauth2/authorize", Token: "https://connect.linux.do/oauth2/token", User: "https://connect.linux.do/api/user", Scope: "user", BasicAuth: true},
}

var oauthClient = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func providerOf(name string) (oauthEndpoints, error) {
	ep, ok := oauthProviders[name]
	if !ok {
		return oauthEndpoints{}, platform.NewError(platform.CodeValidation, false, "不支持的第三方登录")
	}
	return ep, nil
}

// OauthStart 生成一次性 state（只存摘要，十分钟有效）并返回授权跳转地址。
func OauthStart(ctx context.Context, op OperationContext, input gen.OauthStartInput) (gen.OauthStartResult, error) {
	return startOAuth(ctx, input.RedirectUri, "")
}

// startOAuth 记下一次性 state 并给出授权地址；linkUser 非空表示已登录用户在绑定第三方账号。
func startOAuth(ctx context.Context, redirectURI, linkUser string) (gen.OauthStartResult, error) {
	provider := pathProvider(ctx)
	ep, err := providerOf(provider)
	if err != nil {
		return gen.OauthStartResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.OauthStartResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return gen.OauthStartResult{}, err
	}
	conf := settings.Providers[provider]
	if !conf.Enabled || conf.ClientID == "" {
		return gen.OauthStartResult{}, platform.NewError(platform.CodeForbidden, false, "该登录方式未开启")
	}
	if err := oauthCallbackAllowed(ctx, store.db, provider, redirectURI); err != nil {
		return gen.OauthStartResult{}, err
	}
	state := randomCode(24)
	expires := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339Nano)
	if _, err := store.db.ExecContext(ctx, `DELETE FROM oauth_states WHERE expires_at<?`, nowRFC()); err != nil {
		return gen.OauthStartResult{}, identityDBError()
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO oauth_states(state_hash,provider,redirect_uri,expires_at,link_user_id) VALUES(?,?,?,?,?)`, hashCode(state), provider, redirectURI, expires, linkUser); err != nil {
		return gen.OauthStartResult{}, identityDBError()
	}
	query := url.Values{"client_id": {conf.ClientID}, "redirect_uri": {redirectURI}, "response_type": {"code"}, "scope": {ep.Scope}, "state": {state}}
	return gen.OauthStartResult{AuthorizeUrl: ep.Authorize + "?" + query.Encode(), State: state}, nil
}

// oauthCallbackAllowed 回调只能回到本平台后台的 /admin/oauth/<服务商>（来源与 CSRF 认的一致）；服务商侧的回调登记之外，这里再挡掉外站地址。
func oauthCallbackAllowed(ctx context.Context, db sqlQuerier, provider, redirectURI string) error {
	fixed := ""
	if runtime, err := sessionRuntimeFromContext(ctx); err == nil {
		fixed = runtime.PublicOrigin
	}
	parsed, err := url.Parse(redirectURI)
	if err != nil || parsed.Path != "/admin/oauth/"+provider || parsed.RawQuery != "" || parsed.Fragment != "" || !originAllowed(ctx, db, fixed, parsed.Scheme+"://"+parsed.Host) {
		return platform.NewError(platform.CodeValidation, false, "回调地址必须是本平台后台的授权回调地址")
	}
	return nil
}

// consumeOAuthState 用掉一次性 state（删除成功才算用掉）；linkUser 必须和发起时一致（登录流程传空）。
func consumeOAuthState(ctx context.Context, db *sql.DB, provider, state, redirectURI, linkUser string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return identityDBError()
	}
	defer tx.Rollback()
	var stored string
	err = tx.QueryRowContext(ctx, `SELECT redirect_uri FROM oauth_states WHERE state_hash=? AND provider=? AND expires_at>? AND link_user_id=?`, hashCode(state), provider, nowRFC(), linkUser).Scan(&stored)
	if err == nil {
		_, err = tx.ExecContext(ctx, `DELETE FROM oauth_states WHERE state_hash=?`, hashCode(state))
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil || stored != redirectURI {
		return platform.NewError(platform.CodeUnauthenticated, false, "授权状态已失效，请重新操作")
	}
	return nil
}

type oauthProfile struct{ Subject, Email, Name, Avatar string }

// fetchProfile 用授权码换 access_token 再取用户资料；地址是固定的服务商端点，不跟随重定向。
func fetchProfile(ctx context.Context, provider string, ep oauthEndpoints, conf providerConf, secret, code, redirectURI string) (oauthProfile, error) {
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI}}
	if !ep.BasicAuth {
		form.Set("client_id", conf.ClientID)
		form.Set("client_secret", secret)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ep.Token, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if ep.BasicAuth {
		req.SetBasicAuth(conf.ClientID, secret)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := doJSON(req, &token); err != nil || token.AccessToken == "" {
		return oauthProfile{}, platform.NewError(platform.CodeUnauthenticated, false, "第三方登录授权失败")
	}
	userReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ep.User, nil)
	userReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	userReq.Header.Set("Accept", "application/json")
	var raw map[string]any
	if err := doJSON(userReq, &raw); err != nil {
		return oauthProfile{}, platform.NewError(platform.CodeUnauthenticated, false, "读取第三方账号资料失败")
	}
	profile := oauthProfile{}
	str := func(keys ...string) string {
		for _, k := range keys {
			switch v := raw[k].(type) {
			case string:
				if v != "" {
					return v
				}
			case float64:
				return fmt.Sprintf("%.0f", v)
			}
		}
		return ""
	}
	switch provider {
	case "github":
		profile = oauthProfile{Subject: str("id"), Email: str("email"), Name: str("name", "login"), Avatar: str("avatar_url")}
		if profile.Email == "" {
			profile.Email = githubPrimaryEmail(ctx, token.AccessToken)
		}
	case "google":
		profile = oauthProfile{Subject: str("sub"), Name: str("name", "email"), Avatar: str("picture")}
		if verified, _ := raw["email_verified"].(bool); verified {
			profile.Email = str("email")
		}
	case "linuxdo":
		profile = oauthProfile{Subject: str("id"), Email: str("email"), Name: str("name", "username"), Avatar: str("avatar_url")}
	}
	if profile.Subject == "" {
		return oauthProfile{}, platform.NewError(platform.CodeUnauthenticated, false, "第三方账号缺少标识")
	}
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))
	return profile, nil
}

func githubPrimaryEmail(ctx context.Context, accessToken string) string {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if doJSON(req, &emails) != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	return ""
}

func doJSON(req *http.Request, target any) error {
	response, err := oauthClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("上游返回非 200")
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target)
}

// OauthLogin 用回调里的 code 完成登录：已绑定身份直接登录，否则自动建号（不需要注册邀请码，也不按邮箱合并）。
func OauthLogin(ctx context.Context, op OperationContext, input gen.OauthLoginInput) (RegisterFirstSuperadminResult, error) {
	ctx, op, err := anonymousIdentityOperation(ctx, op)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	provider := pathProvider(ctx)
	ep, err := providerOf(provider)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	conf := settings.Providers[provider]
	if !conf.Enabled || conf.ClientID == "" {
		return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeForbidden, false, "该登录方式未开启")
	}
	attempt, err := beginIdentityLogin(ctx, op, "oauth:"+provider, "login", "", "")
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	outcomeSent := false
	defer func() {
		if !outcomeSent {
			_ = finishIdentityLogin(ctx, op, attempt, "failed")
		}
	}()
	if err := consumeOAuthState(ctx, store.db, provider, input.State, input.RedirectUri, ""); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	secret, err := loadProviderSecret(ctx, store.db, provider)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	profile, err := fetchProfile(ctx, provider, ep, conf, secret, input.Code, input.RedirectUri)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return RegisterFirstSuperadminResult{}, identityDBError()
	}
	defer tx.Rollback()
	var userID string
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM user_identities WHERE provider=? AND subject=?`, provider, profile.Subject).Scan(&userID)
	var account Account
	switch {
	case err == nil:
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT a.account_id,a.site_id,a.account_name,a.status,a.revision,a.permission_revision,a.created_at,a.updated_at
            FROM users u JOIN accounts a ON a.account_id=u.account_id WHERE u.user_id=?`, userID).Scan(&account.AccountID, &account.SiteID, &account.AccountName,
			&status, &account.Revision, &account.PermissionRevision, &account.CreatedAt, &account.UpdatedAt); err != nil {
			return RegisterFirstSuperadminResult{}, identityDBError()
		}
		account.Status, account.Permissions = AccountStatus(status), []string{}
		if account.Status != AccountStatusActive {
			return RegisterFirstSuperadminResult{}, platform.NewError(platform.CodeForbidden, false, "账号已停用")
		}
	case errors.Is(err, sql.ErrNoRows):
		email := profile.Email
		var taken int
		if email != "" {
			_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email=?`, email).Scan(&taken)
			if taken > 0 {
				email = "" // 不按邮箱合并账号：邮箱已被别的用户占用时，新用户不记邮箱
			}
		}
		name := strings.TrimSpace(profile.Name)
		if name == "" {
			name = provider + " 用户"
		}
		account, userID, err = createUserTx(ctx, tx, settings, newUser{Email: email, DisplayName: truncateRunes(name, 100), Avatar: profile.Avatar,
			AccountName: provider + ":" + profile.Subject, PasswordHash: "!"})
		if err != nil {
			return RegisterFirstSuperadminResult{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_identities(provider,subject,user_id,email,created_at) VALUES(?,?,?,?,?)`, provider, profile.Subject, userID, profile.Email, platform.Now()); err != nil {
			return RegisterFirstSuperadminResult{}, identityDBError()
		}
		if err := publishUserCreated(ctx, op, tx, account); err != nil {
			return RegisterFirstSuperadminResult{}, err
		}
	default:
		return RegisterFirstSuperadminResult{}, identityDBError()
	}
	tokens, err := issueSessionTx(ctx, op, tx, account)
	if err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	outcomeSent = true
	if err := finishIdentityLogin(ctx, op, attempt, "authenticated"); err != nil {
		return RegisterFirstSuperadminResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return RegisterFirstSuperadminResult{}, identityDBError()
	}
	return RegisterFirstSuperadminResult{Account: account, Tokens: tokens, NextRoute: "/admin"}, nil
}

// ---- 绑定第三方账号 ----

// myUser 是当前登录的平台用户。
func myUser(ctx context.Context, op OperationContext) (*Store, string, error) {
	if err := requireAdminActor(op); err != nil {
		return nil, "", err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return nil, "", platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	userID, err := userOfAccount(ctx, store.db, op.Actor.SubjectID)
	return store, userID, err
}

func myIdentities(ctx context.Context, db sqlQuerier, userID string) (gen.MyIdentitiesResult, error) {
	result := gen.MyIdentitiesResult{Identities: []gen.LinkedIdentity{}}
	var hash string
	if err := db.QueryRowContext(ctx, `SELECT a.account_name, COALESCE(c.password_hash,'!') FROM users u JOIN accounts a ON a.account_id=u.account_id
        LEFT JOIN credentials c ON c.account_id=a.account_id WHERE u.user_id=?`, userID).Scan(&result.LoginName, &hash); err != nil {
		return result, identityDBError()
	}
	result.HasPassword = hash != "!"
	rows, err := db.QueryContext(ctx, `SELECT provider,email,created_at FROM user_identities WHERE user_id=? ORDER BY created_at`, userID)
	if err != nil {
		return result, identityDBError()
	}
	defer rows.Close()
	for rows.Next() {
		var item gen.LinkedIdentity
		var provider, created string
		if err := rows.Scan(&provider, &item.Email, &created); err != nil {
			return result, identityDBError()
		}
		item.Provider, item.CreatedAt = gen.LinkedIdentityProvider(provider), parseTS(created)
		result.Identities = append(result.Identities, item)
	}
	return result, nil
}

// ListMyIdentities 列出当前用户的登录方式。
func ListMyIdentities(ctx context.Context, op OperationContext, _ struct{}) (gen.MyIdentitiesResult, error) {
	store, userID, err := myUser(ctx, op)
	if err != nil {
		return gen.MyIdentitiesResult{}, err
	}
	return myIdentities(ctx, store.db, userID)
}

// StartIdentityLink 已登录用户开始绑定第三方账号：授权回来后由 LinkIdentity 完成。
func StartIdentityLink(ctx context.Context, op OperationContext, input gen.OauthStartInput) (gen.OauthStartResult, error) {
	_, userID, err := myUser(ctx, op)
	if err != nil {
		return gen.OauthStartResult{}, err
	}
	return startOAuth(ctx, input.RedirectUri, userID)
}

// LinkIdentity 把授权回来的第三方账号绑到当前用户；这个第三方账号已经属于别的用户时拒绝（不合并账号）。
func LinkIdentity(ctx context.Context, op OperationContext, input gen.OauthLoginInput) (gen.MyIdentitiesResult, error) {
	store, userID, err := myUser(ctx, op)
	if err != nil {
		return gen.MyIdentitiesResult{}, err
	}
	provider := pathProvider(ctx)
	ep, err := providerOf(provider)
	if err != nil {
		return gen.MyIdentitiesResult{}, err
	}
	if err := consumeOAuthState(ctx, store.db, provider, input.State, input.RedirectUri, userID); err != nil {
		return gen.MyIdentitiesResult{}, err
	}
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return gen.MyIdentitiesResult{}, err
	}
	secret, err := loadProviderSecret(ctx, store.db, provider)
	if err != nil {
		return gen.MyIdentitiesResult{}, err
	}
	profile, err := fetchProfile(ctx, provider, ep, settings.Providers[provider], secret, input.Code, input.RedirectUri)
	if err != nil {
		return gen.MyIdentitiesResult{}, err
	}
	var owner string
	err = store.db.QueryRowContext(ctx, `SELECT user_id FROM user_identities WHERE provider=? AND subject=?`, provider, profile.Subject).Scan(&owner)
	switch {
	case err == nil && owner == userID:
	case err == nil:
		return gen.MyIdentitiesResult{}, platform.NewError(platform.CodeConflict, false, "这个第三方账号已经绑定在别的用户上")
	case errors.Is(err, sql.ErrNoRows):
		if _, err := store.db.ExecContext(ctx, `INSERT INTO user_identities(provider,subject,user_id,email,created_at) VALUES(?,?,?,?,?)`, provider, profile.Subject, userID, profile.Email, platform.Now()); err != nil {
			return gen.MyIdentitiesResult{}, identityDBError()
		}
	default:
		return gen.MyIdentitiesResult{}, identityDBError()
	}
	return myIdentities(ctx, store.db, userID)
}

// UnlinkIdentity 解绑第三方账号；解绑后至少要留一种登录方式（另一个第三方账号或口令）。
func UnlinkIdentity(ctx context.Context, op OperationContext, _ struct{}) (gen.MyIdentitiesResult, error) {
	store, userID, err := myUser(ctx, op)
	if err != nil {
		return gen.MyIdentitiesResult{}, err
	}
	provider := pathProvider(ctx)
	current, err := myIdentities(ctx, store.db, userID)
	if err != nil {
		return current, err
	}
	others := 0
	for _, item := range current.Identities {
		if string(item.Provider) != provider {
			others++
		}
	}
	if others == 0 && !current.HasPassword {
		return current, platform.NewError(platform.CodeConflict, false, "这是唯一的登录方式，先设口令或绑定别的账号再解绑")
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM user_identities WHERE user_id=? AND provider=?`, userID, provider); err != nil {
		return current, identityDBError()
	}
	return myIdentities(ctx, store.db, userID)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

type pathProviderKey struct{}

// WithPathProvider 由传输层把路径里的 {provider} 放进上下文。
func WithPathProvider(ctx context.Context, provider string) context.Context {
	return context.WithValue(ctx, pathProviderKey{}, provider)
}

func pathProvider(ctx context.Context) string {
	provider, _ := ctx.Value(pathProviderKey{}).(string)
	return provider
}

var _ = base64.StdEncoding
