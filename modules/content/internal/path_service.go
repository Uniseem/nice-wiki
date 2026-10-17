package content

import (
	"context"
	"regexp"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type PlanOldPathRedirectsInput struct {
	DocumentId   Identifier             `json:"documentId"`
	Documents    []gen.DocumentMetadata `json:"documents"`
	TreeRevision int                    `json:"treeRevision"`
}
type PlanOldPathRedirectsResult struct {
	Redirects []gen.RouteTombstone `json:"redirects"`
}

// PlanOldPathRedirects只规划精确已知旧路径，不提交manifest，也不将未知已发布路径当空。
func PlanOldPathRedirects(input PlanOldPathRedirectsInput) (PlanOldPathRedirectsResult, error) {
	if _, err := EnsureTreeAcyclic(EnsureTreeAcyclicInput{Documents: input.Documents}); err != nil {
		return PlanOldPathRedirectsResult{}, err
	}
	index := make(map[Identifier]gen.DocumentMetadata, len(input.Documents))
	for _, document := range input.Documents {
		index[document.DocumentId] = document
	}
	if _, found := index[input.DocumentId]; !found {
		return PlanOldPathRedirectsResult{}, platform.NewError(platform.CodeNotFound, false, "旧路径规划节点不存在")
	}
	subtree, err := EnsureTreeAcyclic(EnsureTreeAcyclicInput{Documents: input.Documents, MovingID: &input.DocumentId})
	if err != nil {
		return PlanOldPathRedirectsResult{}, err
	}
	included := make(map[Identifier]bool, len(subtree.Descendants)+1)
	included[input.DocumentId] = true
	for _, id := range subtree.Descendants {
		included[id] = true
	}
	result := PlanOldPathRedirectsResult{Redirects: []gen.RouteTombstone{}}
	paths := map[string]Identifier{}
	for _, document := range treePreorder(input.Documents, true) {
		if !included[document.DocumentId] {
			continue
		}
		everPublished := document.PublishedReference != nil || document.LastPublishedAt != nil || document.State == "published" || document.VisibilityRevision > 0
		if !everPublished {
			continue
		}
		if document.PublishedReference != nil && document.PublishedReference.DocumentId != document.DocumentId {
			return result, platform.NewError(platform.CodeIntegrityError, false, "旧路径发布引用文档不匹配")
		}
		segments := make([]string, 0, 32)
		current := document
		for {
			if current.PathSegment == nil {
				return result, platform.NewError(platform.CodeDependencyUnavailable, true, "曾发布节点或祖先缺少权威路径，不能视为空重定向")
			}
			segments = append(segments, *current.PathSegment)
			if current.ParentId == nil {
				break
			}
			current = index[*current.ParentId]
		}
		for i, j := 0, len(segments)-1; i < j; i, j = i+1, j-1 {
			segments[i], segments[j] = segments[j], segments[i]
		}
		path := "/" + strings.Join(segments, "/")
		if len(path) > 2048 {
			return result, platform.NewError(platform.CodeIntegrityError, false, "旧路径超过登记长度")
		}
		if previous, found := paths[path]; found && previous != document.DocumentId {
			return result, platform.NewError(platform.CodeIntegrityError, false, "不同文档绑定同一旧路径")
		}
		paths[path] = document.DocumentId
		target := document.DocumentId
		if len(result.Redirects) == 10000 {
			return PlanOldPathRedirectsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "旧路径规划输出超出登记预算")
		}
		result.Redirects = append(result.Redirects, gen.RouteTombstone{OldPath: path, DocumentId: document.DocumentId, Status: gen.RouteTombstoneStatus("redirect"), TargetDocumentId: &target, Revision: input.TreeRevision})
	}
	return result, nil
}

type ValidatePathSegmentInput struct {
	Segment    string                 `json:"segment"`
	ParentId   *Identifier            `json:"parentId"`
	DocumentId Identifier             `json:"documentId"`
	Documents  []gen.DocumentMetadata `json:"documents"`
}
type ValidatePathSegmentResult struct {
	Valid bool `json:"valid"`
}

var pathSegmentPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ValidatePathSegment只消费调用方同SQL可信树，不另查身份/存储或改历史重定向。
func ValidatePathSegment(ctx context.Context, op OperationContext, input ValidatePathSegmentInput) (ValidatePathSegmentResult, error) {
	return validatePathSnapshot(input)
}

func validatePathSnapshot(input ValidatePathSegmentInput) (ValidatePathSegmentResult, error) {
	if len(input.Segment) < 1 || len(input.Segment) > 128 || !pathSegmentPattern.MatchString(input.Segment) {
		return ValidatePathSegmentResult{}, platform.NewError(platform.CodeValidation, false, "文档路径名不符合英文slug规则")
	}
	if input.ParentId == nil && (input.Segment == "navigation" || input.Segment == "ask") {
		return ValidatePathSegmentResult{}, platform.NewError(platform.CodeConflict, false, "根级路径与固定公开入口冲突")
	}
	parentFound := input.ParentId == nil
	for _, document := range input.Documents {
		if input.ParentId != nil && document.DocumentId == *input.ParentId {
			parentFound = true
			if document.State != "draft" && document.State != "published" {
				return ValidatePathSegmentResult{}, platform.NewError(platform.CodeLocked, false, "路径父节点不是活动文档")
			}
		}
		if document.DocumentId == input.DocumentId || (document.State != "draft" && document.State != "published") || !sameIdentifierPointer(document.ParentId, input.ParentId) {
			continue
		}
		if document.PathSegment != nil && *document.PathSegment == input.Segment {
			return ValidatePathSegmentResult{}, platform.NewError(platform.CodeConflict, false, "同父活动文档已使用该路径")
		}
	}
	if !parentFound {
		return ValidatePathSegmentResult{}, platform.NewError(platform.CodeNotFound, false, "路径父节点不存在")
	}
	return ValidatePathSegmentResult{Valid: true}, nil
}
