package publicsite

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"nice-wiki/libs/platform"
)

// loadAnnouncement 读取站点配置里的公告横幅；没有、未启用、不在时间窗内或读取失败都返回空串，不影响页面。
func loadAnnouncement(ctx context.Context, op platform.OperationContext) string {
	directory, err := directoryFrom(ctx)
	if err != nil || directory.SiteConfig == nil {
		return ""
	}
	raw, ok := readSiteConfigCached(ctx)
	if !ok {
		return ""
	}
	var config struct {
		Config struct {
			Banner *struct {
				AnnouncementID string  `json:"announcementId"`
				RevisionID     string  `json:"revisionId"`
				BodyHash       string  `json:"bodyHash"`
				Enabled        bool    `json:"enabled"`
				StartsAt       *string `json:"startsAt"`
				EndsAt         *string `json:"endsAt"`
			} `json:"banner"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &config) != nil || config.Config.Banner == nil || !config.Config.Banner.Enabled {
		return ""
	}
	banner := config.Config.Banner
	response, err := directory.SiteConfig.ReadConfigObject(ctx, banner.RevisionID, url.Values{"version": []string{"1"}, "expectedHash": []string{banner.BodyHash}})
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<10))
	if err != nil {
		return ""
	}
	var content struct {
		Text   string `json:"text"`
		Target *struct {
			Kind string `json:"kind"`
			URL  string `json:"url"`
		} `json:"target"`
	}
	if json.Unmarshal(body, &content) != nil || content.Text == "" {
		return ""
	}
	input := RenderAnnouncementInput{AnnouncementID: banner.AnnouncementID, Text: content.Text, StartAt: "1970-01-01T00:00:00Z", EndAt: "9999-12-31T00:00:00Z", Now: time.Now().UTC()}
	if banner.StartsAt != nil {
		input.StartAt = *banner.StartsAt
	}
	if banner.EndsAt != nil {
		input.EndAt = *banner.EndsAt
	}
	if content.Target != nil && content.Target.Kind == "external" {
		input.Link = content.Target.URL
	}
	rendered, err := RenderAnnouncement(ctx, op, input)
	if err != nil || !rendered.Visible {
		return ""
	}
	return rendered.HTML
}

// qaAvailable 公开问答是否对访客开放（qa 已启用且模型任务可用）。
func qaAvailable(ctx context.Context, op platform.OperationContext) bool {
	directory, err := directoryFrom(ctx)
	return err == nil && directory.QA.Available(ctx, op)
}

// languageLinks 列出已启用语言各自对应的本页地址（把路径开头的语言段换掉）；少于两种语言时返回空。
func languageLinks(ctx context.Context, canonicalPath, current string) []LanguageLink {
	directory, err := directoryFrom(ctx)
	if err != nil || directory.Localization == nil || !strings.HasPrefix(canonicalPath, "/"+current+"/") {
		return nil
	}
	snapshot, err := readLanguageSnapshot(ctx, directory.Localization)
	if err != nil || len(snapshot.versions) < 2 {
		return nil
	}
	tags := make([]string, 0, len(snapshot.versions))
	for tag := range snapshot.versions {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	rest := strings.TrimPrefix(canonicalPath, "/"+current)
	links := make([]LanguageLink, 0, len(tags))
	for _, tag := range tags {
		link := LanguageLink{Tag: tag, Href: "/" + tag + rest}
		if tag != current {
			catalog := loadUIStrings(ctx, tag)
			link.Prompt = make(map[string]string, len(languagePromptKeys))
			for _, key := range languagePromptKeys {
				link.Prompt[key] = uiText(tag, catalog, key)
			}
		}
		links = append(links, link)
	}
	return links
}

// siteForSEO 站点公开地址（去掉末尾斜杠），读取失败时不输出规范链接。
func siteForSEO(ctx context.Context) (string, error) {
	directory, err := directoryFrom(ctx)
	if err != nil {
		return "", err
	}
	snapshot, err := readSiteSnapshot(ctx, directory.SiteConfig)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(snapshot.publicBaseURL, "/"), nil
}
