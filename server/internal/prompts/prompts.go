package prompts

import (
	"embed"
	"errors"
	"strings"
)

//go:embed *.md
var files embed.FS

func Get(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "/\\") {
		return "", errors.New("提示词名称无效")
	}
	data, err := files.ReadFile(name + ".md")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
