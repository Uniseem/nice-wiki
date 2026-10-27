package content

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/secretbox"
	"nice-wiki/libs/platform/serviceauth"
)

// SiteData 汇集站点数据功能（导入、导出、备份、Git 同步）需要的运行依赖。
type SiteData struct {
	Drafts DraftRuntime
	Locks  EditLockRuntime
	Tree   TreeRuntime
	Media  *MediaGateway
	Box    *secretbox.Box
	Epoch  Identifier
	// AllowLocal 只在本地开发时为真：备份目标和 Git 远端允许 http 与环回地址。
	AllowLocal bool
}

// Context 给上下文挂上草稿、锁和文档树运行时，导入导出里调用的核心函数都要它们。
func (d SiteData) Context(ctx context.Context) context.Context {
	return WithTreeRuntime(WithEditLockRuntime(WithDraftRuntime(ctx, d.Drafts), d.Locks), d.Tree)
}

// step 给一次内部调用换一个新的操作标识：草稿相关操作按它做幂等，一次导入里的多次调用不能共用同一个。
func step(op OperationContext) OperationContext {
	next := op
	id := platform.NewIdentifier()
	next.OperationID = id
	next.RequestID = id
	return next
}

// MediaGateway 是内容域调用媒体服务的最小客户端：读资源元数据与原文件字节、导入一个文件。
type MediaGateway struct {
	base   string
	client *http.Client
}

func NewMediaGateway(baseURL, issuer string, key platform.SigningKey) *MediaGateway {
	return &MediaGateway{base: strings.TrimRight(baseURL, "/"), client: &http.Client{
		Transport:     serviceauth.SigningTransport(issuer, "media", key, nil),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// AssetInfo 是导出需要的资源信息。
type AssetInfo struct {
	Ref          map[string]any `json:"ref"`
	OriginalName string         `json:"originalName"`
	MediaType    string         `json:"mediaType"`
}

func (m *MediaGateway) do(ctx context.Context, op OperationContext, method, path string, body io.Reader, headers map[string]string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(platform.WithPortalProof(platform.WithOperationContext(ctx, op), ""), method, m.base+path, body)
	if err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "媒体请求构造失败")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体服务不可达")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "读取媒体响应失败")
	}
	if resp.StatusCode >= 300 {
		return nil, platform.NewError(platform.CodeNotFound, false, "媒体服务拒绝了请求")
	}
	return raw, nil
}

// ReadAsset 读资源元数据（版本引用、原始文件名、类型）。
func (m *MediaGateway) ReadAsset(ctx context.Context, op OperationContext, assetID string) (AssetInfo, error) {
	raw, err := m.do(ctx, op, http.MethodGet, "/v1/assets/"+url.PathEscape(assetID), nil, map[string]string{"Accept": "application/json"}, 1<<20)
	if err != nil {
		return AssetInfo{}, err
	}
	var out struct {
		Asset struct {
			Ref          map[string]any `json:"ref"`
			OriginalName string         `json:"originalName"`
		} `json:"asset"`
	}
	if json.Unmarshal(raw, &out) != nil || out.Asset.Ref == nil {
		return AssetInfo{}, platform.NewError(platform.CodeIntegrityError, false, "媒体元数据格式不对")
	}
	info := AssetInfo{Ref: out.Asset.Ref, OriginalName: out.Asset.OriginalName}
	info.MediaType, _ = out.Asset.Ref["mediaType"].(string)
	return info, nil
}

// ReadBytes 读某个版本的原文件字节（上限 128MiB）。
func (m *MediaGateway) ReadBytes(ctx context.Context, op OperationContext, assetID string, version int) ([]byte, error) {
	return m.do(ctx, op, http.MethodGet, "/v1/assets/"+url.PathEscape(assetID)+"/bytes?version="+strconv.Itoa(version), nil, map[string]string{"Accept": "application/octet-stream"}, 128<<20)
}

// Import 把一个文件交给媒体服务登记（走完整的上传校验与配额），返回新资源的版本引用。
func (m *MediaGateway) Import(ctx context.Context, op OperationContext, name string, data []byte, createdBy string) (map[string]any, error) {
	raw, err := m.do(ctx, op, http.MethodPost, "/v1/asset-imports", bytes.NewReader(data), map[string]string{
		"Content-Type": "application/octet-stream", "X-Original-Name": url.PathEscape(name), "X-Created-By": createdBy,
	}, 1<<20)
	if err != nil {
		return nil, err
	}
	var out struct {
		Asset struct {
			Ref map[string]any `json:"ref"`
		} `json:"asset"`
	}
	if json.Unmarshal(raw, &out) != nil || out.Asset.Ref == nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "媒体导入回执格式不对")
	}
	return out.Asset.Ref, nil
}

// isAssetRef 判断一个 JSON 对象是不是资源版本引用（五个字段齐全）。
func isAssetRef(m map[string]any) bool {
	for _, key := range []string{"assetId", "version", "sha256", "mediaType", "byteSize"} {
		if _, ok := m[key]; !ok {
			return false
		}
	}
	return true
}

// walkAssetRefs 遍历 JSON，对每个资源引用调用 fn；fn 返回新引用时就地替换。
func walkAssetRefs(v any, fn func(ref map[string]any) map[string]any) any {
	switch node := v.(type) {
	case map[string]any:
		if isAssetRef(node) {
			if replaced := fn(node); replaced != nil {
				return replaced
			}
			return node
		}
		for key, child := range node {
			node[key] = walkAssetRefs(child, fn)
		}
		return node
	case []any:
		for i, child := range node {
			node[i] = walkAssetRefs(child, fn)
		}
		return node
	}
	return v
}

var idAttrs = map[string]bool{"blockId": true, "itemId": true, "cellId": true, "rowId": true, "columnId": true}

// regenerateIDs 给正文里的所有稳定 ID 换新（复制内容时的规则），返回旧→新的映射。
func regenerateIDs(v any, mapping map[string]string) {
	switch node := v.(type) {
	case map[string]any:
		if attrs, ok := node["attrs"].(map[string]any); ok {
			for key := range idAttrs {
				if old, ok := attrs[key].(string); ok {
					fresh := platform.NewIdentifier()
					mapping[old] = fresh
					attrs[key] = fresh
				}
			}
		}
		for _, child := range node {
			regenerateIDs(child, mapping)
		}
	case []any:
		for _, child := range node {
			regenerateIDs(child, mapping)
		}
	}
}

// remapLinks 把站内链接指向的文档和块换成新 ID；指向的文档不在导入范围内时去掉链接标记。
func remapLinks(v any, docs, blocks map[string]string) any {
	switch node := v.(type) {
	case map[string]any:
		if node["type"] == "text" {
			if marks, ok := node["marks"].([]any); ok {
				kept := make([]any, 0, len(marks))
				for _, raw := range marks {
					mark, _ := raw.(map[string]any)
					if mark["type"] == "link" {
						attrs, _ := mark["attrs"].(map[string]any)
						target, _ := attrs["target"].(map[string]any)
						if target["kind"] == "internal" {
							doc, ok := docs[stringOf(target["documentId"])]
							if !ok {
								continue
							}
							target["documentId"] = doc
							if old := stringOf(target["blockId"]); old != "" {
								if fresh, ok := blocks[old]; ok {
									target["blockId"] = fresh
								} else {
									target["blockId"] = nil
								}
							}
						}
					}
					kept = append(kept, raw)
				}
				node["marks"] = kept
			}
			return node
		}
		for key, child := range node {
			node[key] = remapLinks(child, docs, blocks)
		}
		return node
	case []any:
		for i, child := range node {
			node[i] = remapLinks(child, docs, blocks)
		}
		return node
	}
	return v
}

func stringOf(v any) string {
	s, _ := v.(string)
	return s
}
