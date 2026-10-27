package content

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/netguard"
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

// ---- S3 兼容存储（AWS Signature V4，路径式寻址） ----

type s3Store struct {
	client     *http.Client
	endpoint   *url.URL
	bucket     string
	region     string
	prefix     string
	accessKey  string
	secretKey  string
	pathPrefix string
}

func newS3Store(client *http.Client, endpoint *url.URL, bucket, region, prefix, accessKey, secretKey string) *s3Store {
	return &s3Store{client: client, endpoint: endpoint, bucket: bucket, region: region, prefix: cleanPrefix(prefix), accessKey: accessKey, secretKey: secretKey}
}

func awsEscape(s string, keepSlash bool) string {
	var out strings.Builder
	for _, b := range []byte(s) {
		switch {
		case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9', b == '-', b == '_', b == '.', b == '~':
			out.WriteByte(b)
		case b == '/' && keepSlash:
			out.WriteByte(b)
		default:
			fmt.Fprintf(&out, "%%%02X", b)
		}
	}
	return out.String()
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func (s *s3Store) do(ctx context.Context, method, key string, query url.Values, body []byte) (*http.Response, error) {
	basePath := strings.TrimRight(s.endpoint.Path, "/")
	canonicalPath := basePath + "/" + awsEscape(s.bucket, false)
	if key != "" {
		canonicalPath += "/" + awsEscape(key, true)
	}
	var pairs []string
	for k, values := range query {
		for _, v := range values {
			pairs = append(pairs, awsEscape(k, false)+"="+awsEscape(v, false))
		}
	}
	sort.Strings(pairs)
	canonicalQuery := strings.Join(pairs, "&")
	target := s.endpoint.Scheme + "://" + s.endpoint.Host + canonicalPath
	if canonicalQuery != "" {
		target += "?" + canonicalQuery
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "存储地址不合法")
	}
	now := time.Now().UTC()
	amzDate, date := now.Format("20060102T150405Z"), now.Format("20060102")
	sum := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(sum[:])
	host := s.endpoint.Host
	canonicalRequest := strings.Join([]string{method, canonicalPath, canonicalQuery,
		"host:" + host + "\nx-amz-content-sha256:" + payloadHash + "\nx-amz-date:" + amzDate + "\n", "host;x-amz-content-sha256;x-amz-date", payloadHash}, "\n")
	scope := date + "/" + s.region + "/s3/aws4_request"
	requestHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(requestHash[:])
	signingKey := hmacSHA256(hmacSHA256(hmacSHA256(hmacSHA256([]byte("AWS4"+s.secretKey), date), s.region), "s3"), "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.accessKey+"/"+scope+", SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="+signature)
	if method == http.MethodPut {
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "备份目标不可达")
	}
	return resp, nil
}

func (s *s3Store) Put(ctx context.Context, name string, data []byte) error {
	resp, err := s.do(ctx, http.MethodPut, joinKey(s.prefix, name), nil, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return responseError(resp, "写入备份")
	}
	return nil
}

func (s *s3Store) Delete(ctx context.Context, name string) error {
	resp, err := s.do(ctx, http.MethodDelete, joinKey(s.prefix, name), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return responseError(resp, "删除旧备份")
	}
	return nil
}

func (s *s3Store) List(ctx context.Context, prefix string) ([]string, error) {
	query := url.Values{"list-type": {"2"}, "prefix": {joinKey(s.prefix, prefix)}, "max-keys": {"1000"}}
	resp, err := s.do(ctx, http.MethodGet, "", query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, responseError(resp, "列出备份")
	}
	var listing struct {
		Keys []string `xml:"Contents>Key"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&listing); err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "备份目标返回的列表不能解析")
	}
	var names []string
	for _, key := range listing.Keys {
		if s.prefix != "" {
			key = strings.TrimPrefix(key, s.prefix+"/")
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
