package content

import (
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
