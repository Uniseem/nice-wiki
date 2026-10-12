package nodes

type divider struct{}

func init() { Register("horizontalRule", divider{}) }
func (divider) Render(n Node, c Context) (string, error) {
	if c.Format == HTML {
		return "<hr" + Anchor(n) + ` class="nw-divider">`, nil
	}
	if c.Format == Markdown {
		return "\n---\n\n", nil
	}
	return "\n", nil
}
func (divider) Translate(n Node) []string { return nil }
