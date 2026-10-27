package transport

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// 会改变公开页内容或访问规则的写入操作（“所有者:操作名”）：成功后让 edge 清掉这个站点的页面缓存。
var purgeOperations = map[string]bool{
	"content:publishDocument": true, "content:withdrawDocument": true, "content:rollbackPublishedRevision": true, "content:updateDocumentMetadata": true,
	"content:trashDocument": true, "content:restoreDocument": true, "content:moveDocument": true, "content:reorderChildren": true, "content:purgeDocuments": true,
	"site-config:updateBanner": true, "site-config:updateBranding": true, "site-config:updateSiteConfig": true, "site-config:restoreConfigRevision": true,
	"site-config:updateFooter": true, "site-config:updateHome": true,
	"localization:putLanguage": true, "localization:setLanguageEnabled": true, "localization:approveLanguagePlan": true, "localization:putInterfaceString": true,
	"identity:updateSite": true, "identity:setPrimaryDomain": true, "identity:removeSiteDomain": true, "identity:setSiteStatus": true,
}

var siteHeaderPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// SetEdgePurge 登记 edge 的清缓存地址与共享密钥文件；没有登记（开发时不经 edge）就什么都不做。
func (a *Application) SetEdgePurge(baseURL, secretFile string) {
	if baseURL == "" || secretFile == "" {
		return
	}
	a.edgePurgeURL = strings.TrimRight(baseURL, "/")
	a.edgePurgeSecretFile = secretFile
}

// purgeEdge 异步通知 edge 清掉某个站点的缓存；失败只会让缓存多留几秒（页面缓存本来就有短 TTL），不影响写入结果。
func (a *Application) purgeEdge(site string) {
	if a.edgePurgeURL == "" || !siteHeaderPattern.MatchString(site) {
		return
	}
	secret, err := os.ReadFile(a.edgePurgeSecretFile)
	if err != nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.edgePurgeURL+"/_edge/purge?site="+url.QueryEscape(strings.ToLower(site)), nil)
		if err != nil {
			return
		}
		request.Header.Set("x-nw-purge", strings.TrimSpace(string(secret)))
		if response, err := http.DefaultClient.Do(request); err == nil {
			response.Body.Close()
		}
	}()
}

// targetSiteOf 是这次写入操作针对的站点：请求头里界面选中的站点，其次是域名对应的站点。
func (a *Application) targetSiteOf(r *http.Request, hostSite string) string {
	if v := r.Header.Get("X-Nw-Site"); siteHeaderPattern.MatchString(v) {
		return v
	}
	return hostSite
}
