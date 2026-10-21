package publicsite

import (
	"context"
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
		Locale:   in.Locale, Announcement: loadAnnouncement(ctx, op), Footer: footerHTML, SourceCodeURL: sourceURL,
		SiteName: meta.Name, LogoURL: meta.Logo.url(), ThemeColor: meta.ThemeColor, AIEnabled: qaAvailable(ctx, op), AskURL: "/" + in.Locale + "/ask", UIStrings: catalog,
	}, "ask"), Tree: tree}
	page.HasNav = len(tree) > 0
	return render("ask", page)
}
