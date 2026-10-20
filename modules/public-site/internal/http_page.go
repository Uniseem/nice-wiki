package publicsite

import (
	"context"
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
	Status int
	HTML   string
	ETag   string
}

// HandlePage 每次请求都读权威：导航树确认已发布，再取精确发布正文渲染成完整 HTML。
func HandlePage(ctx context.Context, op platform.OperationContext, in HandlePageInput) (HandlePageResult, error) {
	page, err := renderPage(ctx, op, in.Locale, in.DataEpoch, func(nodes []NavigationNode) (NavigationNode, bool) {
		suffix := "/" + in.Locale + "/" + url.PathEscape(in.Slug)
		for _, node := range nodes {
			if strings.HasSuffix(node.CanonicalPath, suffix) {
				return node, true
			}
		}
		return NavigationNode{}, false
	})
	if err != nil {
		return HandlePageResult{}, err
	}
	return HandlePageResult{200, page, "\"" + string(platform.HashBytes([]byte(page))) + "\""}, nil
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

// HandleHome 暂以导航树第一篇文档作为首页；没有已发布文档时给出空站提示。
func HandleHome(ctx context.Context, op platform.OperationContext, in HandleHomeInput) (HandleHomeResult, error) {
	navigation, err := BuildNavigation(ctx, op, BuildNavigationInput{in.Locale, in.DataEpoch, 10000})
	if err != nil {
		return HandleHomeResult{}, err
	}
	nodes, err := navigationOrder(navigation.Tree)
	if err != nil {
		return HandleHomeResult{}, err
	}
	if len(nodes) == 0 {
		return HandleHomeResult{200, "<!doctype html><html lang=\"" + html.EscapeString(in.Locale) + "\"><meta charset=\"utf-8\"><title>文档</title><body><p>还没有已发布的文档。</p></body></html>", ""}, nil
	}
	return HandleHomeResult{302, "", nodes[0].CanonicalPath}, nil
}

func renderPage(ctx context.Context, op platform.OperationContext, locale, epoch string, pick func([]NavigationNode) (NavigationNode, bool)) (string, error) {
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
	metadata := "<title>" + html.EscapeString(bundle.Title) + "</title>"
	if bundle.Description != "" {
		metadata += "<meta name=\"description\" content=\"" + html.EscapeString(bundle.Description) + "\">"
	}
	composed, err := ComposeHTML(ctx, op, ComposeHTMLInput{
		SafeHTML:   body.SafeHTML,
		Title:      bundle.Title,
		TOC:        toc.Items,
		Navigation: navigation.Tree,
		Metadata:   metadata,
		Locale:     locale,
		DocumentID: node.DocumentID,
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
