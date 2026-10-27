// Package objectstore 是 S3 兼容对象存储的最小客户端（AWS Signature V4，路径式寻址）：写入、按范围读取、列出、删除。
// 用户站点备份（content）和媒体文件（media）共用。上传用 UNSIGNED-PAYLOAD，大文件不用先读进内存算摘要。
package objectstore

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
)

// Config 是一个存储桶的连接信息；Prefix 是桶内的公共前缀（可空）。
type Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	Prefix    string
	AccessKey string
	SecretKey string
}

// Client 访问一个存储桶。
type Client struct {
	http     *http.Client
	endpoint *url.URL
	cfg      Config
	prefix   string
}

// New 用给定的 HTTP 客户端连接存储桶（调用方决定超时与出站限制）。
func New(client *http.Client, cfg Config) (*Client, error) {
	endpoint, err := url.Parse(strings.TrimSpace(cfg.Endpoint))
	if err != nil || (endpoint.Scheme != "https" && endpoint.Scheme != "http") || endpoint.Host == "" || cfg.Bucket == "" {
		return nil, platform.NewError(platform.CodeValidation, false, "对象存储地址或存储桶不合法")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return &Client{http: client, endpoint: endpoint, cfg: cfg, prefix: strings.Trim(strings.TrimSpace(cfg.Prefix), "/")}, nil
}

// Key 把相对名字拼上公共前缀。
func (c *Client) Key(name string) string {
	if c.prefix == "" {
		return name
	}
	return c.prefix + "/" + name
}

func escape(s string, keepSlash bool) string {
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

// do 签名并发出请求；key 是完整对象键（已含前缀），空表示桶本身。
func (c *Client) do(ctx context.Context, method, key string, query url.Values, body io.Reader, size int64, headers map[string]string) (*http.Response, error) {
	canonicalPath := strings.TrimRight(c.endpoint.Path, "/") + "/" + escape(c.cfg.Bucket, false)
	if key != "" {
		canonicalPath += "/" + escape(key, true)
	}
	var pairs []string
	for k, values := range query {
		for _, v := range values {
			pairs = append(pairs, escape(k, false)+"="+escape(v, false))
		}
	}
	sort.Strings(pairs)
	canonicalQuery := strings.Join(pairs, "&")
	target := c.endpoint.Scheme + "://" + c.endpoint.Host + canonicalPath
	if canonicalQuery != "" {
		target += "?" + canonicalQuery
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "对象存储地址不合法")
	}
	payloadHash := "UNSIGNED-PAYLOAD"
	if body == nil {
		sum := sha256.Sum256(nil)
		payloadHash = hex.EncodeToString(sum[:])
	}
	now := time.Now().UTC()
	amzDate, date := now.Format("20060102T150405Z"), now.Format("20060102")
	canonicalRequest := strings.Join([]string{method, canonicalPath, canonicalQuery,
		"host:" + c.endpoint.Host + "\nx-amz-content-sha256:" + payloadHash + "\nx-amz-date:" + amzDate + "\n", "host;x-amz-content-sha256;x-amz-date", payloadHash}, "\n")
	scope := date + "/" + c.cfg.Region + "/s3/aws4_request"
	requestHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(requestHash[:])
	signingKey := hmacSHA256(hmacSHA256(hmacSHA256(hmacSHA256([]byte("AWS4"+c.cfg.SecretKey), date), c.cfg.Region), "s3"), "aws4_request")
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+c.cfg.AccessKey+"/"+scope+", SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="+hex.EncodeToString(hmacSHA256(signingKey, stringToSign)))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.ContentLength = size
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "对象存储不可达")
	}
	return resp, nil
}

func failure(resp *http.Response, what string) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	text := []rune(strings.Join(strings.Fields(string(raw)), " "))
	if len(text) > 160 {
		text = text[:160]
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "%s失败（%d）%s", what, resp.StatusCode, string(text))
}

// Put 写入对象；size 是 body 的确切字节数。
func (c *Client) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	resp, err := c.do(ctx, http.MethodPut, key, nil, body, size, map[string]string{"Content-Type": "application/octet-stream"})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return failure(resp, "写入对象存储")
	}
	return nil
}

// Get 读对象的 [start, start+length) 一段；length 小于 1 表示读到末尾。
func (c *Client) Get(ctx context.Context, key string, start, length int64) (io.ReadCloser, error) {
	headers := map[string]string{}
	if start > 0 || length > 0 {
		end := ""
		if length > 0 {
			end = strconv.FormatInt(start+length-1, 10)
		}
		headers["Range"] = "bytes=" + strconv.FormatInt(start, 10) + "-" + end
	}
	resp, err := c.do(ctx, http.MethodGet, key, nil, nil, 0, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, platform.NewError(platform.CodeNotFound, false, "对象不存在")
		}
		return nil, failure(resp, "读取对象存储")
	}
	return resp.Body, nil
}

// Delete 删除对象；不存在不算错。
func (c *Client) Delete(ctx context.Context, key string) error {
	resp, err := c.do(ctx, http.MethodDelete, key, nil, nil, 0, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return failure(resp, "删除对象")
	}
	return nil
}

// List 列出以 prefix（完整键前缀）开头的全部对象键，自动翻页。
func (c *Client) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	token := ""
	for {
		query := url.Values{"list-type": {"2"}, "prefix": {prefix}, "max-keys": {"1000"}}
		if token != "" {
			query.Set("continuation-token", token)
		}
		resp, err := c.do(ctx, http.MethodGet, "", query, nil, 0, nil)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 300 {
			defer resp.Body.Close()
			return nil, failure(resp, "列出对象")
		}
		var listing struct {
			Keys      []string `xml:"Contents>Key"`
			Truncated bool     `xml:"IsTruncated"`
			Next      string   `xml:"NextContinuationToken"`
		}
		err = xml.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&listing)
		resp.Body.Close()
		if err != nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "对象存储返回的列表不能解析")
		}
		keys = append(keys, listing.Keys...)
		if !listing.Truncated || listing.Next == "" {
			return keys, nil
		}
		token = listing.Next
	}
}
