package publicsite

import (
	"context"
	"encoding/json"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/public-site/internal/gen"
)

type NavigationNode struct {
	DocumentID    string                   `json:"documentId"`
	ParentID      *string                  `json:"parentId"`
	Title         string                   `json:"title"`
	CanonicalPath string                   `json:"canonicalPath"`
	SortKey       string                   `json:"sortKey"`
	Publication   gen.PublishedRevisionRef `json:"publication"`
	// 来自同一公开树快照：sitemap 的 lastmod 和 llms 摘要直接用，不逐篇读正文。
	LastPublishedAt time.Time `json:"lastPublishedAt"`
	SEODescription  string    `json:"seoDescription"`
}
type BuildNavigationInput struct {
	Locale    string
	DataEpoch string
	MaxNodes  int64
}
type BuildNavigationResult struct {
	Tree     []NavigationNode `json:"tree"`
	Revision int              `json:"revision"`
}

// BuildNavigation 不使用异步投影授予公开权限，返回前重新核对树水位和每个精确发布引用。
func BuildNavigation(ctx context.Context, op platform.OperationContext, in BuildNavigationInput) (BuildNavigationResult, error) {
	directory, err := directoryFrom(ctx)
	if err != nil {
		return BuildNavigationResult{}, err
	}
	site, err := readSiteSnapshot(ctx, directory.SiteConfig)
	if err != nil {
		return BuildNavigationResult{}, err
	}
	languages, err := readLanguageSnapshot(ctx, directory.Localization)
	if err != nil {
		return BuildNavigationResult{}, err
	}
	if _, ok := languages.versions[in.Locale]; !ok {
		return BuildNavigationResult{}, platform.NewError(platform.CodeValidation, false, "LOCALE_UNSUPPORTED")
	}
	raw, err := directory.Content.ReadPublicTree(ctx)
	if err != nil {
		return BuildNavigationResult{}, err
	}
	var tree struct {
		Nodes []struct {
			DocumentID         string                   `json:"documentId"`
			ParentID           *string                  `json:"parentId"`
			Title              string                   `json:"title"`
			CanonicalPath      string                   `json:"canonicalPath"`
			OrderKey           string                   `json:"orderKey"`
			Reference          gen.PublishedRevisionRef `json:"reference"`
			VisibilityRevision int                      `json:"visibilityRevision"`
			LastPublishedAt    time.Time                `json:"lastPublishedAt"`
			SEODescription     *string                  `json:"seoDescription"`
		}
		TreeRevision int `json:"treeRevision"`
	}
	if err = json.Unmarshal(raw, &tree); err != nil || tree.Nodes == nil {
		return BuildNavigationResult{}, dependency("公开树权威不完整")
	}
	if int64(len(tree.Nodes)) > in.MaxNodes {
		return BuildNavigationResult{}, platform.NewError(platform.CodeValidation, false, "NAVIGATION_LIMIT")
	}
	nodes := make([]NavigationNode, 0, len(tree.Nodes))
	refs := make([]gen.PublishedRevisionRef, 0, len(tree.Nodes))
	for _, node := range tree.Nodes {
		if string(node.Reference.DataEpoch) != in.DataEpoch || in.DataEpoch != string(op.DataEpoch) {
			return BuildNavigationResult{}, platform.NewError(platform.CodeVersionMismatch, false, "EPOCH_MISMATCH")
		}
		href, err := relativeLink(ctx, op, site.publicBaseURL, in.Locale, node.CanonicalPath, "")
		if err != nil {
			return BuildNavigationResult{}, err
		}
		summary := ""
		if node.SEODescription != nil {
			summary = *node.SEODescription
		}
		nodes = append(nodes, NavigationNode{node.DocumentID, node.ParentID, node.Title, href, node.OrderKey, node.Reference, node.LastPublishedAt, summary})
		refs = append(refs, node.Reference)
	}
	// 非基准语言：有标题译文且对应的发布修订没变的文档，导航里显示译文标题和搜索摘要译文；
	// 摘要还没译出时不显示原文摘要（首页卡片、llms 摘要宁可留空也不混入另一种语言）。
	if in.Locale != site.baseLanguage {
		translations := loadTranslatedTitles(ctx, in.Locale)
		for j := range nodes {
			translated, ok := translations[nodes[j].DocumentID]
			if ok && string(nodes[j].Publication.PublishedRevisionId) == translated.revision {
				nodes[j].Title, nodes[j].SEODescription = translated.title, translated.description
			} else {
				nodes[j].SEODescription = ""
			}
		}
	}
	ordered, err := navigationOrder(nodes)
	if err != nil {
		return BuildNavigationResult{}, err
	}
	for start := 0; start < len(refs); start += 100 {
		end := start + 100
		if end > len(refs) {
			end = len(refs)
		}
		raw, err = directory.Content.ValidatePublications(ctx, map[string]any{"refs": refs[start:end]})
		if err != nil {
			return BuildNavigationResult{}, err
		}
		var result struct {
			Validations []struct {
				Reference          gen.PublishedRevisionRef `json:"reference"`
				Visible            bool                     `json:"visible"`
				MatchesCurrent     bool                     `json:"matchesCurrent"`
				VisibilityRevision int                      `json:"visibilityRevision"`
			} `json:"validations"`
		}
		if err = json.Unmarshal(raw, &result); err != nil || len(result.Validations) != end-start {
			return BuildNavigationResult{}, dependency("公开引用权威不完整")
		}
		for i, item := range result.Validations {
			if !item.Visible || !item.MatchesCurrent || !samePublication(item.Reference, refs[start+i]) || item.VisibilityRevision != tree.Nodes[start+i].VisibilityRevision {
				return BuildNavigationResult{}, stale()
			}
		}
	}
	if err = confirmWatermarks(ctx, directory, site, languages); err != nil {
		return BuildNavigationResult{}, err
	}
	again, err := readTree(ctx, directory.Content)
	if err != nil {
		return BuildNavigationResult{}, err
	}
	if again.revision != tree.TreeRevision {
		return BuildNavigationResult{}, stale()
	}
	return BuildNavigationResult{ordered, tree.TreeRevision}, nil
}

// navigationOrder 只整理已由 content 过滤的公开节点，不拼接可能缺失祖先的路径。
// content 按文档树先序给出节点：父级是草稿时子文档提升到上层，位置仍在原父级处，
// 它的排序键却属于原来那一层，不能再按排序键重排，同级保持给出的先后即可。
func navigationOrder(nodes []NavigationNode) ([]NavigationNode, error) {
	byID := make(map[string]NavigationNode, len(nodes))
	children := make(map[string][]NavigationNode)
	for _, node := range nodes {
		if _, exists := byID[node.DocumentID]; exists {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "DUPLICATE_DOCUMENT")
		}
		byID[node.DocumentID] = node
	}
	for _, node := range nodes {
		parent := ""
		if node.ParentID != nil {
			parent = *node.ParentID
			if _, ok := byID[parent]; !ok {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "MISSING_PARENT")
			}
		}
		children[parent] = append(children[parent], node)
	}
	result := make([]NavigationNode, 0, len(nodes))
	seen := make(map[string]bool)
	var visit func(string) error
	visit = func(parent string) error {
		for _, node := range children[parent] {
			if seen[node.DocumentID] {
				return platform.NewError(platform.CodeIntegrityError, false, "TREE_CYCLE")
			}
			seen[node.DocumentID] = true
			result = append(result, node)
			if err := visit(node.DocumentID); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(""); err != nil {
		return nil, err
	}
	if len(result) != len(nodes) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "TREE_CYCLE")
	}
	return result, nil
}
