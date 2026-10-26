package transport

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
)

// HostSite 是域名解析出的站点信息（来自 identity:resolveHost）。
type HostSite struct {
	SiteID       platform.Identifier
	Status       string
	Visibility   string
	PrimaryHost  string
	RedirectHost string
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
		scheme := "https"
		if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" {
			scheme = "http"
		}
		target := scheme + "://" + site.RedirectHost + r.URL.RequestURI()
		w.Header().Set("Cache-Control", "public, max-age=300")
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return true
	}
	return false
}

var _ sync.Locker = (*sync.Mutex)(nil)
