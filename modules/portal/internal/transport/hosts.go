package transport

import (
	"context"
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

// denyPrivate：页面请求带去平台域名确认身份，其余请求直接 401。
func (a *Application) denyPrivate(w http.ResponseWriter, r *http.Request, site HostSite) {
	w.Header().Set("Cache-Control", "no-store")
	accept := r.Header.Get("Accept")
	if r.Method == http.MethodGet && strings.Contains(accept, "text/html") && site.PlatformHost != "" {
		back := url.QueryEscape(r.URL.RequestURI())
		target := requestScheme(r) + "://" + site.PlatformHost + "/admin/private-access?site=" + url.QueryEscape(string(site.SiteID)) + "&return=" + back
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("<!doctype html><meta charset=utf-8><title>私有站点</title><p>该站点仅限成员访问，请先登录。</p>"))
}

// SetPrivateRedeemer 登记兑换一次性码的函数（调身份服务）。
func (a *Application) SetPrivateRedeemer(redeem func(ctx context.Context, siteID platform.Identifier, code string) error) {
	a.redeemPrivate = redeem
	mux := a.httpServer.Handler.(*http.ServeMux)
	mux.HandleFunc("GET /_pv/accept", func(w http.ResponseWriter, r *http.Request) {
		site, ok := a.hostSite(r)
		code := r.URL.Query().Get("code")
		back := r.URL.Query().Get("return")
		if !ok || site.Visibility != "private" || code == "" || !strings.HasPrefix(back, "/") || strings.HasPrefix(back, "//") || strings.Contains(back, "\\") {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := a.redeemPrivate(ctx, site.SiteID, code); err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		expires := time.Now().Add(12 * time.Hour)
		token, err := serviceauth.SignCursor(a.signingKey, privateScope(site.SiteID), []byte(strconv.FormatInt(expires.Unix(), 10)))
		if err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: privateCookie, Value: token, Path: "/", Expires: expires, HttpOnly: true, Secure: requestScheme(r) == "https", SameSite: http.SameSiteLaxMode})
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, back, http.StatusFound)
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
