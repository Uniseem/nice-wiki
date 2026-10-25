package publicsite

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"net/url"
	"strings"

	"nice-wiki/libs/platform"
)

type HandlePageInput struct {
	Locale    string
	Slug      string
	DataEpoch string
}
type HandlePageResult struct {
	Status      int
	HTML        string
	ETag        string
	Location    string
	ContentType string // 空表示 HTML
}

// HandlePage 每次请求都读权威：导航树确认已发布，再取精确发布正文渲染成完整 HTML。
func HandlePage(ctx context.Context, op platform.OperationContext, in HandlePageInput) (HandlePageResult, error) {
	// /语言/路径.md 是同一篇文档的 Markdown 版本。
	markdown := strings.HasSuffix(in.Slug, ".md")
	if markdown {
		in.Slug = strings.TrimSuffix(in.Slug, ".md")
	}
	page, err := renderPage(ctx, op, in.Locale, in.DataEpoch, markdown, func(nodes []NavigationNode) (NavigationNode, bool) {
		segments := strings.Split(in.Slug, "/")
		for i := range segments {
			segments[i] = url.PathEscape(segments[i])
		}
		suffix := "/" + in.Locale + "/" + strings.Join(segments, "/")
		for _, node := range nodes {
			if strings.HasSuffix(node.CanonicalPath, suffix) {
				return node, true
			}
		}
		return NavigationNode{}, false
	})
	if err != nil {
		var failure *platform.Error
		if errors.As(err, &failure) && failure.Code == platform.CodeNotFound {
			if moved, resolveErr := resolveMoved(ctx, in); resolveErr == nil && moved.Status != 0 {
				return moved, nil
			}
		}
		return HandlePageResult{}, err
	}
	result := HandlePageResult{Status: 200, HTML: page, ETag: "\"" + string(platform.HashBytes([]byte(page))) + "\""}
	if markdown {
		result.ContentType = "text/markdown; charset=utf-8"
	}
	return result, nil
}

// resolveMoved 页面不在公开树里时，向 content 查旧路径：改名/移动得到 301，删除得到 410。
func resolveMoved(ctx context.Context, in HandlePageInput) (HandlePageResult, error) {
	directory, err := directoryFrom(ctx)
	if err != nil {
		return HandlePageResult{}, err
	}
	raw, err := directory.Content.ResolvePublicPath(ctx, url.Values{"path": []string{"/" + in.Slug}})
	if err != nil {
		return HandlePageResult{}, err
	}
	var result struct {
		State         string  `json:"state"`
		CanonicalPath *string `json:"canonicalPath"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return HandlePageResult{}, dependency("旧路径决定不可读")
	}
	switch result.State {
	case "redirect", "found":
		if result.CanonicalPath != nil {
			site, siteErr := readSiteSnapshot(ctx, directory.SiteConfig)
			if siteErr != nil {
				return HandlePageResult{}, siteErr
			}
			href, hrefErr := relativeLink(ctx, platform.OperationContext{}, site.publicBaseURL, in.Locale, *result.CanonicalPath, "")
			if hrefErr != nil {
				return HandlePageResult{}, hrefErr
			}
			return HandlePageResult{Status: 301, Location: href}, nil
		}
	case "gone":
		return HandlePageResult{Status: 410, HTML: RenderStatusPage(in.Locale, 410, "页面已删除", "这篇文档已经被删除。")}, nil
	}
	return HandlePageResult{}, nil
}

type HandleHomeInput struct {
	Locale    string
	DataEpoch string
}
type HandleHomeResult struct {
	Status   int
	HTML     string
	Location string
}

type homePage struct {
	shell
	WikiURL string
}

// HandleHome 渲染固定一屏的欢迎页，提供 Agent 与第一篇已发布文档的入口。
func HandleHome(ctx context.Context, op platform.OperationContext, in HandleHomeInput) (HandleHomeResult, error) {
	directory, err := directoryFrom(ctx)
	if err != nil {
		return HandleHomeResult{}, err
	}
	navigation, err := BuildNavigation(ctx, op, BuildNavigationInput{in.Locale, in.DataEpoch, 10000})
	if err != nil {
		return HandleHomeResult{}, err
	}
	nodes, err := navigationOrder(navigation.Tree)
	if err != nil {
		return HandleHomeResult{}, err
	}
	raw, err := directory.SiteConfig.ReadSiteConfig(ctx)
	if err != nil {
		return HandleHomeResult{}, err
	}
	var config struct {
		Config struct {
			Name string `json:"name"`
			Home struct {
				HeaderLinks []struct {
					Text string `json:"text"`
					URL  string `json:"url"`
				} `json:"headerLinks"`
			} `json:"home"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return HandleHomeResult{}, dependency("站点首页配置不可读")
	}
	home := config.Config.Home
	meta := loadSiteMeta(ctx)
	homeBase, _ := siteForSEO(ctx)
	head := "<title>" + html.EscapeString(config.Config.Name) + "</title>"
	if indexing, policyErr := readIndexingPolicy(ctx); policyErr == nil && !indexing {
		head += "<meta name=\"robots\" content=\"noindex,nofollow\">"
	}
	if meta.Description != "" {
		head += "<meta name=\"description\" content=\"" + html.EscapeString(meta.Description) + "\">"
	}
	head += strings.Replace(meta.headTags(config.Config.Name, meta.Description, "", homeBase), "og:type\" content=\"article\"", "og:type\" content=\"website\"", 1)
	footerHTML, sourceURL := loadFooter(ctx, op)
	page := homePage{shell: newShell(ComposeHTMLInput{
		Metadata: head, Locale: in.Locale, Announcement: loadAnnouncement(ctx, op), Languages: languageLinks(ctx, "/"+in.Locale+"/", in.Locale),
		SiteName: meta.Name, LogoURL: meta.Logo.url(), ThemeColor: meta.ThemeColor, Footer: footerHTML, SourceCodeURL: sourceURL,
		UIStrings: loadUIStrings(ctx, in.Locale), AIEnabled: qaAvailable(ctx, op), AskURL: "/" + in.Locale + "/ask",
	}, "home")}
	if len(nodes) > 0 {
		page.WikiURL = nodes[0].CanonicalPath
	}
	for _, link := range home.HeaderLinks {
		page.HeaderLinks = append(page.HeaderLinks, Breadcrumb{Label: link.Text, URL: link.URL})
	}
	rendered, err := render("home", page)
	if err != nil {
		return HandleHomeResult{}, err
	}
	return HandleHomeResult{Status: 200, HTML: rendered}, nil
}

func renderPage(ctx context.Context, op platform.OperationContext, locale, epoch string, markdown bool, pick func([]NavigationNode) (NavigationNode, bool)) (string, error) {
	navigation, err := BuildNavigation(ctx, op, BuildNavigationInput{locale, epoch, 10000})
	if err != nil {
		return "", err
	}
	node, found := pick(navigation.Tree)
	if !found {
		return "", platform.NewError(platform.CodeNotFound, false, "NOT_PUBLISHED")
	}
	return renderNode(ctx, op, locale, markdown, navigation, node)
}

// renderNode 用调用方已经建好的公开导航渲染一篇文档；批量渲染（llms-full）时导航只建一次。
func renderNode(ctx context.Context, op platform.OperationContext, locale string, markdown bool, navigation BuildNavigationResult, node NavigationNode) (string, error) {
	page, err := renderNodeDetail(ctx, op, locale, markdown, navigation, node)
	return page.text, err
}

// renderedNode 除了渲染结果，还记下这篇是否含站内链接、是否用了译文，供 llms-full 判断缓存何时失效。
type renderedNode struct {
	text       string
	linked     bool
	translated bool
}

func renderNodeDetail(ctx context.Context, op platform.OperationContext, locale string, markdown bool, navigation BuildNavigationResult, node NavigationNode) (renderedNode, error) {
	bundle, err := LoadPublishedBundle(ctx, op, LoadPublishedBundleInput{Publication: node.Publication, Locale: locale})
	if err != nil {
		return renderedNode{}, err
	}
	media, err := ResolvePublicMedia(ctx, op, ResolvePublicMediaInput{MediaRefs: bundle.MediaRefs, Publication: node.Publication})
	if err != nil {
		return renderedNode{}, err
	}
	// 站内链接按当前语言的公开路径改写；目标未发布、已撤回或锚点不在时由渲染器显示为“暂不可用”。
	links := map[string]string{}
	targets := collectInternalLinks(bundle.Document)
	if len(targets) > 0 {
		resolved, linkErr := resolveInternalLinks(ctx, navigation, targets)
		if linkErr != nil {
			return renderedNode{}, linkErr
		}
		links = resolved.Rewritten
	}
	if markdown {
		text, mdErr := RenderTiptapMarkdown(ctx, op, RenderTiptapMarkdownInput{Document: bundle.Document, Locale: locale, Assets: markdownAssets(media.Assets), Links: links})
		if mdErr != nil {
			return renderedNode{}, mdErr
		}
		return renderedNode{"# " + bundle.Title + "\n\n" + text.Markdown, len(targets) > 0, locale != bundle.SourceLanguage}, nil
	}
	body, err := RenderTiptapHTML(ctx, op, RenderTiptapHTMLInput{Document: bundle.Document, Locale: locale, Assets: media.Assets, Links: links})
	if err != nil {
		return renderedNode{}, err
	}
	var headings []Heading
	var walk func([]TiptapNode)
	walk = func(nodes []TiptapNode) {
		for _, n := range nodes {
			if n.Type == "heading" {
				level, _ := n.Attrs["level"].(float64)
				id := attrString(n.Attrs, "blockId")
				if id != "" && level >= 1 && level <= 4 {
					headings = append(headings, Heading{Level: int(level), Text: plainText(n), Anchor: id, BlockID: id})
				}
			}
			walk(n.Content)
		}
	}
	walk(bundle.Document.Content)
	toc, err := BuildTOC(ctx, op, BuildTOCInput{Headings: headings, MaxDepth: 4})
	if err != nil {
		return renderedNode{}, err
	}
	meta := loadSiteMeta(ctx)
	fullTitle := meta.pageTitle(bundle.Title)
	description := bundle.Description
	if description == "" {
		description = meta.Description // 文档没有摘要时用站点描述
	}
	metadata := "<title>" + html.EscapeString(fullTitle) + "</title>"
	if indexing, policyErr := readIndexingPolicy(ctx); policyErr == nil && !indexing {
		metadata += "<meta name=\"robots\" content=\"noindex,nofollow\">" // 站点关闭收录：页面本身也声明不要索引
	}
	if description != "" {
		metadata += "<meta name=\"description\" content=\"" + html.EscapeString(description) + "\">"
	}
	languages := languageLinks(ctx, node.CanonicalPath, locale)
	if site, siteErr := siteForSEO(ctx); siteErr == nil {
		// 规范地址与各语言互链，帮搜索引擎把译文当作同一页的不同语言版本。
		metadata += "<link rel=\"canonical\" href=\"" + html.EscapeString(site+node.CanonicalPath) + "\">"
		metadata += meta.headTags(fullTitle, description, site+node.CanonicalPath, site)
		for _, link := range languages {
			metadata += "<link rel=\"alternate\" hreflang=\"" + html.EscapeString(link.Tag) + "\" href=\"" + html.EscapeString(site+link.Href) + "\">"
		}
	}
	crumbs := []Breadcrumb{}
	if built, crumbErr := BuildBreadcrumbs(ctx, op, BuildBreadcrumbsInput{DocumentID: node.DocumentID, Tree: navigation.Tree, HomeURL: "/" + locale + "/"}); crumbErr == nil {
		crumbs = built.Items
	}
	// 上一篇/下一篇按导航树顺序取相邻文档。
	previous, next := []Breadcrumb{}, []Breadcrumb{}
	for i, item := range navigation.Tree {
		if item.DocumentID != node.DocumentID {
			continue
		}
		if i > 0 {
			previous = append(previous, Breadcrumb{navigation.Tree[i-1].Title, navigation.Tree[i-1].CanonicalPath})
		}
		if i+1 < len(navigation.Tree) {
			next = append(next, Breadcrumb{navigation.Tree[i+1].Title, navigation.Tree[i+1].CanonicalPath})
		}
	}
	footerHTML, sourceURL := loadFooter(ctx, op)
	composed, err := ComposeHTML(ctx, op, ComposeHTMLInput{
		SafeHTML:      body.SafeHTML,
		Title:         bundle.Title,
		TOC:           toc.Items,
		Navigation:    navigation.Tree,
		Metadata:      metadata,
		Locale:        locale,
		DocumentID:    node.DocumentID,
		MarkdownURL:   node.CanonicalPath + ".md",
		UpdatedAt:     node.LastPublishedAt,
		Announcement:  loadAnnouncement(ctx, op),
		Breadcrumbs:   crumbs,
		Previous:      previous,
		Next:          next,
		Languages:     languages,
		Untranslated:  len(bundle.UntranslatedBlockIDs) > 0,
		AIEnabled:     qaAvailable(ctx, op),
		AskURL:        "/" + locale + "/ask",
		UIStrings:     loadUIStrings(ctx, locale),
		Footer:        footerHTML,
		SiteName:      meta.Name,
		LogoURL:       meta.Logo.url(),
		ThemeColor:    meta.ThemeColor,
		SourceCodeURL: sourceURL,
	})
	if err != nil {
		return renderedNode{}, err
	}
	return renderedNode{text: composed.HTML}, nil
}

func plainText(node TiptapNode) string {
	var out strings.Builder
	out.WriteString(node.Text)
	for _, child := range node.Content {
		out.WriteString(plainText(child))
	}
	return out.String()
}

func markdownAssets(assets []PublicAsset) []PublicAsset { return assets }
