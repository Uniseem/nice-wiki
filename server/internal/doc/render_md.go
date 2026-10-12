package doc

import "nice-wiki/internal/doc/nodes"

func RenderMarkdown(document Document, options RenderOptions) (string, error) {
	return render(document, options, nodes.Markdown)
}
