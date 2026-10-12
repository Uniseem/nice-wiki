package nodes

import (
	"html"
	"regexp"
)

type codeblock struct{}

func init() { Register("codeBlock", codeblock{}) }
func (codeblock) Render(n Node, c Context) (string, error) {
	value := InlineText(n)
	language := String(n, "language")
	if !regexp.MustCompile(`^[A-Za-z0-9_+-]{0,32}$`).MatchString(language) {
		language = ""
	}
	if c.Format == HTML {
		rendered := html.EscapeString(value)
		if c.Highlight != nil {
			var err error
			rendered, err = c.Highlight(value, language)
			if err != nil {
				return "", err
			}
		}
		return "<pre" + Anchor(n) + ` class="nw-code chroma"><code data-language="` + language + `">` + rendered + "</code></pre>", nil
	}
	if c.Format == Markdown {
		fence := Fence(value)
		return fence + language + "\n" + value + "\n" + fence + "\n\n", nil
	}
	return value + "\n", nil
}
func (codeblock) Translate(n Node) []string { return nil }
