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

// HandleAskShell 独立问答页：页面外壳（导航、页头、页脚）加一句引导，问答窗口自动打开，与文档页右下角浮窗共用同一个会话。
// 公开问答没开放时给出说明而不是空页面。
func HandleAskShell(ctx context.Context, op platform.OperationContext, in HandleAskShellInput) (string, error) {
	navigation, err := BuildNavigation(ctx, op, BuildNavigationInput{in.Locale, in.DataEpoch, 10000})
	if err != nil {
		return "", err
	}
	meta := loadSiteMeta(ctx)
	footerHTML, sourceURL := loadFooter(ctx, op)
	enabled := qaAvailable(ctx, op)
	body := "<p>公开问答暂未开放。</p>"
	if enabled {
		body = "<p>向这个站点的文档提问，回答会附上来源。</p><script>addEventListener('load',function(){var b=document.querySelector('.ai-fab');if(b)b.click()})</script>"
	}
	title := meta.pageTitle("AI 问答")
	metadata := "<title>" + html.EscapeString(title) + "</title><meta name=\"robots\" content=\"noindex\">"
	composed, err := ComposeHTML(ctx, op, ComposeHTMLInput{
		SafeHTML:      body,
		Title:         "AI 问答",
		Navigation:    navigation.Tree,
		Metadata:      metadata,
		Locale:        in.Locale,
		Breadcrumbs:   []Breadcrumb{},
		Previous:      []Breadcrumb{},
		Next:          []Breadcrumb{},
		Announcement:  loadAnnouncement(ctx, op),
		Footer:        footerHTML,
		SourceCodeURL: sourceURL,
		SiteName:      meta.Name,
		LogoURL:       meta.Logo.url(),
		AIEnabled:     enabled,
		AskURL:        "/" + in.Locale + "/ask",
		UIStrings:     loadUIStrings(ctx, in.Locale),
	})
	return composed.HTML, err
}
