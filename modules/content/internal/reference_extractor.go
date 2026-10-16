package content

import (
	"encoding/json"
	"strconv"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type CollectResourceReferencesInput struct {
	DocumentID    Identifier                `json:"documentId"`
	RevisionID    Identifier                `json:"revisionId"`
	SourceVersion gen.Revision              `json:"sourceVersion"`
	Body          DocumentBody              `json:"body"`
	Kind          gen.ResourceReferenceKind `json:"kind"`
}

type CollectResourceReferencesResult struct {
	References []gen.ResourceReference `json:"references"`
}

type resourceAssetKey struct {
	assetID Identifier
	version gen.Revision
}

type resourceSourceKey struct {
	blockID Identifier
	asset   resourceAssetKey
}

type resourceReferenceCollector struct {
	input      CollectResourceReferencesInput
	references []gen.ResourceReference
	assets     map[resourceAssetKey]gen.AssetVersionRef
	seen       map[resourceSourceKey]bool
}

// CollectResourceReferences 按正文前序抽取精确引用；当前稿、发布稿和历史由调用方分类。
func CollectResourceReferences(input CollectResourceReferencesInput) (CollectResourceReferencesResult, error) {
	value, err := bodyJSONValue(input.Body)
	if err != nil {
		return CollectResourceReferencesResult{}, err
	}
	collector := resourceReferenceCollector{
		input: input, references: make([]gen.ResourceReference, 0),
		assets: make(map[resourceAssetKey]gen.AssetVersionRef), seen: make(map[resourceSourceKey]bool),
	}
	for _, child := range value.(map[string]any)["content"].([]any) {
		if err := collector.visitReferenceNode(child.(map[string]any)); err != nil {
			return CollectResourceReferencesResult{}, err
		}
	}
	return CollectResourceReferencesResult{References: collector.references}, nil
}

func (collector *resourceReferenceCollector) visitReferenceNode(node map[string]any) error {
	attrs, _ := node["attrs"].(map[string]any)
	blockID, _ := attrs["blockId"].(string)
	var assets []any
	switch node["type"] {
	case "image", "attachment":
		assets = []any{attrs["asset"]}
	case "drawing":
		assets = []any{attrs["scene"], attrs["lightSVG"], attrs["darkSVG"]}
	case "video":
		source := attrs["source"].(map[string]any)
		if source["kind"] == "uploaded" {
			assets = append(assets, source["asset"])
		} else {
			assets = append(assets, source["snapshot"].(map[string]any)["cover"])
		}
		assets = append(assets, attrs["poster"])
	}
	for _, asset := range assets {
		if err := collector.appendResourceReference(blockID, asset); err != nil {
			return err
		}
	}
	children, _ := node["content"].([]any)
	for _, child := range children {
		if err := collector.visitReferenceNode(child.(map[string]any)); err != nil {
			return err
		}
	}
	return nil
}

func (collector *resourceReferenceCollector) appendResourceReference(blockID Identifier, value any) error {
	if value == nil {
		return nil
	}
	object := value.(map[string]any)
	version, err := strconv.Atoi(object["version"].(json.Number).String())
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "资源版本不能表示为服务修订号")
	}
	asset := gen.AssetVersionRef{
		AssetId: object["assetId"].(string), Version: version,
		Sha256: object["sha256"].(string), ByteSize: contentInteger(object["byteSize"]), MediaType: object["mediaType"].(string),
	}
	assetKey := resourceAssetKey{assetID: asset.AssetId, version: asset.Version}
	if previous, found := collector.assets[assetKey]; found && previous != asset {
		return platform.NewError(platform.CodeIntegrityError, false, "同一资源版本的哈希、长度或媒体类型不一致")
	}
	collector.assets[assetKey] = asset
	sourceKey := resourceSourceKey{blockID: blockID, asset: assetKey}
	if collector.seen[sourceKey] {
		return nil
	}
	if len(collector.references) == 10000 {
		return platform.NewError(platform.CodeValidation, false, "正文资源引用超过10000项")
	}
	collector.seen[sourceKey] = true
	collector.references = append(collector.references, gen.ResourceReference{
		SourceService: "content", DocumentId: collector.input.DocumentID, SourceRevisionId: collector.input.RevisionID,
		SourceVersion: collector.input.SourceVersion, Kind: collector.input.Kind, BlockId: blockID, Asset: asset, Retained: true,
	})
	return nil
}
