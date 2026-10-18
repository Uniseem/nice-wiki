package publicsite

import (
	"context"
	"nice-wiki/libs/platform"
)

type Breadcrumb struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}
type BuildBreadcrumbsInput struct {
	DocumentID string
	Tree       []NavigationNode
	HomeURL    string
}
type BuildBreadcrumbsResult struct{ Items []Breadcrumb }

// BuildBreadcrumbs 沿公开父链返回首页、祖先与当前文档，绝不按标题重建路径。
func BuildBreadcrumbs(ctx context.Context, op platform.OperationContext, in BuildBreadcrumbsInput) (BuildBreadcrumbsResult, error) {
	nodes := make(map[string]NavigationNode, len(in.Tree))
	for _, node := range in.Tree {
		if _, ok := nodes[node.DocumentID]; ok {
			return BuildBreadcrumbsResult{}, platform.NewError(platform.CodeIntegrityError, false, "DUPLICATE_DOCUMENT")
		}
		nodes[node.DocumentID] = node
	}
	chain := make([]Breadcrumb, 0)
	seen := make(map[string]bool)
	id := in.DocumentID
	for id != "" {
		if seen[id] {
			return BuildBreadcrumbsResult{}, platform.NewError(platform.CodeIntegrityError, false, "TREE_CYCLE")
		}
		seen[id] = true
		node, ok := nodes[id]
		if !ok {
			return BuildBreadcrumbsResult{}, platform.NewError(platform.CodeNotFound, false, "NODE_NOT_FOUND")
		}
		chain = append(chain, Breadcrumb{node.Title, node.CanonicalPath})
		id = ""
		if node.ParentID != nil {
			id = *node.ParentID
		}
	}
	items := make([]Breadcrumb, 0, len(chain)+1)
	items = append(items, Breadcrumb{"首页", in.HomeURL})
	for i := len(chain) - 1; i >= 0; i-- {
		items = append(items, chain[i])
	}
	return BuildBreadcrumbsResult{items}, nil
}
