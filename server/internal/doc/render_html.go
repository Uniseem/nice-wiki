package doc

import (
	"html"
	"nice-wiki/internal/doc/nodes"
)

func RenderHTML(document Document, options RenderOptions) (string, error) {
	return render(document, options, nodes.HTML)
}
func render(document Document, options RenderOptions, format nodes.Format) (string, error) {
	if err := Validate(document); err != nil {
		return "", err
	}
	var visit func(Node) (string, error)
	visit = func(node Node) (string, error) {
		if node.Type == "text" {
			value := node.Text
			switch format {
			case nodes.HTML:
				value = html.EscapeString(value)
			case nodes.Markdown:
				value = nodes.EscapeMarkdown(value)
			}
			return renderMarks(value, node.Marks, options, format), nil
		}
		context := nodes.Context{Format: format, Language: options.Language, Render: visit, Highlight: Highlight, AssetURL: options.AssetURL}
		if node.Type == "doc" {
			return nodes.Children(node, context)
		}
		renderer, exists := nodes.Lookup(node.Type)
		if !exists {
			return "", nodes.Missing(node.Type)
		}
		return renderer.Render(node, context)
	}
	return visit(document.Content)
}
