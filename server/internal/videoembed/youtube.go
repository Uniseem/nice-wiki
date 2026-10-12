package videoembed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type youtube struct{ client HTTPClient }

func NewYouTube(client HTTPClient) Provider { return &youtube{client: client} }
func (*youtube) Name() string               { return "youtube" }
func (*youtube) Recognize(value *url.URL) (string, bool) {
	host := value.Hostname()
	var id string
	switch host {
	case "youtu.be":
		id = value.Path[1:]
	case "www.youtube.com", "youtube.com", "m.youtube.com":
		if value.Path == "/watch" {
			id = value.Query().Get("v")
		} else if len(value.Path) > len("/shorts/") && value.Path[:len("/shorts/")] == "/shorts/" {
			id = value.Path[len("/shorts/"):]
		}
	}
	return id, validID(id, 32)
}
func (*youtube) EmbedURL(id string) (string, error) {
	if !validID(id, 32) {
		return "", ErrUnsupported
	}
	return "https://www.youtube-nocookie.com/embed/" + id, nil
}
func (p *youtube) Fetch(ctx context.Context, id string) (Metadata, error) {
	if p.client == nil {
		return Metadata{}, ErrManualMetadata
	}
	target := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape("https://www.youtube.com/watch?v="+id)
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
		return Metadata{}, fmt.Errorf("YouTube 公开信息返回状态 %d", response.StatusCode)
	}
	var value struct {
		Title     string `json:"title"`
		Author    string `json:"author_name"`
		Thumbnail string `json:"thumbnail_url"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(nil, response.Body, 1<<20))
	decodeErr := decoder.Decode(&value)
	if err := errors.Join(decodeErr, response.Body.Close()); err != nil {
		return Metadata{}, err
	}
	return Metadata{Title: value.Title, Author: value.Author, ThumbnailURL: value.Thumbnail}, nil
}
