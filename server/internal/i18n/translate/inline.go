package translate

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var placeholder = regexp.MustCompile(`<nw-(\d+)>|</nw-(\d+)>`)

type Inline struct {
	Text   string
	Values []string
}

func EncodeInline(values []string) Inline {
	var out strings.Builder
	for i, value := range values {
		fmt.Fprintf(&out, "<nw-%d>%s</nw-%d>", i, value, i)
	}
	return Inline{Text: out.String(), Values: append([]string{}, values...)}
}
func ValidateInline(source, translated string) error {
	expected := placeholder.FindAllString(source, -1)
	actual := placeholder.FindAllString(translated, -1)
	if len(expected) != len(actual) {
		return errors.New("译文行内占位数量不一致")
	}
	for i := range expected {
		if expected[i] != actual[i] {
			return errors.New("译文行内占位顺序不一致")
		}
	}
	return nil
}
