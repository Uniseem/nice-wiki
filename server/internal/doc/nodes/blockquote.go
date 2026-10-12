package nodes

type blockquote struct{}

func init() { Register("blockquote", blockquote{}) }
func (blockquote) Render(n Node, c Context) (string, error) {
	if c.Format == HTML {
		return Wrap(n, c, "blockquote", "nw-blockquote")
	}
	value, err := Children(n, c)
	if err != nil {
		return "", err
	}
	if c.Format == Markdown {
		return PrefixLines(value, "> ") + "\n", nil
	}
	return value, nil
}
func (blockquote) Translate(n Node) []string { return nil }
