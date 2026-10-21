package publicsite

import (
	"context"
	"encoding/json"
	"html"
	"strings"

	"nice-wiki/libs/platform"
)

const defaultSourceCodeURL = "https://github.com/Uniseem/nice-wiki"

// loadFooter 读取站点配置里的页脚：返回站长写的部分（版权、自定义外链、ICP 备案）的 HTML 和源码链接地址。
// 读取失败或没配置时返回空串和默认源码链接，不影响页面。
func loadFooter(ctx context.Context, op platform.OperationContext) (string, string) {
	directory, err := directoryFrom(ctx)
	if err != nil || directory.SiteConfig == nil {
		return "", defaultSourceCodeURL
	}
	raw, err := directory.SiteConfig.ReadSiteConfig(ctx)
	if err != nil {
		return "", defaultSourceCodeURL
	}
	var config struct {
		Config struct {
			Footer *struct {
				Copyright string `json:"copyright"`
				Links     []struct {
					Text   string `json:"text"`
					Target struct {
						Kind string `json:"kind"`
						URL  string `json:"url"`
						Href string `json:"href"`
					} `json:"target"`
				} `json:"links"`
				ICPNumber     *string `json:"icpNumber"`
				ICPLink       *string `json:"icpLink"`
				SourceCodeURL string  `json:"sourceCodeURL"`
			} `json:"footer"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &config) != nil || config.Config.Footer == nil {
		return "", defaultSourceCodeURL
	}
	footer := config.Config.Footer
	var parts []string
	if text := strings.TrimSpace(footer.Copyright); text != "" {
		parts = append(parts, html.EscapeString(text))
	}
	for _, link := range footer.Links {
		target := link.Target.URL
		if target == "" {
			target = link.Target.Href
		}
		if link.Target.Kind == "external" && strings.HasPrefix(target, "https://") && safeLink(target) {
			parts = append(parts, "<a href=\""+html.EscapeString(target)+"\" rel=\"noopener noreferrer\">"+html.EscapeString(link.Text)+"</a>")
		}
	}
	if footer.ICPNumber != nil && strings.TrimSpace(*footer.ICPNumber) != "" {
		label := html.EscapeString(*footer.ICPNumber)
		if footer.ICPLink != nil && strings.HasPrefix(*footer.ICPLink, "https://") && safeLink(*footer.ICPLink) {
			label = "<a href=\"" + html.EscapeString(*footer.ICPLink) + "\" rel=\"noopener noreferrer\">" + label + "</a>"
		}
		parts = append(parts, label)
	}
	source := defaultSourceCodeURL
	if footer.SourceCodeURL != "" && (strings.HasPrefix(footer.SourceCodeURL, "https://") || strings.HasPrefix(footer.SourceCodeURL, "http://")) && safeLink(footer.SourceCodeURL) {
		source = footer.SourceCodeURL
	}
	return strings.Join(parts, " · "), source
}
