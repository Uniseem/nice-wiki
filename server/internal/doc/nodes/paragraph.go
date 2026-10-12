package nodes

type paragraph struct{}

func init() { Register("paragraph", paragraph{}); Register("hardBreak", paragraph{}) }
func (paragraph) Render(n Node, c Context) (string, error) {
	if n.Type == "hardBreak" {
		if c.Format == HTML {
			return "<br>", nil
		}
		if c.Format == Markdown {
			return "  \n", nil
		}
		return "\n", nil
	}
	return Wrap(n, c, "p", "nw-paragraph")
}
func (paragraph) Translate(n Node) []string { return OrdinaryTranslation(n) }
