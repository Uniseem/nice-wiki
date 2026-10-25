package publicsite

import (
	"context"
	"encoding/json"
	"html"

	"nice-wiki/libs/platform"
)

type HandleAskShellInput struct {
	Locale    string
	DataEpoch string
}

type askPage struct {
	shell
	Tree []treeNode
}

// HandleAskShell 独立问答页：页面外壳（导航、页头、页脚）加一块问答区，与文档页右下角的问答面板共用同一个会话。
// 公开问答没开放时给出说明而不是空页面。
func HandleAskShell(ctx context.Context, op platform.OperationContext, in HandleAskShellInput) (string, error) {
	navigation, err := BuildNavigation(ctx, op, BuildNavigationInput{in.Locale, in.DataEpoch, 10000})
	if err != nil {
		return "", err
	}
	tree, err := buildTree(navigation.Tree, "")
	if err != nil {
		return "", err
	}
	meta := loadSiteMeta(ctx)
	footerHTML, sourceURL := loadFooter(ctx, op)
	catalog := loadUIStrings(ctx, in.Locale)
	title := meta.pageTitle(uiText(in.Locale, catalog, "AI 问答"))
	page := askPage{shell: newShell(ComposeHTMLInput{
		Metadata: "<title>" + html.EscapeString(title) + "</title><meta name=\"robots\" content=\"noindex\">" + meta.headTags(title, "", "", ""),
		Locale:   in.Locale, Announcement: loadAnnouncement(ctx, op), Languages: languageLinks(ctx, "/"+in.Locale+"/ask", in.Locale), Footer: footerHTML, SourceCodeURL: sourceURL,
		SiteName: meta.Name, LogoURL: meta.Logo.url(), ThemeColor: meta.ThemeColor, AIEnabled: qaAvailable(ctx, op), AskURL: "/" + in.Locale + "/ask", UIStrings: catalog,
	}, "ask"), Tree: tree}
	// 引导来自同一份公开导航，不生成额外模型调用。
	topics := []map[string]string{}
	var collect func([]treeNode)
	collect = func(nodes []treeNode) {
		for _, node := range nodes {
			if len(topics) >= 4 {
				return
			}
			if node.URL != "" {
				topics = append(topics, map[string]string{"title": node.Title, "href": node.URL})
			}
			collect(node.Children)
		}
	}
	collect(tree)
	var config map[string]any
	_ = json.Unmarshal([]byte(page.Config), &config)
	config["welcome"] = map[string]any{"name": page.SiteName, "description": meta.Description, "topics": topics}
	raw, _ := json.Marshal(config)
	page.Config = string(raw)
	page.HasNav = len(tree) > 0
	return render("ask", page)
}
