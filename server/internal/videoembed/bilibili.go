package videoembed

import (
	"context"
	"net/url"
	"strings"
)

type bilibili struct{ client HTTPClient }

func NewBilibili(client HTTPClient) Provider { return &bilibili{client: client} }
func (*bilibili) Name() string               { return "bilibili" }
func (*bilibili) Recognize(value *url.URL) (string, bool) {
	if value.Hostname() != "www.bilibili.com" && value.Hostname() != "bilibili.com" {
		return "", false
	}
	parts := strings.Split(strings.Trim(value.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "video" || !strings.HasPrefix(parts[1], "BV") {
		return "", false
	}
	return parts[1], validID(parts[1], 32)
}
func (*bilibili) EmbedURL(id string) (string, error) {
	if !validID(id, 32) || !strings.HasPrefix(id, "BV") {
		return "", ErrUnsupported
	}
	return "https://player.bilibili.com/player.html?bvid=" + url.QueryEscape(id), nil
}
func (*bilibili) Fetch(context.Context, string) (Metadata, error) {
	return Metadata{}, ErrManualMetadata
}
