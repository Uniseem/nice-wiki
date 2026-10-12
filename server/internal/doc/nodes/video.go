package nodes

import (
	"html"
	"strings"
)

type video struct{}

func init() { Register("video", video{}) }
func (video) Render(n Node, c Context) (string, error) {
	title, description := String(n, "title"), String(n, "description")
	if c.Format == Text {
		return strings.TrimSpace(title+" "+description) + "\n", nil
	}
	if String(n, "source") == "platform" {
		// 平台提供方尚未接入，只展示保存的公开文字，不加载未经验证的播放器。
		if c.Format == Markdown {
			return "**" + EscapeMarkdown(title) + "**\n\n" + EscapeMarkdown(description) + "\n\n", nil
		}
		return "<figure" + Anchor(n) + ` class="nw-video"><figcaption>` + html.EscapeString(title) + "</figcaption><p>" + html.EscapeString(description) + "</p></figure>", nil
	}
	extension := String(n, "extension")
	if extension == "" {
		extension = ".mp4"
	}
	if extension != ".mp4" && extension != ".webm" {
		return "", nil
	}
	source := assetURL(c, "videos", String(n, "hash"), extension)
	if source == "" {
		return "", nil
	}
	if c.Format == Markdown {
		return "[" + EscapeMarkdown(title) + "](" + source + ")\n\n" + EscapeMarkdown(description) + "\n\n", nil
	}
	poster := assetURL(c, "images", String(n, "posterHash"), ".webp")
	attr := ""
	if poster != "" {
		attr = ` poster="` + html.EscapeString(poster) + `"`
	}
	return "<figure" + Anchor(n) + ` class="nw-video"><video controls preload="none" playsinline src="` + html.EscapeString(source) + `"` + attr + ` aria-label="` + html.EscapeString(title) + `"></video><figcaption>` + html.EscapeString(title) + "</figcaption><p>" + html.EscapeString(description) + "</p></figure>", nil
}
func (video) Translate(n Node) []string {
	result := []string{}
	for _, key := range []string{"title", "description"} {
		if value := String(n, key); value != "" {
			result = append(result, value)
		}
	}
	return result
}
