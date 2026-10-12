package nodes

import (
	"strconv"
	"strings"
)

type heading struct{}

func init() { Register("heading", heading{}) }
func (heading) Render(n Node, c Context) (string, error) {
	level := Number(n, "level", 2)
	if c.Format == HTML {
		return Wrap(n, c, "h"+strconv.Itoa(level), "nw-heading")
	}
	value, err := Children(n, c)
	if err != nil {
		return "", err
	}
	if c.Format == Markdown {
		return strings.Repeat("#", level) + " " + value + "\n\n", nil
	}
	return value + "\n", nil
}
func (heading) Translate(n Node) []string { return OrdinaryTranslation(n) }
