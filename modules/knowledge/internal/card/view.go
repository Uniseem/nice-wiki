package card

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type ReadDocumentIndexInput = gen.ReadDocumentIndexInput
type ReadDocumentIndexResult = gen.ReadDocumentIndexResult

// treeNode 是发布树里的一篇文档（前序）。
type treeNode struct {
	DocumentID    string  `json:"documentId"`
	ParentID      *string `json:"parentId"`
	Title         string  `json:"title"`
	CanonicalPath string  `json:"canonicalPath"`
}

func publishedTree(ctx context.Context, op OperationContext, runtime indexer.Runtime) ([]treeNode, error) {
	if runtime.Content == nil || runtime.Validators["ContentReadPublicTreeResult"] == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "发布树权威未装配")
	}
	service := op
	service.CallerService = "knowledge"
	service.Actor = platform.AuthContext{SubjectID: "knowledge", Kind: "service", SiteID: op.Actor.SiteID, ExpiresAt: op.Deadline, Grants: []string{}}
	raw, err := runtime.Content.ReadPublicTree(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service))
	if err != nil {
		return nil, err
	}
	if err = runtime.Validators["ContentReadPublicTreeResult"](raw); err != nil {
		return nil, err
	}
	var tree struct {
		Nodes []treeNode `json:"nodes"`
	}
	if json.Unmarshal(raw, &tree) != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "发布树不可读")
	}
	return tree.Nodes, nil
}

// TreeNode 是发布树里的一篇文档（前序），供层级摘要使用。
type TreeNode = treeNode

// PublishedTree 读 content 的发布树（前序）。
func PublishedTree(ctx context.Context, op OperationContext, runtime indexer.Runtime) ([]TreeNode, error) {
	return publishedTree(ctx, op, runtime)
}

func documentTitle(ctx context.Context, op OperationContext, runtime indexer.Runtime, documentID string) (string, error) {
	nodes, err := publishedTree(ctx, op, runtime)
	if err != nil {
		return "", err
	}
	for _, node := range nodes {
		if node.DocumentID == documentID {
			return node.Title, nil
		}
	}
	return "", nil
}

// entry 是视图里的一个选项：一篇文档（三级细节之一）或一个折叠后的分区。
type entry struct {
	node     treeNode
	depth    int
	order    int
	weight   int
	card     *Card
	level    int      // 文档：2 详细、1 摘要、0 只有主题；分区为 -1
	children []string // 分区包含的文档
	section  string   // 分区选项的说明
}

func (e *entry) text() string {
	if e.level < 0 {
		return e.section
	}
	if e.card == nil {
		return e.node.Title
	}
	parts := []string{e.card.Title}
	if e.level >= 1 {
		parts = append(parts, "Summary: "+e.card.Summary)
	}
	if e.level != 1 && len(e.card.Topics) > 0 {
		parts = append(parts, "Topics: "+strings.Join(e.card.Topics, ", "))
	}
	if e.level == 2 && len(e.card.Questions) > 0 {
		parts = append(parts, "Answers: "+strings.Join(e.card.Questions, "; "))
	}
	return strings.Join(parts, "\n")
}

func cost(text string) int { return platform.EstimateInputTokens(len(text), 1) }

// ReadDocumentIndex 按预算把卡片和发布树压缩成文档索引视图，不调模型：
// 先全部用详细卡片；超预算时按重要度从低到高降到摘要、再降到只剩主题；还超预算或选项太多，就从最深、文档最多的子树开始折叠成分区选项。
// rootDocumentId 指定时只看这篇文档及其子文档（展开分区）。另给一份 L1 的全量 Markdown（index.md）。
func ReadDocumentIndex(ctx context.Context, op OperationContext, input ReadDocumentIndexInput) (ReadDocumentIndexResult, error) {
	if op.Actor.Kind != "service" || op.CallerService != "qa" {
		return ReadDocumentIndexResult{}, platform.NewError(platform.CodeForbidden, false, "文档索引只给问答服务读取")
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return ReadDocumentIndexResult{}, err
	}
	nodes, err := publishedTree(ctx, op, runtime)
	if err != nil {
		return ReadDocumentIndexResult{}, err
	}
	cards, err := readCards(ctx, op, runtime)
	if err != nil {
		return ReadDocumentIndexResult{}, err
	}
	depth := map[string]int{}
	parent := map[string]string{}
	for _, node := range nodes {
		if node.ParentID != nil {
			parent[node.DocumentID] = *node.ParentID
			depth[node.DocumentID] = depth[*node.ParentID] + 1
		}
	}
	inScope := func(id string) bool {
		if input.RootDocumentId == nil {
			return true
		}
		for current := id; current != ""; current = parent[current] {
			if current == string(*input.RootDocumentId) {
				return true
			}
		}
		return false
	}
	weights := map[string]int{}
	if input.Importance != nil {
		for _, item := range *input.Importance {
			weights[string(item.DocumentId)] = item.Weight
		}
	}
	entries := []*entry{}
	version := []string{}
	markdown := strings.Builder{}
	markdown.WriteString("# Document index\n\n")
	for order, node := range nodes {
		card := cards[node.DocumentID]
		title, summary := node.Title, ""
		if card != nil {
			title, summary = card.Title, ": "+card.Summary
		}
		markdown.WriteString(strings.Repeat("  ", depth[node.DocumentID]) + "- [" + title + "](" + node.CanonicalPath + ")" + summary + "\n")
		version = append(version, node.DocumentID+"|"+node.Title+"|"+title+summary)
		if inScope(node.DocumentID) {
			entries = append(entries, &entry{node: node, depth: depth[node.DocumentID], order: order, weight: weights[node.DocumentID], card: card, level: 2})
		}
	}
	budget, options := 12000, 200
	if input.BudgetTokens != nil {
		budget = *input.BudgetTokens
	}
	if input.MaxOptions != nil {
		options = *input.MaxOptions
	}
	total := func() int {
		sum := 0
		for _, e := range entries {
			sum += cost(e.text())
		}
		return sum
	}
	// 降级：重要度低、层级深、排在后面的先降。
	byPriority := append([]*entry{}, entries...)
	sort.SliceStable(byPriority, func(i, j int) bool {
		a, b := byPriority[i], byPriority[j]
		if a.weight != b.weight {
			return a.weight < b.weight
		}
		if a.depth != b.depth {
			return a.depth > b.depth
		}
		return a.order > b.order
	})
	for level := 1; level >= 0 && total() > budget; level-- {
		for _, e := range byPriority {
			if total() <= budget {
				break
			}
			e.level = level
		}
	}
	// 折叠：从最深、包含文档最多的子树开始，换成一个分区选项。
	for total() > budget || len(entries) > options {
		folded := foldDeepest(entries, parent)
		if folded == nil {
			break
		}
		entries = folded
	}
	// 平铺的大站折叠不动时，只留最重要的文档。
	if len(entries) > options {
		kept := append([]*entry{}, entries...)
		sort.SliceStable(kept, func(i, j int) bool { return kept[i].weight > kept[j].weight })
		keep := map[*entry]bool{}
		for _, e := range kept[:options] {
			keep[e] = true
		}
		entries = entries[:0]
		for _, e := range kept {
			if keep[e] {
				entries = append(entries, e)
			}
		}
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].order < entries[j].order })
	}
	result := ReadDocumentIndexResult{VersionHash: platform.HashBytes([]byte(strings.Join(version, "\n"))), Markdown: markdown.String(), Entries: []gen.DocumentIndexEntry{}}
	for _, e := range entries {
		kind := gen.DocumentIndexEntryKind("document")
		ids := []string{e.node.DocumentID}
		if e.level < 0 {
			kind = "section"
			ids = append(ids, e.children...)
		}
		result.Entries = append(result.Entries, gen.DocumentIndexEntry{Kind: kind, DocumentId: e.node.DocumentID, DocumentIds: ids, Title: e.node.Title, Path: e.node.CanonicalPath, Text: e.text(), Level: e.level})
	}
	return result, nil
}

// foldDeepest 把一棵子树（根和所有后代）换成一个分区选项；没有可折叠的子树时返回 nil。
func foldDeepest(entries []*entry, parent map[string]string) []*entry {
	descendants := map[string][]*entry{}
	for _, e := range entries {
		for up := parent[e.node.DocumentID]; up != ""; up = parent[up] {
			descendants[up] = append(descendants[up], e)
		}
	}
	var root *entry
	for _, e := range entries {
		if len(descendants[e.node.DocumentID]) == 0 || e.level < 0 {
			continue
		}
		if root == nil || e.depth > root.depth || (e.depth == root.depth && len(descendants[e.node.DocumentID]) > len(descendants[root.node.DocumentID])) {
			root = e
		}
	}
	if root == nil {
		return nil
	}
	folded := map[*entry]bool{}
	titles := []string{}
	topics := []string{}
	for _, child := range descendants[root.node.DocumentID] {
		folded[child] = true
		root.children = append(root.children, child.node.DocumentID)
		root.children = append(root.children, child.children...)
		if child.card != nil {
			titles = append(titles, child.card.Title)
			topics = append(topics, child.card.Topics...)
		} else {
			titles = append(titles, child.node.Title)
		}
	}
	name := root.node.Title
	if root.card != nil {
		name = root.card.Title
	}
	text := "Section: " + name + "\nContains: " + clip(strings.Join(titles, "; "), 400)
	if root.card != nil {
		text += "\nSummary: " + root.card.Summary
	}
	if len(topics) > 0 {
		text += "\nTopics: " + clip(strings.Join(topics, ", "), 300)
	}
	root.section = text
	root.level = -1
	root.card = nil
	result := make([]*entry, 0, len(entries)-len(folded))
	for _, e := range entries {
		if !folded[e] {
			result = append(result, e)
		}
	}
	return result
}

func readCards(ctx context.Context, op OperationContext, runtime indexer.Runtime) (map[string]*Card, error) {
	rows, err := runtime.DB.QueryContext(ctx, `SELECT document_id,card_json FROM document_cards WHERE site_id=? AND data_epoch=?`, op.Actor.SiteID, op.DataEpoch)
	if err != nil {
		return nil, platform.NormalizeIOError(err, platform.CodeInternal, false, "文档卡片不可读")
	}
	defer rows.Close()
	cards := map[string]*Card{}
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, platform.NormalizeIOError(err, platform.CodeInternal, false, "文档卡片不可读")
		}
		// 只用英文卡片（有英文标题）；改成英文之前写的旧卡片当作没有，下次建索引时重写。
		var card Card
		if json.Unmarshal([]byte(raw), &card) == nil && card.Title != "" {
			cards[id] = &card
		}
	}
	return cards, platform.NormalizeIOError(rows.Err(), platform.CodeInternal, false, "文档卡片不可读")
}
