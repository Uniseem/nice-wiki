package platform

import (
	_ "embed"
	"strings"
)

// 默认书页标志与前端共用一个 SVG，页头和标签页不会出现不同图形。
//
//go:embed web/ui/mark.svg
var defaultSiteMark string

// DefaultSiteMarkSVG 按调用方已校验的主题色输出默认标志。
func DefaultSiteMarkSVG(color string) string {
	return strings.ReplaceAll(defaultSiteMark, "currentColor", color)
}
