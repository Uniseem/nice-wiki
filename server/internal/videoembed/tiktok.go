package videoembed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type tiktok struct{ client HTTPClient }

func NewTikTok(client HTTPClient) Provider { return &tiktok{client: client} }
func (*tiktok) Name() string               { return "tiktok" }
func (*tiktok) Recognize(value *url.URL) (string, bool) {
	if value.Hostname() != "www.tiktok.com" && value.Hostname() != "tiktok.com" {
		return "", false
	}
	parts := strings.Split(strings.Trim(value.Path, "/"), "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "@") || parts[1] != "video" {
		return "", false
	}
	for _, r := range parts[2] {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return parts[2], len(parts[2]) >= 10 && len(parts[2]) <= 32
}
func (*tiktok) EmbedURL(id string) (string, error) {
	if !validID(id, 32) {
		return "", ErrUnsupported
	}
	return "https://www.tiktok.com/player/v1/" + id, nil
}
func (p *tiktok) Fetch(ctx context.Context, id string) (Metadata, error) {
	if p.client == nil {
		return Metadata{}, ErrManualMetadata
	}
	target := "https://www.tiktok.com/oembed?url=" + url.QueryEscape("https://www.tiktok.com/@video/video/"+id)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Metadata{}, err
	}
	response, err := p.client.Do(request)
	if err != nil {
		return Metadata{}, err
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return Metadata{}, fmt.Errorf("TikTok 公开信息返回状态 %d", response.StatusCode)
	}
	var value struct {
		Title     string `json:"title"`
		Author    string `json:"author_name"`
		Thumbnail string `json:"thumbnail_url"`
	}
	decodeErr := json.NewDecoder(http.MaxBytesReader(nil, response.Body, 1<<20)).Decode(&value)
	if err := errors.Join(decodeErr, response.Body.Close()); err != nil {
		return Metadata{}, err
	}
	return Metadata{Title: value.Title, Author: value.Author, ThumbnailURL: value.Thumbnail}, nil
}
