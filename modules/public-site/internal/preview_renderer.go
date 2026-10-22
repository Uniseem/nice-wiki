package publicsite

import (
	"context"
	"encoding/json"
	"html"
	"nice-wiki/libs/platform"
)

// previewNotice 预览页顶部的提示条：提醒这是草稿，访客看不到（按页面语言显示）。
func previewNotice(locale string) string {
	return `<aside class="border-b border-warning/30 bg-warning-soft px-4 py-2 text-center text-sm text-foreground">` + html.EscapeString(uiText(locale, nil, "草稿预览：访客看不到这个页面，发布后才会公开。")) + `</aside>`
}

type RenderPreviewInput struct {
	PreviewID    string
	PreviewToken string
	Snapshot     ValidatePreviewTokenResult
}
type RenderPreviewResult struct{ HTML string }

// RenderPreview 只消费本次已由content验证的草稿快照，受保护资源仍以原grant向media请求。
func RenderPreview(ctx context.Context, op platform.OperationContext, in RenderPreviewInput) (RenderPreviewResult, error) {
	parsed, err := ParseTiptapDocument(ctx, op, ParseTiptapDocumentInput{in.Snapshot.Body, publishedBodyLimit})
	if err != nil {
		return RenderPreviewResult{}, err
	}
	assets := []PublicAsset{}
	if len(parsed.MediaRefs) > 0 {
		directory, err := directoryFrom(ctx)
		if err != nil {
			return RenderPreviewResult{}, err
		}
		if directory.Media == nil {
			return RenderPreviewResult{}, dependency("受保护预览资源未装配")
		}
		raw, err := directory.Media.BuildDeliveryManifest(ctx, map[string]any{"assets": parsed.MediaRefs, "publishedReference": nil, "previewId": in.PreviewID, "previewToken": in.PreviewToken})
		if err != nil {
			return RenderPreviewResult{}, err
		}
		var manifest struct {
			Manifest *struct {
				PreviewID *string `json:"previewId"`
				Items     []struct {
					Asset       AssetVersionRef `json:"asset"`
					URL         string          `json:"url"`
					CachePolicy string          `json:"cachePolicy"`
				} `json:"items"`
			} `json:"manifest"`
		}
		if json.Unmarshal(raw, &manifest) != nil || manifest.Manifest == nil || manifest.Manifest.PreviewID == nil || *manifest.Manifest.PreviewID != in.PreviewID {
			return RenderPreviewResult{}, dependency("受保护资源来源无法核验")
		}
		requested := map[AssetVersionRef]bool{}
		for _, asset := range parsed.MediaRefs {
			requested[asset] = true
		}
		for _, item := range manifest.Manifest.Items {
			if !requested[item.Asset] || item.CachePolicy != "private_no_store" || !safeLink(item.URL) {
				return RenderPreviewResult{}, platform.NewError(platform.CodeIntegrityError, false, "PREVIEW_MANIFEST_MISMATCH")
			}
			assets = append(assets, PublicAsset{Asset: item.Asset, URL: item.URL, Srcset: []ImageSource{}})
		}
	}
	targets := collectInternalLinks(parsed.Document)
	links := map[string]string{}
	if len(targets) > 0 {
		// 公开导航是访客可见的数据，用本服务身份读：预览请求带着管理员委托，content 的公开树只接受服务身份。
		resolved, err := RewriteInternalLinks(platform.WithPortalProof(ctx, ""), op, RewriteInternalLinksInput{targets, in.Snapshot.Metadata.SourceLanguage, string(op.DataEpoch)})
		if err != nil {
			return RenderPreviewResult{}, err
		}
		links = resolved.Rewritten
	}
	rendered, err := RenderTiptapHTML(ctx, op, RenderTiptapHTMLInput{parsed.Document, in.Snapshot.Metadata.SourceLanguage, assets, links})
	if err != nil {
		return RenderPreviewResult{}, err
	}
	toc, err := BuildTOC(ctx, op, BuildTOCInput{parsed.Headings, 4})
	if err != nil {
		return RenderPreviewResult{}, err
	}
	// 页头的站名、站标和主题色与公开页一致，预览看到的就是发布后的样子。
	meta := loadSiteMeta(ctx)
	page, err := ComposeHTML(ctx, op, ComposeHTMLInput{
		SafeHTML: rendered.SafeHTML, Title: in.Snapshot.Metadata.Title, TOC: toc.Items, Navigation: []NavigationNode{},
		Metadata: "<title>" + html.EscapeString(meta.pageTitle(in.Snapshot.Metadata.Title)) + "</title><meta name=\"robots\" content=\"noindex,nofollow\">",
		Locale:   in.Snapshot.Metadata.SourceLanguage, DocumentID: in.Snapshot.Metadata.DocumentID, Announcement: previewNotice(in.Snapshot.Metadata.SourceLanguage),
		SiteName: meta.Name, LogoURL: meta.Logo.url(), ThemeColor: meta.ThemeColor,
	})
	if err != nil {
		return RenderPreviewResult{}, err
	}
	return RenderPreviewResult{page.HTML}, nil
}

func collectInternalLinks(document TiptapDocument) []InternalLinkTarget {
	targets := make([]InternalLinkTarget, 0)
	seen := map[string]bool{}
	var visit func(TiptapNode)
	visit = func(node TiptapNode) {
		for _, mark := range node.Marks {
			if mark.Type != "link" {
				continue
			}
			target, _ := mark.Attrs["target"].(map[string]any)
			if attrString(target, "kind") != "internal" {
				continue
			}
			id := attrString(target, "documentId")
			block := attrString(target, "blockId")
			key := id + "#" + block
			if seen[key] {
				continue
			}
			seen[key] = true
			var blockID *string
			if block != "" {
				value := block
				blockID = &value
			}
			targets = append(targets, InternalLinkTarget{id, blockID})
		}
		for _, child := range node.Content {
			visit(child)
		}
	}
	for _, node := range document.Content {
		visit(node)
	}
	return targets
}
