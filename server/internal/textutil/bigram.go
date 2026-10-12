package textutil

import (
	"strings"
	"unicode"
)

func Tokens(value string) []string {
	value = Normalize(value)
	result := []string{}
	word := []rune{}
	asian := []rune{}
	flushAsian := func() {
		if len(asian) == 1 {
			result = append(result, string(asian))
		}
		for i := 1; i < len(asian); i++ {
			result = append(result, string(asian[i-1:i+1]))
		}
		asian = nil
	}
	flushWord := func() {
		if len(word) > 0 {
			result = append(result, string(word))
			word = nil
		}
	}
	for _, r := range value {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			flushWord()
			asian = append(asian, r)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			flushAsian()
			word = append(word, r)
		} else {
			flushWord()
			flushAsian()
		}
	}
	flushWord()
	flushAsian()
	return result
}
func KeywordText(value string) string { return strings.Join(Tokens(value), " ") }
