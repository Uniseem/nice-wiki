package publicsite

import (
	"context"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"sync"
	"time"

	"nice-wiki/libs/platform/theme"
)

// 站点外观（docs/theme-standard.md）：site-config 告诉我们用哪个外观包（摘要），包按摘要取一次后常驻内存（内容不可变）。
// 页面上：<html> 挂版式开关，<head> 内联配色/字体样式，有 theme.css 时以 /_theme/<摘要>/theme.css 引入。

// siteTheme 是渲染一页需要的外观信息。
type siteTheme struct {
	Look        theme.Look
	CSS         template.CSS // 由 theme.json 生成的内联样式
	Stylesheet  string       // 补充样式表地址，没有为空
	ColorScheme string       // 访客默认明暗：system | light | dark
	Forced      string       // 外观只支持一种配色时为那一种，服务端直接写到 <html> 上
}

type appearanceEntry struct {
	digest, scheme string
	expires        time.Time
}

var (
	appearanceCache = struct {
		sync.Mutex
		entries map[string]appearanceEntry
	}{entries: map[string]appearanceEntry{}}
	// packageCache 按“站点|摘要”缓存外观包；包内容由摘要决定，不会过期，超过上限时整体清空。
	packageCache = struct {
		sync.Mutex
		entries map[string]*theme.Package
	}{entries: map[string]*theme.Package{}}
)

const maxCachedPackages = 64

// readAppearanceCached 读站点当前外观的摘要与默认明暗，和站点配置一样短缓存；改外观后几秒内生效（入口页面缓存另行清除）。
func readAppearanceCached(ctx context.Context) (string, string, bool) {
	site := siteKey(ctx)
	appearanceCache.Lock()
	if entry, ok := appearanceCache.entries[site]; ok && time.Now().Before(entry.expires) {
		appearanceCache.Unlock()
		return entry.digest, entry.scheme, true
	}
	appearanceCache.Unlock()
	directory, err := directoryFrom(ctx)
	if err != nil || directory.SiteConfig == nil {
		return "", "", false
	}
	raw, err := directory.SiteConfig.ReadPublicAppearance(ctx)
	if err != nil {
		return "", "", false
	}
	var result struct {
		ThemeDigest string `json:"themeDigest"`
		ColorScheme string `json:"colorScheme"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return "", "", false
	}
	appearanceCache.Lock()
	appearanceCache.entries[site] = appearanceEntry{result.ThemeDigest, result.ColorScheme, time.Now().Add(5 * time.Second)}
	appearanceCache.Unlock()
	return result.ThemeDigest, result.ColorScheme, true
}

// themePackage 按摘要取外观包：内置外观直接用，其余向 site-config 取一次后缓存。取不到返回 nil。
func themePackage(ctx context.Context, digest string) *theme.Package {
	if pkg := theme.Builtin(digest); pkg != nil {
		return pkg
	}
	if !theme.ValidDigest(digest) {
		return nil
	}
	key := siteKey(ctx) + "|" + digest
	packageCache.Lock()
	pkg := packageCache.entries[key]
	packageCache.Unlock()
	if pkg != nil {
		return pkg
	}
	directory, err := directoryFrom(ctx)
	if err != nil || directory.SiteConfig == nil {
		return nil
	}
	response, err := directory.SiteConfig.ReadThemePackage(ctx, digest)
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, theme.MaxPackageBytes+1))
	if err != nil {
		return nil
	}
	pkg, err = theme.Parse(raw)
	if err != nil || pkg.Digest != digest {
		return nil
	}
	packageCache.Lock()
	if len(packageCache.entries) >= maxCachedPackages {
		packageCache.entries = map[string]*theme.Package{}
	}
	packageCache.entries[key] = pkg
	packageCache.Unlock()
	return pkg
}

// loadSiteTheme 取站点外观；读不到时用默认外观，页面照常渲染。
func loadSiteTheme(ctx context.Context) siteTheme {
	pkg, scheme := theme.Default(), "system"
	if digest, colorScheme, ok := readAppearanceCached(ctx); ok {
		if found := themePackage(ctx, digest); found != nil {
			pkg = found
		}
		scheme = colorScheme
	}
	base := "/_theme/" + pkg.Digest + "/"
	view := siteTheme{Look: pkg.Look(), CSS: template.CSS(pkg.HeadCSS(base)), ColorScheme: scheme}
	if pkg.Manifest.Stylesheet != "" {
		view.Stylesheet = base + pkg.Manifest.Stylesheet
	}
	if schemes := pkg.Manifest.ColorSchemes; len(schemes) == 1 {
		view.Forced = schemes[0]
	}
	return view
}

// ServeThemeFile 处理 /_theme/<摘要>/<文件>：只提供本站当前可用的外观包里允许对外的文件，按不可变资源缓存。
func ServeThemeFile(ctx context.Context, w http.ResponseWriter, digest, name string) {
	pkg := themePackage(ctx, digest)
	if pkg == nil || !theme.Servable(name) || pkg.Files[name] == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", theme.ContentType(name))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(pkg.Files[name])
}
