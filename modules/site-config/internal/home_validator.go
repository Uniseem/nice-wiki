package siteconfig

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

type homeDocumentTarget struct {
	ButtonID   gen.Identifier
	DocumentID gen.Identifier
	BlockID    *gen.Identifier
}

type homeAssetUse struct {
	ReferenceID string
	Asset       gen.AssetVersionRef
}

type homeLinkValue struct {
	Kind       string          `json:"kind"`
	DocumentID gen.Identifier  `json:"documentId"`
	BlockID    *gen.Identifier `json:"blockId"`
	URL        string          `json:"url"`
}

// decodeHomeTarget 只转换已通过入口或持久schema校验的判别联合体。
func decodeHomeTarget(target gen.LinkTarget) (homeLinkValue, error) {
	raw, err := target.MarshalJSON()
	if err != nil {
		return homeLinkValue{}, platform.NewError(platform.CodeValidation, false, "首页按钮目标无法读取")
	}
	var value homeLinkValue
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, platform.NewError(platform.CodeValidation, false, "首页按钮目标无法转换")
	}
	return value, nil
}

// validateHomeStructure 不访问远端，不把文档ID猜成当前公开路径。
func validateHomeStructure(home gen.HomeConfig) ([]homeDocumentTarget, []homeAssetUse, error) {
	documents := []homeDocumentTarget{}
	assets := []homeAssetUse{}
	ids := map[string]bool{}
	if home.HeaderLinks != nil {
		for _, link := range *home.HeaderLinks {
			if err := validateConfigWebLink(link.Url, false); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, button := range home.Buttons {
		if ids[button.ButtonId] {
			return nil, nil, platform.NewError(platform.CodeValidation, false, "hero按钮ID重复")
		}
		ids[button.ButtonId] = true
		target, err := decodeHomeTarget(button.Target)
		if err != nil {
			return nil, nil, err
		}
		switch target.Kind {
		case "document":
			documents = append(documents, homeDocumentTarget{ButtonID: button.ButtonId, DocumentID: target.DocumentID, BlockID: target.BlockID})
		case "external":
			if err := validateConfigWebLink(target.URL, true); err != nil {
				return nil, nil, err
			}
		}
	}
	raw, err := home.Background.MarshalJSON()
	if err != nil {
		return nil, nil, platform.NewError(platform.CodeValidation, false, "首页背景无法读取")
	}
	var background struct {
		Kind   string               `json:"kind"`
		Asset  *gen.AssetVersionRef `json:"asset"`
		Poster *gen.AssetVersionRef `json:"poster"`
	}
	if err := json.Unmarshal(raw, &background); err != nil {
		return nil, nil, platform.NewError(platform.CodeValidation, false, "首页背景无法转换")
	}
	switch background.Kind {
	case "image", "animation":
		if !strings.HasPrefix(background.Asset.MediaType, "image/") {
			return nil, nil, platform.NewError(platform.CodeValidation, false, "首页图片背景必须引用图像资源")
		}
		assets = append(assets, homeAssetUse{ReferenceID: "home.background", Asset: *background.Asset})
	case "video":
		if !strings.HasPrefix(background.Asset.MediaType, "video/") || !strings.HasPrefix(background.Poster.MediaType, "image/") {
			return nil, nil, platform.NewError(platform.CodeValidation, false, "首页视频和封面资源类型不符")
		}
		assets = append(assets, homeAssetUse{ReferenceID: "home.background", Asset: *background.Asset}, homeAssetUse{ReferenceID: "home.poster", Asset: *background.Poster})
	}
	return documents, assets, nil
}

type ValidateHomeLinksInput struct {
	Home     gen.HomeConfig `json:"home"`
	SiteID   string         `json:"siteId"`
	Language string         `json:"language"`
}

type ValidateHomeLinksResult struct {
	DocumentRefs []gen.PublishedRevisionRef `json:"documentRefs"`
	AssetIDs     []string                   `json:"assetIds"`
}

// ValidateHomeLinks 只读取免费权威，不获取pin、不触发模型或资源处理。
func ValidateHomeLinks(ctx context.Context, op platform.OperationContext, input ValidateHomeLinksInput) (ValidateHomeLinksResult, error) {
	documents, uses, err := validateHomeStructure(input.Home)
	if err != nil {
		return ValidateHomeLinksResult{}, err
	}
	s := currentActiveStore()
	if s == nil {
		return ValidateHomeLinksResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	runtime := s.homeDependencies()
	result := ValidateHomeLinksResult{DocumentRefs: []gen.PublishedRevisionRef{}, AssetIDs: []string{}}
	for _, target := range documents {
		resolved, err := resolveHomeDocument(ctx, op, runtime, input.SiteID, input.Language, homeLinkValue{Kind: "document", DocumentID: target.DocumentID, BlockID: target.BlockID})
		if err != nil {
			return result, err
		}
		if !resolved.Visible {
			return result, platform.NewError(platform.CodeNotFound, false, "首页文档目标、语言或锚点当前不可公开")
		}
		result.DocumentRefs = append(result.DocumentRefs, *resolved.Publication)
	}
	remoteCtx, _ := homeServiceContext(ctx, op, input.SiteID)
	seen := map[string]bool{}
	for _, use := range uses {
		if runtime.Media == nil {
			return result, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体资源客户端未装配")
		}
		raw, err := runtime.Media.ReadAsset(remoteCtx, use.Asset.AssetId, url.Values{"version": []string{strconv.Itoa(use.Asset.Version)}})
		if err != nil {
			return result, err
		}
		var response struct {
			Asset struct {
				Ref      gen.AssetVersionRef `json:"ref"`
				SiteID   string              `json:"siteId"`
				Kind     string              `json:"kind"`
				State    string              `json:"state"`
				Animated bool                `json:"animated"`
			} `json:"asset"`
		}
		if json.Unmarshal(raw, &response) != nil {
			return result, platform.NewError(platform.CodeIntegrityError, false, "媒体资源响应损坏")
		}
		asset := response.Asset
		if asset.SiteID != input.SiteID || asset.Ref != use.Asset || asset.State != "ready" {
			return result, platform.NewError(platform.CodeValidation, false, "首页资源不属于本站、版本不符或尚未ready")
		}
		if strings.HasPrefix(use.Asset.MediaType, "video/") {
			if asset.Kind != "video" {
				return result, platform.NewError(platform.CodeValidation, false, "首页视频资源种类不符")
			}
		} else if asset.Kind != "image" {
			return result, platform.NewError(platform.CodeValidation, false, "首页图片资源种类不符")
		}
		if use.ReferenceID == "home.background" {
			var background struct {
				Kind string `json:"kind"`
			}
			value, _ := input.Home.Background.MarshalJSON()
			_ = json.Unmarshal(value, &background)
			if (background.Kind == "animation" && !asset.Animated) || (background.Kind == "image" && asset.Animated) {
				return result, platform.NewError(platform.CodeValidation, false, "首页动图声明与资源权威不符")
			}
		}
		if !seen[asset.Ref.AssetId] {
			result.AssetIDs = append(result.AssetIDs, asset.Ref.AssetId)
			seen[asset.Ref.AssetId] = true
		}
	}
	return result, nil
}
