package platform

import (
	_ "embed"
	"strings"
)

// 默认书页标志与前端共用一个 SVG，页头和标签页不会出现不同图形。
// 图形取解构主义：一页纸竖向切开，左片只留描边、转开上抬，右片实心；文字行跨过切口时错位并由主题色反白。
//
//go:embed web/ui/mark.svg
var defaultSiteMark string

// DefaultSiteMarkSVG 按调用方已校验的主题色输出默认标志。
func DefaultSiteMarkSVG(color string) string {
	return strings.ReplaceAll(defaultSiteMark, "currentColor", color)
}
