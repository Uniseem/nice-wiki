package publicsite

import (
	"context"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/public-site/internal/gen"
)

type ResolvePublicMediaInput struct {
	MediaRefs   []AssetVersionRef
	Publication gen.PublishedRevisionRef
}
type ResolvePublicMediaResult struct{ Assets []PublicAsset }

// ResolvePublicMedia 只接收精确发布引用，不从单个revision数字推断文档权限。
// media不可达时返回空清单，渲染器依合同输出固定占位，不生成未知地址。
func ResolvePublicMedia(ctx context.Context, op platform.OperationContext, in ResolvePublicMediaInput) (ResolvePublicMediaResult, error) {
	if len(in.MediaRefs) == 0 {
		return ResolvePublicMediaResult{[]PublicAsset{}}, nil
	}
	directory, err := directoryFrom(ctx)
	if err != nil {
		return ResolvePublicMediaResult{}, err
	}
	if directory.Media == nil {
		return ResolvePublicMediaResult{[]PublicAsset{}}, nil
	}
	raw, err := directory.Media.BuildDeliveryManifest(ctx, map[string]any{"assets": in.MediaRefs, "publishedReference": in.Publication, "previewId": nil, "previewToken": nil})
	if err != nil {
		var failure *platform.Error
		if errors.As(err, &failure) && failure.Code == platform.CodeDependencyUnavailable {
			return ResolvePublicMediaResult{[]PublicAsset{}}, nil
		}
		return ResolvePublicMediaResult{}, err
	}
	var response struct {
		Manifest *struct {
			PublishedReference *gen.PublishedRevisionRef `json:"publishedReference"`
			Items              []struct {
				Asset       AssetVersionRef `json:"asset"`
				URL         string          `json:"url"`
				CachePolicy string          `json:"cachePolicy"`
				Width       int             `json:"width"`
				Height      int             `json:"height"`
				Variants    []struct {
					Width  int             `json:"width"`
					Height int             `json:"height"`
					Asset  AssetVersionRef `json:"asset"`
				} `json:"variants"`
			} `json:"items"`
		} `json:"manifest"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Manifest == nil || response.Manifest.PublishedReference == nil || !samePublication(*response.Manifest.PublishedReference, in.Publication) {
		return ResolvePublicMediaResult{}, dependency("公开资源manifest来源无法核验")
	}
	requested := make(map[AssetVersionRef]bool, len(in.MediaRefs))
	for _, asset := range in.MediaRefs {
		requested[asset] = true
	}
	assets := make([]PublicAsset, 0, len(response.Manifest.Items))
	for _, item := range response.Manifest.Items {
		if !requested[item.Asset] || item.CachePolicy != "immutable_public" || !safeLink(item.URL) {
			return ResolvePublicMediaResult{}, platform.NewError(platform.CodeIntegrityError, false, "MEDIA_MANIFEST_MISMATCH")
		}
		asset := PublicAsset{Asset: item.Asset, URL: item.URL, Width: item.Width, Height: item.Height, Srcset: []ImageSource{}}
		for _, variant := range item.Variants {
			asset.Srcset = append(asset.Srcset, ImageSource{Width: variant.Width, URL: "/v1/assets/" + variant.Asset.AssetID + "/bytes?sha=" + variant.Asset.SHA256})
		}
		if len(asset.Srcset) > 0 && item.Width > 0 {
			asset.Srcset = append(asset.Srcset, ImageSource{Width: item.Width, URL: item.URL})
		}
		assets = append(assets, asset)
	}
	return ResolvePublicMediaResult{assets}, nil
}
