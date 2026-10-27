package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/identity/internal/gen"
)

// 海外网盘备份：站主在“平台设置”里为每个网盘填一组 OAuth 应用凭据；站点所有者在备份目标上点“授权”时，
// 身份域用授权码换出长期授权（refresh token）并交给内容域加密保存，身份域不留存令牌；
// 备份时内容域再来这里换短期访问令牌（应用密钥只在身份域）。

var driveProviders = []string{"gdrive", "onedrive", "dropbox"}

var driveLabels = map[string]string{"gdrive": "Google Drive", "onedrive": "OneDrive", "dropbox": "Dropbox"}

type driveEndpoints struct {
	Authorize, Token, Scope string
	Params                  url.Values
}

// 地址固定在代码里，不接受配置，也不跟随重定向。
var driveProviderEndpoints = map[string]driveEndpoints{
	"gdrive":   {Authorize: "https://accounts.google.com/o/oauth2/v2/auth", Token: "https://oauth2.googleapis.com/token", Scope: "https://www.googleapis.com/auth/drive.file", Params: url.Values{"access_type": {"offline"}, "prompt": {"consent"}}},
	"onedrive": {Authorize: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize", Token: "https://login.microsoftonline.com/common/oauth2/v2.0/token", Scope: "Files.ReadWrite.AppFolder offline_access"},
	"dropbox":  {Authorize: "https://www.dropbox.com/oauth2/authorize", Token: "https://api.dropboxapi.com/oauth2/token", Params: url.Values{"token_access_type": {"offline"}}},
}

// configurableProviders 是平台设置里可以填应用凭据的全部提供方：登录用的和网盘用的。
func configurableProviders() []string {
	return append(append([]string{}, allProviders...), driveProviders...)
}

// driveState 是授权发起时记在 state 上的上下文，回调时核对。
type driveState struct {
	Site   string `json:"site"`
	Target string `json:"target"`
	Actor  string `json:"actor"`
}

func driveConf(ctx context.Context, store *Store, provider string) (providerConf, error) {
	settings, err := loadSettings(ctx, store.db)
	if err != nil {
		return providerConf{}, err
	}
	conf := settings.Providers[provider]
	if !conf.Enabled || conf.ClientID == "" || !hasProviderSecret(ctx, store.db, provider) {
		return providerConf{}, platform.NewError(platform.CodeForbidden, false, "站主还没有开启 %s 备份", driveLabels[provider])
	}
	return conf, nil
}

type StartDriveAuthCmd struct {
	SiteID string
	Body   gen.StartDriveAuthInput
}

// StartDriveAuth 生成一次性 state（只存摘要，十分钟有效）并返回网盘的授权跳转地址。
func StartDriveAuth(ctx context.Context, op OperationContext, cmd StartDriveAuthCmd) (gen.StartDriveAuthResult, error) {
	if err := requireSiteScope(op, cmd.SiteID); err != nil {
		return gen.StartDriveAuthResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.StartDriveAuthResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	provider := string(cmd.Body.Provider)
	conf, err := driveConf(ctx, store, provider)
	if err != nil {
		return gen.StartDriveAuthResult{}, err
	}
	ep := driveProviderEndpoints[provider]
	state := randomCode(24)
	stateJSON, _ := json.Marshal(driveState{Site: cmd.SiteID, Target: cmd.Body.TargetId, Actor: op.Actor.SubjectID})
	if _, err := store.db.ExecContext(ctx, `DELETE FROM oauth_states WHERE expires_at<?`, nowRFC()); err != nil {
		return gen.StartDriveAuthResult{}, identityDBError()
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO oauth_states(state_hash,provider,redirect_uri,expires_at,context) VALUES(?,?,?,?,?)`,
		hashCode(state), provider, cmd.Body.RedirectUri, time.Now().UTC().Add(10*time.Minute).Format(time.RFC3339Nano), string(stateJSON)); err != nil {
		return gen.StartDriveAuthResult{}, identityDBError()
	}
	query := url.Values{"client_id": {conf.ClientID}, "redirect_uri": {cmd.Body.RedirectUri}, "response_type": {"code"}, "state": {state}}
	if ep.Scope != "" {
		query.Set("scope", ep.Scope)
	}
	for k, v := range ep.Params {
		query[k] = v
	}
	return gen.StartDriveAuthResult{AuthorizeUrl: ep.Authorize + "?" + query.Encode()}, nil
}

type driveTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// driveTokenCall 向网盘的令牌端点提交一次表单（授权码换取或刷新），自动带上应用凭据。
func driveTokenCall(ctx context.Context, provider string, conf providerConf, secret string, form url.Values) (driveTokenResponse, error) {
	form.Set("client_id", conf.ClientID)
	form.Set("client_secret", secret)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, driveProviderEndpoints[provider].Token, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	var token driveTokenResponse
	if err := doJSON(req, &token); err != nil || token.AccessToken == "" {
		return driveTokenResponse{}, platform.NewError(platform.CodeValidation, false, "%s 授权失败，请重新授权", driveLabels[provider])
	}
	return token, nil
}

type CompleteDriveAuthCmd struct {
	SiteID string
	Body   gen.CompleteDriveAuthInput
}

// CompleteDriveAuth 核对 state，用授权码换长期授权，再交给内容域保存到那个备份目标。
func CompleteDriveAuth(ctx context.Context, op OperationContext, cmd CompleteDriveAuthCmd) (gen.OkResult, error) {
	if err := requireSiteScope(op, cmd.SiteID); err != nil {
		return gen.OkResult{}, err
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.OkResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	gateway, ok := ctx.Value(contentGatewayKey{}).(*ContentGateway)
	if !ok || gateway == nil {
		return gen.OkResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少内容服务地址")
	}
	// state 单次有效：删除成功才算用掉。
	var provider, redirect, raw string
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return gen.OkResult{}, identityDBError()
	}
	err = tx.QueryRowContext(ctx, `SELECT provider,redirect_uri,context FROM oauth_states WHERE state_hash=? AND expires_at>?`, hashCode(cmd.Body.State), nowRFC()).Scan(&provider, &redirect, &raw)
	if err == nil {
		_, err = tx.ExecContext(ctx, `DELETE FROM oauth_states WHERE state_hash=?`, hashCode(cmd.Body.State))
	}
	if err == nil {
		err = tx.Commit()
	} else {
		tx.Rollback()
	}
	var state driveState
	if err != nil || json.Unmarshal([]byte(raw), &state) != nil || state.Site != cmd.SiteID || state.Actor != op.Actor.SubjectID || redirect != cmd.Body.RedirectUri {
		return gen.OkResult{}, platform.NewError(platform.CodeValidation, false, "授权状态已失效，请重新授权")
	}
	if _, known := driveProviderEndpoints[provider]; !known {
		return gen.OkResult{}, platform.NewError(platform.CodeValidation, false, "授权状态已失效，请重新授权")
	}
	conf, err := driveConf(ctx, store, provider)
	if err != nil {
		return gen.OkResult{}, err
	}
	secret, err := loadProviderSecret(ctx, store.db, provider)
	if err != nil {
		return gen.OkResult{}, err
	}
	token, err := driveTokenCall(ctx, provider, conf, secret, url.Values{"grant_type": {"authorization_code"}, "code": {cmd.Body.Code}, "redirect_uri": {cmd.Body.RedirectUri}})
	if err != nil {
		return gen.OkResult{}, err
	}
	if token.RefreshToken == "" {
		return gen.OkResult{}, platform.NewError(platform.CodeValidation, false, "%s 没有给出长期授权，请在授权页勾选保持访问后重试", driveLabels[provider])
	}
	if err := gateway.StoreToken(ctx, op, cmd.SiteID, state.Target, provider, token.RefreshToken); err != nil {
		return gen.OkResult{}, err
	}
	return gen.OkResult{Ok: true}, nil
}

// RefreshDriveToken 给内容域：用保存的长期授权换短期访问令牌；服务商轮换了长期授权时一并带回。
func RefreshDriveToken(ctx context.Context, op OperationContext, input gen.RefreshDriveTokenInput) (gen.RefreshDriveTokenResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return gen.RefreshDriveTokenResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份存储尚未装配")
	}
	provider := string(input.Provider)
	conf, err := driveConf(ctx, store, provider)
	if err != nil {
		return gen.RefreshDriveTokenResult{}, err
	}
	secret, err := loadProviderSecret(ctx, store.db, provider)
	if err != nil {
		return gen.RefreshDriveTokenResult{}, err
	}
	token, err := driveTokenCall(ctx, provider, conf, secret, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {input.RefreshToken}})
	if err != nil {
		return gen.RefreshDriveTokenResult{}, err
	}
	result := gen.RefreshDriveTokenResult{AccessToken: token.AccessToken}
	if token.RefreshToken != "" && token.RefreshToken != input.RefreshToken {
		result.RefreshToken = &token.RefreshToken
	}
	return result, nil
}

// ---- 身份域调内容域 ----

type contentGatewayKey struct{}

// WithContentGateway 由传输层放进上下文。
func WithContentGateway(ctx context.Context, gateway *ContentGateway) context.Context {
	return context.WithValue(ctx, contentGatewayKey{}, gateway)
}

// ContentGateway 是身份域调用内容域的最小客户端：把网盘长期授权交给备份目标保存。
type ContentGateway struct {
	base   string
	client *http.Client
}

func NewContentGateway(baseURL, issuer string, key platform.SigningKey) *ContentGateway {
	return &ContentGateway{base: strings.TrimRight(baseURL, "/"), client: &http.Client{
		Timeout:       30 * time.Second,
		Transport:     serviceauth.SigningTransport(issuer, "content", key, nil),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (g *ContentGateway) StoreToken(ctx context.Context, op OperationContext, site, targetID, provider, refreshToken string) error {
	id := platform.NewIdentifier()
	deadline := platform.Timestamp(time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano))
	svc := OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: "identity", Deadline: deadline, DataEpoch: op.DataEpoch,
		Actor: platform.AuthContext{Kind: "service", SubjectID: "identity", SiteID: site, Grants: []string{}, ExpiresAt: deadline}}
	body, _ := json.Marshal(map[string]string{"provider": provider, "refreshToken": refreshToken})
	req, err := http.NewRequestWithContext(platform.WithPortalProof(platform.WithOperationContext(ctx, svc), ""), http.MethodPut, g.base+"/v1/backup-targets/"+url.PathEscape(targetID)+"/drive-token", bytes.NewReader(body))
	if err != nil {
		return platform.NewError(platform.CodeValidation, false, "内容服务请求构造失败")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "内容服务不可达")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		var problem struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(raw, &problem)
		if problem.Detail == "" {
			problem.Detail = "备份目标不存在或类型不符"
		}
		return platform.NewError(platform.CodeValidation, false, "%s", problem.Detail)
	}
	return nil
}
