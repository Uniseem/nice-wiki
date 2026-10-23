package publicsite

import "strings"

// markSVG 是 nice-wiki 的默认标志：主题色圆角方块里一张折角的白色文档，左下角带对话气泡的尾巴（文档 + 问答），
// 页上两行字和折角用主题色。站点没上传站标、图标时，页头和浏览器标签页都用它；后台的同一图形在 portal/web/src/brand.tsx。
const markSVG = `<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'>` +
	`<rect width='64' height='64' rx='16' fill='{c}'/>` +
	`<path fill='#fff' d='M24 12h12l11 11v16a7 7 0 0 1-7 7H29.5l-8.6 6.9A1.8 1.8 0 0 1 18 51.5V19a7 7 0 0 1 6-7Z'/>` +
	`<path fill='{c}' fill-opacity='.4' d='M36 12v5.5a5.5 5.5 0 0 0 5.5 5.5H47Z'/>` +
	`<rect x='25' y='27' width='15' height='4' rx='2' fill='{c}'/>` +
	`<rect x='25' y='35' width='9' height='4' rx='2' fill='{c}'/></svg>`

// siteMark 按站点主题色生成默认标志的 data URI；主题色无效时用近黑色。
func siteMark(themeColor string) string {
	if !hexColor.MatchString(themeColor) {
		themeColor = "#050505"
	}
	svg := strings.ReplaceAll(markSVG, "{c}", themeColor)
	return "data:image/svg+xml," + strings.NewReplacer("#", "%23", "<", "%3C", ">", "%3E").Replace(svg)
}
