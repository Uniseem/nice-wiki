package doc

import (
	"bytes"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"html"
)

func Highlight(code, language string) (string, error) {
	lexer := lexers.Get(language)
	if lexer == nil {
		return html.EscapeString(code), nil
	}
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", err
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.PreventSurroundingPre(true))
	var output bytes.Buffer
	if err := formatter.Format(&output, styles.Fallback, iterator); err != nil {
		return "", err
	}
	return output.String(), nil
}
