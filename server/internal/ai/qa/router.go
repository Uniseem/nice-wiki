package qa

import (
	"errors"
	"strings"
	"unicode/utf8"
)

type Route struct {
	Question     string
	Subquestions []string
	Risk         string
	Complete     bool
}

func ValidateQuestion(value string, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || len([]rune(value)) > maximum {
		return "", errors.New("问题为空、编码无效或超过长度上限")
	}
	return value, nil
}
