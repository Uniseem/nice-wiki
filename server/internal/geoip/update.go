package geoip

import (
	"context"
	"errors"
	"io"
	"net/http"
	"nice-wiki/internal/fsutil"
)

func Download(ctx context.Context, client *http.Client, target, url string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return errors.New("GeoIP 数据库下载失败")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 128<<20))
	if err != nil {
		return err
	}
	if len(data) < 1024 {
		return errors.New("GeoIP 数据库内容无效")
	}
	return fsutil.WriteFile(target, data, 0600)
}
