// Package searchclient 是 Rust 检索服务（rust/search）的 Go 客户端。
// 认证：共享密钥对“时间戳 方法 路径 请求体摘要”做 HMAC-SHA256（与服务端 auth.rs 一致）。
package searchclient

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
)

type Client struct {
	base   string
	secret []byte
	http   *http.Client
}

// New 读取密钥文件并建立客户端；base 形如 http://127.0.0.1:8120。
func New(base, secretFile string) (*Client, error) {
	raw, err := os.ReadFile(secretFile)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "检索服务密钥不可读")
	}
	secret := []byte(strings.TrimSpace(string(raw)))
	if len(secret) < 16 {
		return nil, platform.NewError(platform.CodeValidation, false, "检索服务密钥太短")
	}
	return &Client{base: strings.TrimRight(base, "/"), secret: secret, http: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

type Chunk struct {
	ChunkID string         `json:"chunkId"`
	Terms   map[string]int `json:"terms"`
	Vector  []float32      `json:"vector,omitempty"`
}

type Replace struct {
	SpaceID string  `json:"spaceId"`
	BuildID string  `json:"buildId"`
	Chunks  []Chunk `json:"chunks"`
}

type Query struct {
	SpaceID   string    `json:"spaceId"`
	Terms     []string  `json:"terms,omitempty"`
	Vector    []float32 `json:"vector,omitempty"`
	Documents []string  `json:"documents,omitempty"`
	Limit     int       `json:"limit,omitempty"`
}

type Hit struct {
	ChunkID    string  `json:"chunkId"`
	BuildID    string  `json:"buildId"`
	DocumentID string  `json:"documentId"`
	Score      float64 `json:"score"`
}

type Result struct {
	Lexical []Hit `json:"lexical"`
	Vector  []Hit `json:"vector"`
}

func (c *Client) do(ctx context.Context, method, target string, body any, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return platform.NewError(platform.CodeInternal, false, "检索请求无法编码")
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, c.base+target, bytes.NewReader(payload))
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "检索请求无法建立")
	}
	timestamp := time.Now().Unix()
	digest := sha256.Sum256(payload)
	mac := hmac.New(sha256.New, c.secret)
	fmt.Fprintf(mac, "%d\n%s\n%s\n%s", timestamp, method, target, hex.EncodeToString(digest[:]))
	request.Header.Set("X-Nw-Timestamp", strconv.FormatInt(timestamp, 10))
	request.Header.Set("X-Nw-Signature", hex.EncodeToString(mac.Sum(nil)))
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "检索服务不可达")
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if response.StatusCode >= 400 {
		if response.StatusCode >= 500 {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "检索服务出错")
		}
		return platform.NewError(platform.CodeValidation, false, "检索服务拒绝了请求")
	}
	if out != nil && len(data) > 0 && json.Unmarshal(data, out) != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "检索服务响应不符合约定")
	}
	return nil
}

func (c *Client) ReplaceDocument(ctx context.Context, site, document string, in Replace) error {
	return c.do(ctx, http.MethodPut, "/v1/sites/"+url.PathEscape(site)+"/documents/"+url.PathEscape(document), in, nil)
}

func (c *Client) DeleteDocument(ctx context.Context, site, document string) error {
	return c.do(ctx, http.MethodDelete, "/v1/sites/"+url.PathEscape(site)+"/documents/"+url.PathEscape(document), nil, nil)
}

func (c *Client) DeleteSite(ctx context.Context, site string) error {
	return c.do(ctx, http.MethodDelete, "/v1/sites/"+url.PathEscape(site), nil, nil)
}

// Manifest 是检索服务里这个站点每篇文档当前对应的构建，用于和权威记录对账。
func (c *Client) Manifest(ctx context.Context, site string) (map[string]string, error) {
	var out struct {
		Documents map[string]string `json:"documents"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/sites/"+url.PathEscape(site)+"/manifest", nil, &out); err != nil {
		return nil, err
	}
	return out.Documents, nil
}

func (c *Client) Search(ctx context.Context, site string, in Query) (Result, error) {
	var out Result
	err := c.do(ctx, http.MethodPost, "/v1/sites/"+url.PathEscape(site)+"/search", in, &out)
	return out, err
}
