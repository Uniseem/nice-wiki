package transport

import (
	"context"
	"errors"
	"html"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// HostSite 是域名解析出的站点信息（来自 identity:resolveHost）。
type HostSite struct {
	SiteID       platform.Identifier
	Status       string
	Visibility   string
	PrimaryHost  string
	RedirectHost string
	PlatformHost string
	// PasswordEnabled：私有站点设了访问密码，访客可以在站点域名上输入密码进入。
	PasswordEnabled bool
}

// PrivateCredential 是在站点域名上换取私有站点访问的凭据：成员一次性码、访问密码或分享链接令牌，只填一个。
type PrivateCredential struct {
	Code       string `json:"code,omitempty"`
	Password   string `json:"password,omitempty"`
	ShareToken string `json:"shareToken,omitempty"`
}

type hostEntry struct {
	site    HostSite
	found   bool
	expires time.Time
}

// SetHostResolver 登记域名→站点解析函数；没有登记时所有请求落到默认站点（单站点用法）。
func (a *Application) SetHostResolver(resolve func(context.Context, string) (HostSite, bool, error)) {
	a.resolveHost = resolve
}

func requestHost(r *http.Request) string {
	host := strings.ToLower(r.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimSuffix(host, ".")
}

// hostSite 按访问域名取站点，结果短时间缓存（正向 20 秒，未命中 5 秒）。
func (a *Application) hostSite(r *http.Request) (HostSite, bool) {
	if a.resolveHost == nil {
		return HostSite{}, false
	}
	host := requestHost(r)
	a.hostMu.Lock()
	if entry, ok := a.hostCache[host]; ok && time.Now().Before(entry.expires) {
		a.hostMu.Unlock()
		return entry.site, entry.found
	}
	a.hostMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	site, found, err := a.resolveHost(ctx, host)
	if err != nil {
		return HostSite{}, false
	}
	ttl := 5 * time.Second
	if found {
		ttl = 20 * time.Second
	}
	a.hostMu.Lock()
	if a.hostCache == nil {
		a.hostCache = map[string]hostEntry{}
	}
	if len(a.hostCache) > 10000 {
		a.hostCache = map[string]hostEntry{}
	}
	a.hostCache[host] = hostEntry{site: site, found: found, expires: time.Now().Add(ttl)}
	a.hostMu.Unlock()
	return site, found
}

// forgetHosts 清空域名解析缓存。
func (a *Application) forgetHosts() {
	a.hostMu.Lock()
	a.hostCache = nil
	a.hostMu.Unlock()
}

// siteForRequest 是本次请求的站点：按域名解析；解析不到（平台域名、本地地址）用默认站点。
func (a *Application) siteForRequest(r *http.Request) platform.Identifier {
	if site, ok := a.hostSite(r); ok && site.SiteID != "" {
		return site.SiteID
	}
	return a.currentSite(r.Context())
}

// siteGate 处理已解析站点的状态：非主域名跳转主域名；下架或删除的站点公开页返回 410。
// 返回 true 表示已写出响应。
func (a *Application) siteGate(w http.ResponseWriter, r *http.Request) bool {
	site, ok := a.hostSite(r)
	if !ok {
		return false
	}
	path := r.URL.Path
	adminPath := strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/_admin/") || strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/_site/")
	if site.Status != "" && site.Status != "active" && !adminPath {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte("<!doctype html><meta charset=utf-8><title>站点不可用</title><p>该站点已下架或已删除。</p>"))
		return true
	}
	if site.RedirectHost != "" && r.Method == http.MethodGet && !adminPath {
		scheme := requestScheme(r)
		target := scheme + "://" + site.RedirectHost + r.URL.RequestURI()
		w.Header().Set("Cache-Control", "public, max-age=300")
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return true
	}
	if site.Visibility == "private" && !privateOpenPath(path) && !a.hasPrivateAccess(r, site.SiteID) && !hasSessionCookie(r) {
		a.denyPrivate(w, r, site)
		return true
	}
	return false
}

func requestScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func hasSessionCookie(r *http.Request) bool {
	cookie, err := r.Cookie("nw_sid")
	return err == nil && cookie.Value != ""
}

// privateOpenPath 是私有站点上不需要站点访问凭据的路径：后台页面与登录接口（登录本身要能打开）、静态资源、进入私有站点的入口。
func privateOpenPath(path string) bool {
	for _, prefix := range []string{"/admin", "/_admin/", "/_site/", "/_pv/", "/_tls/", "/_report", "/v1/sessions", "/v1/auth/", "/v1/oauth/", "/v1/users:", "/v1/registration", "/v1/challenges", "/v1/self", "/v1/initialization"} {
		if path == prefix || strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

const privateCookie = "nw_pv"

func privateScope(siteID platform.Identifier) string { return "private-site:" + string(siteID) }

func (a *Application) hasPrivateAccess(r *http.Request, siteID platform.Identifier) bool {
	cookie, err := r.Cookie(privateCookie)
	if err != nil {
		return false
	}
	payload, err := serviceauth.VerifyCursor(a.signingKey, privateScope(siteID), cookie.Value)
	if err != nil {
		return false
	}
	expires, err := strconv.ParseInt(string(payload), 10, 64)
	return err == nil && time.Now().Unix() < expires
}

// denyPrivate：设了访问密码的站点给访客一个输入密码的页面（成员可以从这里去登录）；没设的，页面请求带去平台域名确认身份；其余请求直接 401。
func (a *Application) denyPrivate(w http.ResponseWriter, r *http.Request, site HostSite) {
	w.Header().Set("Cache-Control", "no-store")
	accept := r.Header.Get("Accept")
	page := r.Method == http.MethodGet && strings.Contains(accept, "text/html")
	member := ""
	if site.PlatformHost != "" {
		member = requestScheme(r) + "://" + site.PlatformHost + "/admin/private-access?site=" + url.QueryEscape(string(site.SiteID)) + "&return=" + url.QueryEscape(r.URL.RequestURI())
	}
	if page && site.PasswordEnabled {
		writePasswordPage(w, http.StatusUnauthorized, r.URL.RequestURI(), member, "")
		return
	}
	if page && member != "" {
		http.Redirect(w, r, member, http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("<!doctype html><meta charset=utf-8><title>私有站点</title><p>该站点仅限成员访问，请先登录。</p>"))
}

// writePasswordPage 输出私有站点的访问密码页（纯表单，不依赖脚本）。
func writePasswordPage(w http.ResponseWriter, status int, back, member, problem string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>私有站点</title>`)
	b.WriteString(`<style>body{font:16px/1.6 system-ui,sans-serif;max-width:22rem;margin:15vh auto;padding:0 1rem;color:#222}input,button{font:inherit;width:100%;box-sizing:border-box;padding:.5rem;margin:.25rem 0}button{cursor:pointer}.e{color:#b00020}a{color:inherit}@media(prefers-color-scheme:dark){body{background:#111;color:#eee}}</style>`)
	b.WriteString(`<h1 style="font-size:1.25rem">这个站点需要访问密码</h1>`)
	if problem != "" {
		b.WriteString(`<p class="e">` + html.EscapeString(problem) + `</p>`)
	}
	b.WriteString(`<form method="post" action="/_pv/password"><input type="hidden" name="return" value="` + html.EscapeString(back) + `">`)
	b.WriteString(`<input type="password" name="password" autocomplete="current-password" required autofocus aria-label="访问密码"><button type="submit">进入</button></form>`)
	if member != "" {
		b.WriteString(`<p><a href="` + html.EscapeString(member) + `">我是站点成员，去登录</a></p>`)
	}
	_, _ = w.Write([]byte(b.String()))
}

// safeReturn 只接受站内相对路径，防止被用来跳到别的网站。
func safeReturn(back string) bool {
	return strings.HasPrefix(back, "/") && !strings.HasPrefix(back, "//") && !strings.Contains(back, "\\")
}

// grantPrivate 发站点专用的短期访问 cookie（12 小时）并回到原页面。
func (a *Application) grantPrivate(w http.ResponseWriter, r *http.Request, site HostSite, back string) {
	expires := time.Now().Add(12 * time.Hour)
	token, err := serviceauth.SignCursor(a.signingKey, privateScope(site.SiteID), []byte(strconv.FormatInt(expires.Unix(), 10)))
	if err != nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: privateCookie, Value: token, Path: "/", Expires: expires, HttpOnly: true, Secure: requestScheme(r) == "https", SameSite: http.SameSiteLaxMode})
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, back, http.StatusFound)
}

// SetPrivateRedeemer 登记兑换私有访问凭据的函数（调身份服务），并挂上站点域名上的三个入口：
// 成员带一次性码回来（/_pv/accept）、访客提交访问密码（/_pv/password）、打开分享链接（/_pv/share）。
func (a *Application) SetPrivateRedeemer(redeem func(ctx context.Context, siteID platform.Identifier, credential PrivateCredential) error) {
	a.redeemPrivate = redeem
	mux := a.httpServer.Handler.(*http.ServeMux)
	redeemFor := func(r *http.Request, site HostSite, credential PrivateCredential) error {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		return a.redeemPrivate(ctx, site.SiteID, credential)
	}
	mux.HandleFunc("GET /_pv/accept", func(w http.ResponseWriter, r *http.Request) {
		site, ok := a.hostSite(r)
		code := r.URL.Query().Get("code")
		back := r.URL.Query().Get("return")
		if !ok || site.Visibility != "private" || code == "" || !safeReturn(back) {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := redeemFor(r, site, PrivateCredential{Code: code}); err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		a.grantPrivate(w, r, site, back)
	})
	mux.HandleFunc("POST /_pv/password", func(w http.ResponseWriter, r *http.Request) {
		site, ok := a.hostSite(r)
		r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
		if !ok || site.Visibility != "private" || !site.PasswordEnabled || r.ParseForm() != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		back, password := r.PostForm.Get("return"), r.PostForm.Get("password")
		if !safeReturn(back) {
			back = "/"
		}
		member := ""
		if site.PlatformHost != "" {
			member = requestScheme(r) + "://" + site.PlatformHost + "/admin/private-access?site=" + url.QueryEscape(string(site.SiteID)) + "&return=" + url.QueryEscape(back)
		}
		if password == "" || len(password) > 128 {
			writePasswordPage(w, http.StatusBadRequest, back, member, "请输入访问密码。")
			return
		}
		if err := redeemFor(r, site, PrivateCredential{Password: password}); err != nil {
			message := "访问密码不对。"
			var failure *platform.Error
			if errors.As(err, &failure) && failure.Code == platform.CodeRateLimited {
				message = "试错太多，请过几分钟再试。"
			}
			writePasswordPage(w, http.StatusForbidden, back, member, message)
			return
		}
		a.grantPrivate(w, r, site, back)
	})
	mux.HandleFunc("GET /_pv/share", func(w http.ResponseWriter, r *http.Request) {
		site, ok := a.hostSite(r)
		token := r.URL.Query().Get("token")
		if !ok || site.Visibility != "private" || token == "" || len(token) > 128 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := redeemFor(r, site, PrivateCredential{ShareToken: token}); err != nil {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("<!doctype html><meta charset=utf-8><title>私有站点</title><p>分享链接无效或已过期，请向站点所有者要一个新的。</p>"))
			return
		}
		a.grantPrivate(w, r, site, "/")
	})
}

var _ sync.Locker = (*sync.Mutex)(nil)

// SetTLSAllow 登记“域名可否签证书”的询问函数，供入口前面的 Caddy（on-demand TLS）调用。
func (a *Application) SetTLSAllow(allow func(context.Context, string) (bool, error)) {
	a.tlsAllow = allow
	mux := a.httpServer.Handler.(*http.ServeMux)
	mux.HandleFunc("GET /_tls/allow", func(w http.ResponseWriter, r *http.Request) {
		domain := strings.ToLower(strings.TrimSuffix(r.URL.Query().Get("domain"), "."))
		if domain == "" || len(domain) > 253 {
			http.Error(w, "bad domain", http.StatusBadRequest)
			return
		}
		// 部署时配置的公开域名（SITE_DOMAIN）一定放行：站主还没在后台填平台域名时，靠它拿到第一张证书。
		if bootstrap, err := url.Parse(a.config.PublicBaseURL); err == nil && bootstrap.Hostname() != "" && strings.EqualFold(bootstrap.Hostname(), domain) {
			w.WriteHeader(http.StatusOK)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		ok, err := a.tlsAllow(ctx, domain)
		switch {
		case err != nil:
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		case ok:
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "not allowed", http.StatusForbidden)
		}
	})
}
