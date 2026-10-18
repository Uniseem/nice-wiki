package publicsite

import (
	"context"
	"encoding/json"
	"html"
	"nice-wiki/libs/platform"
	"strings"
)

type AlternateLocale struct {
	Locale string `json:"locale"`
	URL    string `json:"url"`
}
type BuildSEOMetadataInput struct {
	Title        string
	Description  string
	CanonicalURL string
	Alternates   []AlternateLocale
	ShareImage   string
	Breadcrumbs  []Breadcrumb
	MarkdownURL  string
}
type BuildSEOMetadataResult struct{ MetaHTML string }

// BuildSEOMetadata 只消费已选定的公开元数据，不调用模型；所有属性在输出点转义。
func BuildSEOMetadata(ctx context.Context, op platform.OperationContext, in BuildSEOMetadataInput) (BuildSEOMetadataResult, error) {
	var out strings.Builder
	out.WriteString("<title>" + html.EscapeString(in.Title) + "</title>")
	for _, meta := range []struct{ name, value string }{{"description", in.Description}, {"og:type", "article"}, {"og:title", in.Title}, {"og:description", in.Description}, {"og:url", in.CanonicalURL}, {"og:image", in.ShareImage}} {
		if meta.value == "" {
			continue
		}
		attribute := "name"
		if strings.HasPrefix(meta.name, "og:") {
			attribute = "property"
		}
		out.WriteString("<meta " + attribute + "=\"" + meta.name + "\" content=\"" + html.EscapeString(meta.value) + "\">")
	}
	out.WriteString("<link rel=\"canonical\" href=\"" + html.EscapeString(in.CanonicalURL) + "\">")
	for _, alternate := range in.Alternates {
		out.WriteString("<link rel=\"alternate\" hreflang=\"" + html.EscapeString(alternate.Locale) + "\" href=\"" + html.EscapeString(alternate.URL) + "\">")
	}
	if in.MarkdownURL != "" {
		out.WriteString("<link rel=\"alternate\" type=\"text/markdown\" href=\"" + html.EscapeString(in.MarkdownURL) + "\">")
	}
	items := make([]map[string]any, 0, len(in.Breadcrumbs))
	for i, crumb := range in.Breadcrumbs {
		items = append(items, map[string]any{"@type": "ListItem", "position": i + 1, "name": crumb.Label, "item": crumb.URL})
	}
	structured, err := json.Marshal(map[string]any{"@context": "https://schema.org", "@type": "BreadcrumbList", "itemListElement": items})
	if err != nil {
		return BuildSEOMetadataResult{}, platform.NewError(platform.CodeInternal, false, "SEO_ENCODING_FAILED")
	}
	out.WriteString("<script type=\"application/ld+json\">")
	out.Write(structured)
	out.WriteString("</script>")
	return BuildSEOMetadataResult{out.String()}, nil
}
