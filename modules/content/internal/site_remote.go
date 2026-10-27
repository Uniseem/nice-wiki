package content

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/netguard"
	"nice-wiki/libs/platform/objectstore"
)

// remoteStore 是备份目标的最小抽象：写入、列出、删除。S3 兼容存储与 WebDAV 各一个实现。
type remoteStore interface {
	Put(ctx context.Context, name string, data []byte) error
	List(ctx context.Context, prefix string) ([]string, error)
	Delete(ctx context.Context, name string) error
}

// remoteClient 是备份目标专用的 HTTP 客户端：拨号前检查实际 IP（防 SSRF），不跟随跳转。
func remoteClient(allowLoopback bool) *http.Client {
	return &http.Client{
		Timeout: 15 * time.Minute,
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second, Control: netguard.Control(allowLoopback)}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConns:        2,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func cleanPrefix(prefix string) string { return strings.Trim(strings.TrimSpace(prefix), "/") }

func joinKey(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

func responseError(resp *http.Response, what string) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	text := strings.Join(strings.Fields(string(raw)), " ")
	return platform.NewError(platform.CodeDependencyUnavailable, true, "%s失败（%d）%s", what, resp.StatusCode, firstN(text, 160))
}

func firstN(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// ---- S3 兼容存储（客户端在 libs/platform/objectstore） ----

type s3Store struct{ client *objectstore.Client }

func newS3Store(client *http.Client, endpoint *url.URL, bucket, region, prefix, accessKey, secretKey string) (*s3Store, error) {
	c, err := objectstore.New(client, objectstore.Config{Endpoint: endpoint.String(), Bucket: bucket, Region: region, Prefix: prefix, AccessKey: accessKey, SecretKey: secretKey})
	if err != nil {
		return nil, err
	}
	return &s3Store{client: c}, nil
}

func (s *s3Store) Put(ctx context.Context, name string, data []byte) error {
	return s.client.Put(ctx, s.client.Key(name), bytes.NewReader(data), int64(len(data)))
}

func (s *s3Store) Delete(ctx context.Context, name string) error {
	return s.client.Delete(ctx, s.client.Key(name))
}

func (s *s3Store) List(ctx context.Context, prefix string) ([]string, error) {
	keys, err := s.client.List(ctx, s.client.Key(prefix))
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(s.client.Key(""), "/")
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		if base != "" {
			key = strings.TrimPrefix(key, base+"/")
		}
		names = append(names, key)
	}
	return names, nil
}

// ---- WebDAV ----

type davStore struct {
	client *http.Client
	base   *url.URL
	user   string
	pass   string
	prefix string
}

func newDavStore(client *http.Client, base *url.URL, user, pass, prefix string) *davStore {
	return &davStore{client: client, base: base, user: user, pass: pass, prefix: cleanPrefix(prefix)}
}

func (d *davStore) url(key string) string {
	segments := strings.Split(strings.Trim(joinKey(d.prefix, key), "/"), "/")
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment != "" {
			escaped = append(escaped, url.PathEscape(segment))
		}
	}
	return strings.TrimRight(d.base.String(), "/") + "/" + strings.Join(escaped, "/")
}

func (d *davStore) do(ctx context.Context, method, target string, body []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "存储地址不合法")
	}
	req.SetBasicAuth(d.user, d.pass)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "备份目标不可达")
	}
	return resp, nil
}

// ensureDirs 逐级创建前缀目录（已存在时服务端回 405，忽略）。
func (d *davStore) ensureDirs(ctx context.Context) error {
	if d.prefix == "" {
		return nil
	}
	var path string
	for _, segment := range strings.Split(d.prefix, "/") {
		if segment == "" {
			continue
		}
		path = joinKey(path, segment)
		target := strings.TrimRight(d.base.String(), "/") + "/" + strings.ReplaceAll(url.PathEscape(path), "%2F", "/")
		resp, err := d.do(ctx, "MKCOL", target, nil, nil)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 && resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusConflict {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "创建备份目录失败（%d）", resp.StatusCode)
		}
	}
	return nil
}

func (d *davStore) Put(ctx context.Context, name string, data []byte) error {
	if err := d.ensureDirs(ctx); err != nil {
		return err
	}
	resp, err := d.do(ctx, http.MethodPut, d.url(name), data, map[string]string{"Content-Type": "application/octet-stream"})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return responseError(resp, "写入备份")
	}
	return nil
}

func (d *davStore) Delete(ctx context.Context, name string) error {
	resp, err := d.do(ctx, http.MethodDelete, d.url(name), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return responseError(resp, "删除旧备份")
	}
	return nil
}

func (d *davStore) List(ctx context.Context, prefix string) ([]string, error) {
	resp, err := d.do(ctx, "PROPFIND", strings.TrimRight(d.url(""), "/")+"/", []byte(`<?xml version="1.0"?><propfind xmlns="DAV:"><prop><resourcetype/></prop></propfind>`), map[string]string{"Depth": "1", "Content-Type": "application/xml"})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		return nil, responseError(resp, "列出备份")
	}
	var listing struct {
		Hrefs []string `xml:"response>href"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&listing); err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "备份目标返回的列表不能解析")
	}
	var names []string
	for _, href := range listing.Hrefs {
		decoded, err := url.PathUnescape(href)
		if err != nil {
			continue
		}
		name := decoded[strings.LastIndex(strings.TrimRight(decoded, "/"), "/")+1:]
		if name != "" && strings.HasPrefix(name, prefix) && !strings.HasSuffix(decoded, "/") {
			names = append(names, name)
		}
	}
	return names, nil
}
