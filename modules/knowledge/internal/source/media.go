package source

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
	"sort"
	"strconv"
)

type AssetVersionRef struct {
	AssetID   string `json:"assetId"`
	Version   int64  `json:"version"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"mediaType"`
	ByteSize  int64  `json:"byteSize"`
}
type AssetSource struct {
	Asset     AssetVersionRef `json:"asset"`
	Block     gen.BlockRef    `json:"block"`
	Supported bool            `json:"supported"`
}
type ListExtractableAssetsInput struct {
	Document gen.PublishedRevisionRef `json:"document"`
	Source   string                   `json:"source"`
}
type ListExtractableAssetsResult struct {
	Assets            []platform.AssetRef `json:"assets"`
	UnsupportedAssets []platform.AssetRef `json:"unsupportedAssets"`
	Sources           []AssetSource       `json:"sources"`
}

// ListExtractableAssets 保留精确资源version和原块来源；读取授权不等于引用预留。
func ListExtractableAssets(ctx context.Context, op OperationContext, input ListExtractableAssetsInput) (ListExtractableAssetsResult, error) {
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return ListExtractableAssetsResult{}, err
	}
	if runtime.Media == nil || runtime.Validators["MediaReadAssetResult"] == nil {
		return ListExtractableAssetsResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "媒体权威未装配")
	}
	canonical, err := platform.CanonicalJSON(json.RawMessage(input.Source))
	if err != nil {
		return ListExtractableAssetsResult{}, err
	}
	if len(canonical) > 16*1024*1024 || platform.HashBytes(canonical) != input.Document.ContentHash || input.Document.DataEpoch != op.DataEpoch {
		return ListExtractableAssetsResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源清单不属于固定发布正文")
	}
	sources, err := scanAssetSources(canonical, input.Document)
	if err != nil {
		return ListExtractableAssetsResult{}, err
	}
	result := ListExtractableAssetsResult{Assets: []platform.AssetRef{}, UnsupportedAssets: []platform.AssetRef{}, Sources: sources}
	checked := map[string]bool{}
	for i := range result.Sources {
		source := &result.Sources[i]
		ref := source.Asset
		key := ref.AssetID + ":" + strconv.FormatInt(ref.Version, 10) + ":" + ref.SHA256
		supported := extractableMediaType(ref.MediaType) && ref.ByteSize <= 50*1024*1024
		source.Supported = supported
		if checked[key] {
			continue
		}
		checked[key] = true
		if len(checked) > 100 {
			return ListExtractableAssetsResult{}, platform.NewError(platform.CodeValidation, false, "每篇文档最多提取100个资源")
		}
		raw, err := runtime.Media.ReadAsset(platform.WithOperationContext(ctx, op), ref.AssetID, url.Values{"version": {strconv.FormatInt(ref.Version, 10)}})
		if err != nil {
			return ListExtractableAssetsResult{}, err
		}
		if err = runtime.Validators["MediaReadAssetResult"](raw); err != nil {
			return ListExtractableAssetsResult{}, err
		}
		var authority struct {
			Asset struct {
				Ref    AssetVersionRef `json:"ref"`
				SiteID string          `json:"siteId"`
				State  string          `json:"state"`
			} `json:"asset"`
		}
		if json.Unmarshal(raw, &authority) != nil || authority.Asset.Ref != ref || authority.Asset.SiteID != op.Actor.SiteID || authority.Asset.State != "ready" {
			return ListExtractableAssetsResult{}, platform.NewError(platform.CodeIntegrityError, false, "媒体授权版本或内容引用不一致")
		}
		asset := platform.AssetRef{AssetID: ref.AssetID, SHA256: ref.SHA256, MediaType: ref.MediaType, ByteSize: ref.ByteSize}
		if supported {
			result.Assets = append(result.Assets, asset)
		} else {
			result.UnsupportedAssets = append(result.UnsupportedAssets, asset)
		}
	}
	return result, nil
}

// scanAssetSources 沿已核正文树保留最近稳定块；不从URL、文件名或字符串猜资源。
func scanAssetSources(body []byte, document gen.PublishedRevisionRef) ([]AssetSource, error) {
	var root any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&root) != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "正文资源树不可读")
	}
	result := []AssetSource{}
	var walk func(any, *gen.BlockRef) error
	walk = func(value any, current *gen.BlockRef) error {
		switch node := value.(type) {
		case map[string]any:
			if attrs, ok := node["attrs"].(map[string]any); ok {
				if id, ok := attrs["blockId"].(string); ok {
					raw, _ := platform.CanonicalJSON(node)
					ref := gen.BlockRef{Document: document, BlockId: id, BlockHash: platform.HashBytes(raw)}
					current = &ref
				}
			}
			if _, asset := node["assetId"]; asset {
				raw, _ := json.Marshal(node)
				var ref AssetVersionRef
				if json.Unmarshal(raw, &ref) != nil || current == nil || ref.AssetID == "" || ref.SHA256 == "" {
					return platform.NewError(platform.CodeIntegrityError, false, "资源引用缺精确版本或稳定原块")
				}
				result = append(result, AssetSource{Asset: ref, Block: *current})
				return nil
			}
			// map顺序不作为提取优先级；正文有序content先走，其他固定属性保持确定字典序。
			if content, ok := node["content"]; ok {
				if err := walk(content, current); err != nil {
					return err
				}
			}
			keys := make([]string, 0, len(node))
			for key := range node {
				if key != "content" {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				if err := walk(node[key], current); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range node {
				if err := walk(child, current); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(root, nil); err != nil {
		return nil, err
	}
	return result, nil
}
func extractableMediaType(mediaType string) bool {
	switch mediaType {
	case "application/pdf", "text/plain", "text/markdown", "text/csv", "application/json", "image/png", "image/jpeg", "image/webp", "image/gif", "image/svg+xml", "video/mp4", "video/webm":
		return true
	}
	return false
}
