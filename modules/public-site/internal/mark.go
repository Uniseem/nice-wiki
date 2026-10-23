package publicsite

import (
	"strings"

	"nice-wiki/libs/platform"
)

// siteMark 按站点主题色生成默认标志的 data URI；主题色无效时用近黑色。
func siteMark(themeColor string) string {
	if !hexColor.MatchString(themeColor) {
		themeColor = "#171717"
	}
	svg := platform.DefaultSiteMarkSVG(themeColor)
	return "data:image/svg+xml," + strings.NewReplacer("#", "%23", "<", "%3C", ">", "%3E").Replace(svg)
}
