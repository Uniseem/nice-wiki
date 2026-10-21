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
		return HandlePageResult{Status: 410, HTML: "<!doctype html><html lang=\"" + html.EscapeString(in.Locale) + "\"><meta charset=\"utf-8\"><title>页面已删除</title><body><h1>该页面已被删除</h1></body></html>"}, nil
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

// HandleHome 用站点配置里的首页文案渲染一屏首页，按钮指向导航树里的第一篇文档。
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
				MainTitle      string `json:"mainTitle"`
				Subtitle       string `json:"subtitle"`
				BackgroundText string `json:"backgroundText"`
				OverlayPercent int64  `json:"overlayPercent"`
				TextColor      string `json:"textColor"`
				Background     struct {
					Kind  string `json:"kind"`
					Color string `json:"color"`
				} `json:"background"`
				Buttons []struct {
					Text   string `json:"text"`
					Target struct {
						Kind       string `json:"kind"`
						URL        string `json:"url"`
						DocumentID string `json:"documentId"`
					} `json:"target"`
				} `json:"buttons"`
			} `json:"home"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return HandleHomeResult{}, dependency("站点首页配置不可读")
	}
	home := config.Config.Home
	var labels, urls []string
	for _, button := range home.Buttons {
		target := ""
		switch button.Target.Kind {
		case "external":
			target = button.Target.URL
		case "document":
			for _, node := range nodes {
				if node.DocumentID == button.Target.DocumentID {
					target = node.CanonicalPath
				}
			}
		case "qa":
			target = "/" + in.Locale + "/ask"
		}
		if target != "" && len(labels) < 3 {
			labels, urls = append(labels, button.Text), append(urls, target)
		}
	}
	if len(labels) == 0 && len(nodes) > 0 {
		labels, urls = []string{"开始阅读"}, []string{nodes[0].CanonicalPath}
	}
	hero, err := RenderHero(ctx, op, RenderHeroInput{Title: home.MainTitle, Subtitle: home.Subtitle, BackgroundKind: "solid", OverlayPercent: home.OverlayPercent,
		ButtonLabels: labels, ButtonURLs: urls, BackgroundText: home.BackgroundText, TextColor: home.TextColor})
	if err != nil {
		return HandleHomeResult{}, err
	}
	if home.Background.Kind == "color" && len(home.Background.Color) == 7 && home.Background.Color[0] == '#' {
		hero.HTML = strings.Replace(hero.HTML, "<main class=\"hero ", "<main style=\"background:"+html.EscapeString(home.Background.Color)+"\" class=\"hero ", 1)
	}
	siteMeta := loadSiteMeta(ctx)
	homeBase, _ := siteForSEO(ctx)
	homeHead := ""
	if indexing, policyErr := readIndexingPolicy(ctx); policyErr == nil && !indexing {
		homeHead += "<meta name=\"robots\" content=\"noindex,nofollow\">"
	}
	if siteMeta.Description != "" {
		homeHead = "<meta name=\"description\" content=\"" + html.EscapeString(siteMeta.Description) + "\">"
	}
	homeHead += strings.Replace(siteMeta.headTags(config.Config.Name, siteMeta.Description, "", homeBase), "og:type\" content=\"article\"", "og:type\" content=\"website\"", 1)
	page := "<!doctype html><html lang=\"" + html.EscapeString(in.Locale) + "\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>" + html.EscapeString(config.Config.Name) + "</title>" + homeHead + "<style>body{margin:0;font:16px/1.6 system-ui}" + announcementCSS + ".hero a{display:inline-block;margin:.5rem;padding:.6rem 1.4rem;border:1px solid currentColor;border-radius:999px;color:inherit;text-decoration:none}</style></head><body>" + loadAnnouncement(ctx, op) + hero.HTML + "</body></html>"
	return HandleHomeResult{Status: 200, HTML: page}, nil
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
	bundle, err := LoadPublishedBundle(ctx, op, LoadPublishedBundleInput{Publication: node.Publication, Locale: locale})
	if err != nil {
		return "", err
	}
	media, err := ResolvePublicMedia(ctx, op, ResolvePublicMediaInput{MediaRefs: bundle.MediaRefs, Publication: node.Publication})
	if err != nil {
		return "", err
	}
	if markdown {
		text, mdErr := RenderTiptapMarkdown(ctx, op, RenderTiptapMarkdownInput{Document: bundle.Document, Locale: locale, Assets: markdownAssets(media.Assets)})
		if mdErr != nil {
			return "", mdErr
		}
		return "# " + bundle.Title + "\n\n" + text.Markdown, nil
	}
	body, err := RenderTiptapHTML(ctx, op, RenderTiptapHTMLInput{Document: bundle.Document, Locale: locale, Assets: media.Assets})
	if err != nil {
		return "", err
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
		return "", err
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
		Announcement:  loadAnnouncement(ctx, op),
		Breadcrumbs:   crumbs,
		Previous:      previous,
		Next:          next,
		Languages:     languages,
		AIEnabled:     qaAvailable(ctx, op),
		AskURL:        "/" + locale + "/ask",
		UIStrings:     loadUIStrings(ctx, locale),
		Footer:        footerHTML,
		SiteName:      meta.Name,
		LogoURL:       meta.Logo.url(),
		SourceCodeURL: sourceURL,
	})
	if err != nil {
		return "", err
	}
	return composed.HTML, nil
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
