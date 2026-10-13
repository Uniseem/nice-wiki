package translate

import (
	"errors"
	"fmt"
	"html"
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

// DecodeInline 只接受完整有序的槽位，防止模型把格式标签写进正文。
func DecodeInline(text string, count int) ([]string, bool) {
	result := []string{}
	rest := text
	for i := 0; i < count; i++ {
		start, end := fmt.Sprintf("<nw-%d>", i), fmt.Sprintf("</nw-%d>", i)
		if !strings.HasPrefix(rest, start) {
			return nil, false
		}
		rest = strings.TrimPrefix(rest, start)
		at := strings.Index(rest, end)
		if at < 0 || placeholder.MatchString(rest[:at]) {
			return nil, false
		}
		result = append(result, html.UnescapeString(rest[:at]))
		rest = rest[at+len(end):]
	}
	return result, rest == ""
}

func compatibleInline(source, translated string, legacy bool) bool {
	count := len(placeholder.FindAllString(source, -1)) / 2
	if count == 0 {
		return ValidateInline(source, translated) == nil
	}
	if _, ok := DecodeInline(translated, count); ok {
		return true
	}
	return legacy && count == 1 && !placeholder.MatchString(translated)
}
