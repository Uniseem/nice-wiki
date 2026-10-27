package identity

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// 平台子域名的泛域名证书：站主在后台填 Cloudflare API 令牌后，本域经 Caddy 管理接口（NW_CADDY_ADMIN_URL，只在内部网络）
// 加一条 DNS 验证的签发策略并让 Caddy 管理 *.<站点域名> 证书；令牌只加密存在库里和 Caddy 的内存配置里，不写配置文件。
// Caddy 重启会回到 Caddyfile，所以后台每分钟检查一次，缺了就补；令牌换了就替换策略。没填令牌时子域名照旧按需签证书。

const dnsSecretName = "dns:cloudflare"

var caddyPushed struct {
	sync.Mutex
	token string
}

// syncCaddyWildcard 让 Caddy 有泛域名证书的签发策略；Caddy 不可达只记日志，不影响其余后台工作。
func syncCaddyWildcard(ctx context.Context, db *sql.DB) {
	admin := os.Getenv("NW_CADDY_ADMIN_URL")
	if admin == "" {
		return
	}
	settings, err := loadSettings(ctx, db)
	if err != nil || settings.SitesDomain == "" {
		return
	}
	token, found, err := loadSecret(ctx, db, dnsSecretName)
	if err != nil || !found {
		return
	}
	if err := pushCaddyWildcard(ctx, admin, "*."+settings.SitesDomain, token); err != nil {
		fmt.Fprintf(os.Stderr, "下发泛域名证书策略失败 err=%v\n", err)
	}
}

func pushCaddyWildcard(ctx context.Context, admin, wildcard, token string) error {
	caddyPushed.Lock()
	defer caddyPushed.Unlock()
	var policies []struct {
		Subjects []string `json:"subjects"`
	}
	if err := caddyJSON(ctx, http.MethodGet, admin+"/config/apps/tls/automation/policies", nil, &policies); err != nil {
		return err
	}
	policy := map[string]any{"subjects": []string{wildcard}, "issuers": []any{map[string]any{
		"module": "acme", "challenges": map[string]any{"dns": map[string]any{"provider": map[string]any{"name": "cloudflare", "api_token": token}}},
	}}}
	index := -1
	for i, existing := range policies {
		for _, subject := range existing.Subjects {
			if subject == wildcard {
				index = i
			}
		}
	}
	switch {
	case index < 0:
		// 有主体的策略要排在 Caddyfile 那条不限主体的按需策略前面，否则匹配不到。
		if err := caddyJSON(ctx, http.MethodPut, admin+"/config/apps/tls/automation/policies/0", policy, nil); err != nil {
			return err
		}
	case caddyPushed.token != token:
		if err := caddyJSON(ctx, http.MethodPatch, admin+"/config/apps/tls/automation/policies/"+strconv.Itoa(index), policy, nil); err != nil {
			return err
		}
	}
	certificates := map[string]any{}
	if err := caddyJSON(ctx, http.MethodGet, admin+"/config/apps/tls/certificates", nil, &certificates); err != nil {
		return err
	}
	if certificates == nil {
		certificates = map[string]any{}
	}
	automate, _ := certificates["automate"].([]any)
	for _, subject := range automate {
		if subject == wildcard {
			caddyPushed.token = token
			return nil
		}
	}
	certificates["automate"] = append(automate, wildcard)
	if err := caddyJSON(ctx, http.MethodPost, admin+"/config/apps/tls/certificates", certificates, nil); err != nil {
		return err
	}
	caddyPushed.token = token
	return nil
}

// caddyJSON 调 Caddy 管理接口；GET 时路径不存在（Caddy 返回 null）留空目标。
func caddyJSON(ctx context.Context, method, url string, body, target any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 300 {
		return fmt.Errorf("caddy %s %s: %d", method, url, response.StatusCode)
	}
	if target != nil && len(bytes.TrimSpace(raw)) > 0 && string(bytes.TrimSpace(raw)) != "null" {
		return json.Unmarshal(raw, target)
	}
	return nil
}
