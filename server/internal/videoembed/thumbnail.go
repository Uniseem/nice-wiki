package videoembed

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
)

type ThumbnailStore interface {
	SaveThumbnail(context.Context, string, io.Reader) (string, error)
}

func DownloadThumbnail(ctx context.Context, client HTTPClient, store ThumbnailStore, raw string) (string, error) {
	if client == nil || store == nil {
		return "", errors.New("封面下载能力未配置")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Host == "" {
		return "", errors.New("封面地址无效")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return "", errors.New("封面下载失败")
	}
	contentType := response.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		_ = response.Body.Close()
		return "", errors.New("封面文件类型无效")
	}
	value, saveErr := store.SaveThumbnail(ctx, contentType, io.LimitReader(response.Body, 16<<20))
	return value, errors.Join(saveErr, response.Body.Close())
}
