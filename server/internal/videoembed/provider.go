package videoembed

import (
	"context"
	"errors"
	"net/url"
)

var ErrUnsupported = errors.New("不支持的视频地址")
var ErrManualMetadata = errors.New("平台未提供稳定的公开信息接口，请手动填写标题和简介")

type Metadata struct {
	Provider     string `json:"provider"`
	ProviderID   string `json:"providerId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Author       string `json:"author"`
	Duration     int    `json:"duration"`
	ThumbnailURL string `json:"thumbnailUrl"`
	EmbedURL     string `json:"embedUrl"`
}

type Provider interface {
	Name() string
	Recognize(*url.URL) (string, bool)
	EmbedURL(string) (string, error)
	Fetch(context.Context, string) (Metadata, error)
}

func validID(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
