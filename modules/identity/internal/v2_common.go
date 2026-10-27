package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/secretbox"
)

// 多用户平台的共用定义：设置、角色权限、通用查询。

// homeSitePrefix 是用户“家账户”的站点标识前缀；家账户只用于仪表盘类操作（我的站点、创建站点、个人设置）。
const homeSitePrefix = "home:"

func homeSiteID(userID string) string { return homeSitePrefix + userID }

func isHomeSite(site string) bool { return strings.HasPrefix(site, homeSitePrefix) }

type secretBoxKey struct{}

// WithSecretBox 注入主密钥加密盒（保存第三方登录密钥用）。
func WithSecretBox(ctx context.Context, box *secretbox.Box) context.Context {
	return context.WithValue(ctx, secretBoxKey{}, box)
}

func secretBoxFrom(ctx context.Context) (*secretbox.Box, error) {
	box, _ := ctx.Value(secretBoxKey{}).(*secretbox.Box)
	if box == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "主密钥未配置，不能保存或读取密钥")
	}
	return box, nil
}

// platformSettings 是持久化的平台设置（密钥不在其中）。
type platformSettings struct {
	RegistrationOpen      bool                    `json:"registrationOpen"`
	EmailRegistration     bool                    `json:"emailRegistration"`
	InviteRequired        bool                    `json:"inviteRequired"`
	DefaultMaxSites       int                     `json:"defaultMaxSites"`
	DefaultStorageBytes   int64                   `json:"defaultStorageBytes"`
	DefaultVisitorQaDaily int                     `json:"defaultVisitorQaDaily"`
	PlatformHost          string                  `json:"platformHost"`
	SitesDomain           string                  `json:"sitesDomain"`
	Providers             map[string]providerConf `json:"providers"`
}

type providerConf struct {
	Enabled  bool   `json:"enabled"`
	ClientID string `json:"clientId"`
}

var allProviders = []string{"github", "google", "linuxdo"}

var providerLabels = map[string]string{"github": "GitHub", "google": "Google", "linuxdo": "Linux DO"}

func defaultSettings() platformSettings {
	return platformSettings{EmailRegistration: true, InviteRequired: true, DefaultMaxSites: 5, DefaultStorageBytes: 1 << 30, DefaultVisitorQaDaily: 50, Providers: map[string]providerConf{}}
}

type sqlQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func loadSettings(ctx context.Context, q sqlQuerier) (platformSettings, error) {
	settings := defaultSettings()
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value_json FROM platform_settings WHERE key='settings'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	if err != nil {
		return settings, identityDBError()
	}
	if json.Unmarshal([]byte(raw), &settings) != nil {
		return defaultSettings(), platform.NewError(platform.CodeIntegrityError, false, "平台设置记录损坏")
	}
	if settings.Providers == nil {
		settings.Providers = map[string]providerConf{}
	}
	return settings, nil
}

func saveSettings(ctx context.Context, q sqlQuerier, settings platformSettings) error {
	raw, _ := json.Marshal(settings)
	_, err := q.ExecContext(ctx, `INSERT INTO platform_settings(key,value_json,updated_at) VALUES('settings',?,?)
        ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json, updated_at=excluded.updated_at`, string(raw), platform.Now())
	if err != nil {
		return identityDBError()
	}
	return nil
}

func providerSecretName(provider string) string { return "oauth:" + provider }

func hasProviderSecret(ctx context.Context, q sqlQuerier, provider string) bool {
	return hasSecret(ctx, q, providerSecretName(provider))
}

func saveProviderSecret(ctx context.Context, q sqlQuerier, provider, secret string) error {
	return saveSecret(ctx, q, providerSecretName(provider), secret)
}

func loadProviderSecret(ctx context.Context, q sqlQuerier, provider string) (string, error) {
	plain, found, err := loadSecret(ctx, q, providerSecretName(provider))
	if err == nil && !found {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "第三方登录密钥未配置")
	}
	return plain, err
}

// 平台密钥（第三方登录凭证、DNS 令牌）用主密钥加密后存在 platform_secrets，名字同时作附加数据，换名解不开。

func hasSecret(ctx context.Context, q sqlQuerier, name string) bool {
	var n int
	_ = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_secrets WHERE name=?`, name).Scan(&n)
	return n == 1
}

func saveSecret(ctx context.Context, q sqlQuerier, name, secret string) error {
	box, err := secretBoxFrom(ctx)
	if err != nil {
		return err
	}
	nonce, ciphertext, err := box.Seal([]byte(secret), name)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "密钥加密失败")
	}
	_, err = q.ExecContext(ctx, `INSERT INTO platform_secrets(name,nonce,ciphertext,updated_at) VALUES(?,?,?,?)
        ON CONFLICT(name) DO UPDATE SET nonce=excluded.nonce, ciphertext=excluded.ciphertext, updated_at=excluded.updated_at`,
		name, nonce, ciphertext, platform.Now())
	if err != nil {
		return identityDBError()
	}
	return nil
}

func deleteSecret(ctx context.Context, q sqlQuerier, name string) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM platform_secrets WHERE name=?`, name); err != nil {
		return identityDBError()
	}
	return nil
}

// loadSecret 解密读出平台密钥；没存过时 found 为 false。
func loadSecret(ctx context.Context, q sqlQuerier, name string) (string, bool, error) {
	box, err := secretBoxFrom(ctx)
	if err != nil {
		return "", false, err
	}
	var nonce, ciphertext []byte
	err = q.QueryRowContext(ctx, `SELECT nonce,ciphertext FROM platform_secrets WHERE name=?`, name).Scan(&nonce, &ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, identityDBError()
	}
	plain, err := box.Open(nonce, ciphertext, name)
	if err != nil {
		return "", false, platform.NewError(platform.CodeIntegrityError, false, "平台密钥无法解密")
	}
	return string(plain), true, nil
}

// 站点内角色：owner 拥有全部十项功能并额外管理成员、域名、审计与备份；editor 能写能发布；viewer 只读。
var (
	ownerFunctions  = []FunctionPermission{PermissionEditDocuments, PermissionPublishDocuments, PermissionManageTranslations, PermissionManageLanguages, PermissionManageResources, PermissionManageQA, PermissionManageAI, PermissionManageMemory, PermissionManageSite, PermissionViewStats}
	editorFunctions = []FunctionPermission{PermissionEditDocuments, PermissionPublishDocuments, PermissionManageTranslations, PermissionManageResources, PermissionViewStats}
	viewerGrants    = []string{"portal.use", "account.self", "site.read", "content.read", "content.preview", "media.read", "stats.read"}
	ownerExtra      = []string{"account.manage", "audit.read", "backup.manage"}
	// 平台级权限：只给站主，站点所有者拿不到。
	platformOnlyGrants = []string{"system.manage", "superadmin", "model.manage"}
)

// roleGrants 把站点角色映射成技术权限（浏览器不能自选）。
func roleGrants(role string) ([]string, error) {
	var functions []FunctionPermission
	var extra []string
	switch role {
	case "owner":
		functions, extra = ownerFunctions, ownerExtra
	case "editor":
		functions = editorFunctions
	case "viewer":
		return append([]string{}, viewerGrants...), nil
	default:
		return nil, platform.NewError(platform.CodeForbidden, false, "不是该站点成员")
	}
	mapped, err := MapFunctionGrants(MapFunctionGrantsInput{Permissions: functions})
	if err != nil {
		return nil, err
	}
	grants := append(append([]string{}, mapped.Grants...), extra...)
	seen := map[string]bool{}
	out := grants[:0]
	for _, g := range grants {
		if !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	return out, nil
}

var emailPattern = regexp.MustCompile(`^[^@\s]{1,64}@[^@\s]+\.[^@\s]{2,}$`)

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) > 254 || !emailPattern.MatchString(email) {
		return "", platform.NewError(platform.CodeValidation, false, "邮箱格式不合法")
	}
	return email, nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func randomCode(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw)
}

// userOfAccount 取家账户对应的用户；站主之外没有用户记录的账户（旧子账号）返回空。
func userOfAccount(ctx context.Context, q sqlQuerier, accountID string) (string, error) {
	var userID string
	err := q.QueryRowContext(ctx, `SELECT user_id FROM users WHERE account_id=?`, accountID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", platform.NewError(platform.CodeForbidden, false, "当前账户不是平台用户")
	}
	if err != nil {
		return "", identityDBError()
	}
	return userID, nil
}

func nowRFC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func bumpPermissionRevision(ctx context.Context, q sqlQuerier, userID string) error {
	_, err := q.ExecContext(ctx, `UPDATE accounts SET permission_revision=permission_revision+1, updated_at=? WHERE account_id=(SELECT account_id FROM users WHERE user_id=?)`, platform.Now(), userID)
	if err != nil {
		return identityDBError()
	}
	return nil
}

// parseTS 把库里存的 RFC3339 文本转成 time.Time（合同里的 Timestamp）。
func parseTS(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}
