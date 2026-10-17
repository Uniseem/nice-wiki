package content

import (
	"context"
	"encoding/json"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type PreparePublicationMetadataInput struct {
	Metadata          gen.DocumentMetadata `json:"metadata"`
	Body              DocumentBody         `json:"body"`
	ManualPathSegment *string              `json:"manualPathSegment"`
	ManualSEO         *string              `json:"manualSEO"`
}

type PreparePublicationMetadataResult struct {
	Metadata gen.DocumentMetadata `json:"metadata"`
	Warnings []string             `json:"warnings"`
}

// PreparePublicationMetadata只合并人工slug/SEO与已有英文路径。
// 本轮不调用模型，不制造slug或SEO；缺项只记提示。
func PreparePublicationMetadata(ctx context.Context, op OperationContext, input PreparePublicationMetadataInput) (PreparePublicationMetadataResult, error) {
	if op.CallerService != "portal" || op.Actor.Kind != "admin" {
		return PreparePublicationMetadataResult{}, platform.NewError(platform.CodeForbidden, false, "发布元数据仅允许当前管理账户")
	}
	metadata := input.Metadata
	warnings := []string{}
	if input.ManualPathSegment != nil {
		metadata.PathSegment = input.ManualPathSegment
	} else if metadata.PathSegment == nil {
		warnings = append(warnings, "publication.slug_missing")
	} else if len(*metadata.PathSegment) < 1 || len(*metadata.PathSegment) > 128 || !pathSegmentPattern.MatchString(*metadata.PathSegment) {
		return PreparePublicationMetadataResult{}, platform.NewError(platform.CodeIntegrityError, false, "已保存路径不是可保持的英文slug")
	}
	switch {
	case input.ManualSEO != nil:
		metadata.SeoDescription = input.ManualSEO
		metadata.SeoDescriptionSource = gen.DocumentMetadataSeoDescriptionSourceManual
	case metadata.SeoDescriptionSource == gen.DocumentMetadataSeoDescriptionSourceManual || metadata.SeoDescriptionSource == gen.DocumentMetadataSeoDescriptionSourceGenerated:
		if metadata.SeoDescription == nil {
			return PreparePublicationMetadataResult{}, platform.NewError(platform.CodeIntegrityError, false, "已保存SEO来源缺少描述")
		}
	case metadata.SeoDescriptionSource == gen.DocumentMetadataSeoDescriptionSourcePending:
		if metadata.SeoDescription != nil {
			return PreparePublicationMetadataResult{}, platform.NewError(platform.CodeIntegrityError, false, "待生成SEO已带有未确认描述")
		}
		warnings = append(warnings, "publication.seo_missing")
	default:
		return PreparePublicationMetadataResult{}, platform.NewError(platform.CodeIntegrityError, false, "SEO来源不在已登记枚举")
	}
	return PreparePublicationMetadataResult{Metadata: metadata, Warnings: warnings}, nil
}

func publishedBodySchema(body DocumentBody) (int, error) {
	switch value := body.SchemaVersion.(type) {
	case float64:
		if value == 1 {
			return 1, nil
		}
	case int:
		if value == 1 {
			return 1, nil
		}
	case int64:
		if value == 1 {
			return 1, nil
		}
	case json.Number:
		parsed, err := value.Int64()
		if err == nil && parsed == 1 {
			return 1, nil
		}
	}
	return 0, platform.NewError(platform.CodeIntegrityError, false, "已提交正文schemaVersion不是可发布的1")
}
