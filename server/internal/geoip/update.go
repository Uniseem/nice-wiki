package geoip

import (
	"compress/gzip"
	"context"
	"errors"
	"github.com/oschwald/maxminddb-golang/v2"
	"io"
	"net/http"
	"nice-wiki/internal/fsutil"
	"os"
	"time"
)

func MonthlyURL(now time.Time) string {
	return "https://download.db-ip.com/free/dbip-country-lite-" + now.UTC().Format("2006-01") + ".mmdb.gz"
}
func NeedsUpdate(path string, now time.Time) bool {
	info, err := os.Stat(path)
	return err != nil || now.Sub(info.ModTime()) >= 30*24*time.Hour
}

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
	compressed, err := gzip.NewReader(io.LimitReader(response.Body, 128<<20))
	if err != nil {
		return err
	}
	data, readErr := io.ReadAll(io.LimitReader(compressed, 128<<20))
	closeErr := compressed.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	if len(data) < 1024 {
		return errors.New("GeoIP 数据库内容无效")
	}
	reader, err := maxminddb.OpenBytes(data)
	if err != nil {
		return err
	}
	if err := errors.Join(reader.Verify(), reader.Close()); err != nil {
		return err
	}
	return fsutil.WriteFile(target, data, 0600)
}
