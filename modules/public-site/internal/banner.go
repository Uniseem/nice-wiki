package publicsite

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"time"

	"nice-wiki/libs/platform"
)

// loadAnnouncement 读取站点配置里的公告横幅；没有、未启用、不在时间窗内或读取失败都返回空串，不影响页面。
func loadAnnouncement(ctx context.Context, op platform.OperationContext) string {
	directory, err := directoryFrom(ctx)
	if err != nil || directory.SiteConfig == nil {
		return ""
	}
	raw, err := directory.SiteConfig.ReadSiteConfig(ctx)
	if err != nil {
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
