package doc

import "nice-wiki/internal/doc/nodes"

func RenderText(document Document) (string, error) {
	return render(document, RenderOptions{}, nodes.Text)
}
