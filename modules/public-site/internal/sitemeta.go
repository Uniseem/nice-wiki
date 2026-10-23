package publicsite

import (
	"context"
	"encoding/json"
	"html"
	"strings"
	"sync"
	"time"
)

var siteConfigCache = struct {
	sync.Mutex
	raw     json.RawMessage
	expires time.Time
}{}

// readSiteConfigCached 页面渲染里站点配置被好几处读取（公告、页脚、站点元信息），短缓存避免每页多次远程调用；改配置后几秒内生效。
func readSiteConfigCached(ctx context.Context) (json.RawMessage, bool) {
	siteConfigCache.Lock()
	if siteConfigCache.raw != nil && time.Now().Before(siteConfigCache.expires) {
		raw := siteConfigCache.raw
		siteConfigCache.Unlock()
		return raw, true
	}
	siteConfigCache.Unlock()
	directory, err := directoryFrom(ctx)
	if err != nil || directory.SiteConfig == nil {
		return nil, false
	}
	raw, err := directory.SiteConfig.ReadSiteConfig(ctx)
	if err != nil {
		return nil, false
	}
	siteConfigCache.Lock()
	siteConfigCache.raw, siteConfigCache.expires = raw, time.Now().Add(5*time.Second)
	siteConfigCache.Unlock()
	return raw, true
}

type assetRef struct {
	AssetID string `json:"assetId"`
	SHA256  string `json:"sha256"`
}

func (a *assetRef) url() string {
	if a == nil || a.AssetID == "" || a.SHA256 == "" {
		return ""
	}
	return "/v1/assets/" + a.AssetID + "/bytes?sha=" + a.SHA256
}

// siteMeta 站点级的展示信息：站名、标题模板、主题色、描述、站标/图标/分享图。
type siteMeta struct {
	Name          string
	Description   string
	TitleTemplate string
	ThemeColor    string
	Logo          *assetRef
	Favicon       *assetRef
	ShareImage    *assetRef
}

func loadSiteMeta(ctx context.Context) siteMeta {
	raw, ok := readSiteConfigCached(ctx)
	if !ok {
		return siteMeta{}
	}
	var envelope struct {
		Config struct {
			Name              string    `json:"name"`
			Description       string    `json:"description"`
			TitleTemplate     string    `json:"titleTemplate"`
			ThemeColor        string    `json:"themeColor"`
			Logo              *assetRef `json:"logo"`
			Favicon           *assetRef `json:"favicon"`
			DefaultShareImage *assetRef `json:"defaultShareImage"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return siteMeta{}
	}
	c := envelope.Config
	return siteMeta{c.Name, c.Description, c.TitleTemplate, c.ThemeColor, c.Logo, c.Favicon, c.DefaultShareImage}
}

// pageTitle 按标题模板拼 <title>：{title} 换成文档标题，{site} 换成站名；没有站名或模板时只用文档标题。
func (m siteMeta) pageTitle(title string) string {
	if m.Name == "" || m.TitleTemplate == "" {
		return title
	}
	return strings.NewReplacer("{title}", title, "{site}", m.Name).Replace(m.TitleTemplate)
}

// headTags 输出主题色、图标、Open Graph 与 Twitter 卡片标签（值都已转义）。
func (m siteMeta) headTags(pageTitle, description, canonicalURL, baseURL string) string {
	var out strings.Builder
	if len(m.ThemeColor) == 7 && strings.HasPrefix(m.ThemeColor, "#") {
		out.WriteString("<meta name=\"theme-color\" content=\"" + html.EscapeString(m.ThemeColor) + "\">")
	}
	if icon := m.Favicon.url(); icon != "" {
		out.WriteString("<link rel=\"icon\" href=\"" + html.EscapeString(icon) + "\">")
	} else {
		out.WriteString("<link rel=\"icon\" type=\"image/svg+xml\" href=\"" + html.EscapeString(siteMark(m.ThemeColor)) + "\">")
	}
	out.WriteString("<meta property=\"og:type\" content=\"article\"><meta property=\"og:title\" content=\"" + html.EscapeString(pageTitle) + "\">")
	if description != "" {
		out.WriteString("<meta property=\"og:description\" content=\"" + html.EscapeString(description) + "\">")
	}
	if m.Name != "" {
		out.WriteString("<meta property=\"og:site_name\" content=\"" + html.EscapeString(m.Name) + "\">")
	}
	if canonicalURL != "" {
		out.WriteString("<meta property=\"og:url\" content=\"" + html.EscapeString(canonicalURL) + "\">")
	}
	if image := m.ShareImage.url(); image != "" {
		if strings.HasPrefix(image, "/") {
			image = strings.TrimRight(baseURL, "/") + image // Open Graph 要绝对地址
		}
		out.WriteString("<meta property=\"og:image\" content=\"" + html.EscapeString(image) + "\"><meta name=\"twitter:card\" content=\"summary_large_image\">")
	} else {
		out.WriteString("<meta name=\"twitter:card\" content=\"summary\">")
	}
	return out.String()
}
