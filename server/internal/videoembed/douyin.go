package videoembed

import (
	"context"
	"net/url"
	"strings"
)

type douyin struct{ client HTTPClient }

func NewDouyin(client HTTPClient) Provider { return &douyin{client: client} }
func (*douyin) Name() string               { return "douyin" }
func (*douyin) Recognize(value *url.URL) (string, bool) {
	if value.Hostname() != "www.douyin.com" && value.Hostname() != "douyin.com" {
		return "", false
	}
	parts := strings.Split(strings.Trim(value.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "video" {
		return "", false
	}
	for _, r := range parts[1] {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return parts[1], len(parts[1]) >= 10 && len(parts[1]) <= 32
}
func (*douyin) EmbedURL(id string) (string, error) {
	if !validID(id, 32) {
		return "", ErrUnsupported
	}
	return "https://open.douyin.com/player/video?vid=" + url.QueryEscape(id), nil
}
func (*douyin) Fetch(context.Context, string) (Metadata, error) { return Metadata{}, ErrManualMetadata }
