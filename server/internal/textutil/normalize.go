package textutil

import (
	"golang.org/x/text/unicode/norm"
	"strings"
	"unicode"
)

func Normalize(value string) string {
	value = norm.NFKC.String(value)
	return strings.ToLower(strings.Join(strings.FieldsFunc(value, unicode.IsSpace), " "))
}
