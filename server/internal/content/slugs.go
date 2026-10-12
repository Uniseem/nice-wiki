package content

import (
	"regexp"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ValidateSlug(slug string) error {
	if len(slug) > 120 || !slugPattern.MatchString(slug) {
		return invalid("路径名只允许小写英文、数字及单个连字符")
	}
	switch slug {
	case "admin", "api", "assets", "static", "ask", "llms", "sitemap", "robots":
		return invalid("路径名属于系统保留名称")
	}
	return nil
}
func PathFor(id string, files map[string]File) (string, error) {
	parts := []string{}
	seen := map[string]bool{}
	for id != "" {
		if seen[id] {
			return "", invalid("文档父子关系形成循环")
		}
		seen[id] = true
		value, ok := files[id]
		if !ok {
			return "", invalid("文档父节点不存在")
		}
		parts = append([]string{value.Meta.Slug}, parts...)
		id = value.Meta.ParentID
	}
	return strings.Join(parts, "/"), nil
}
