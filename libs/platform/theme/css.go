package theme

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// CheckStylesheet 校验 theme.css：只允许纯样式，url() 只能指向包里 assets/ 下的文件（相对 theme.css 解析到同一外观目录），
// 不能 @import、不能用转义（避免拼出被禁止的写法），也不接受 image-set()/src() 这类不经 url() 就能请求外部地址的函数。
func CheckStylesheet(css []byte, files map[string][]byte) error {
	if len(css) > MaxStylesheet {
		return invalid("theme.css 不能超过 64KiB")
	}
	lower := strings.ToLower(string(css))
	for _, banned := range []string{"\\", "@import", "expression(", "javascript:", "vbscript:", "image-set(", "src(", "</"} {
		if strings.Contains(lower, banned) {
			return invalid("theme.css 不能包含 %s", banned)
		}
	}
	rest := lower
	for {
		index := strings.Index(rest, "url(")
		if index < 0 {
			return nil
		}
		rest = rest[index+4:]
		end := strings.IndexByte(rest, ')')
		if end < 0 {
			return invalid("theme.css 里的 url( 没有闭合")
		}
		target := strings.Trim(strings.TrimSpace(rest[:end]), `"'`)
		if !assetPattern.MatchString(target) || files[target] == nil {
			return invalid("theme.css 的 url() 只能指向包里 assets/ 下的文件（%s）", truncate(target, 60))
		}
		rest = rest[end+1:]
	}
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

// Look 是渲染公开站时挂在 <html> 上的版式开关（未设置的取默认值）。
type Look struct {
	ID      string
	Sidebar string // left | right
	TOC     string // right | hidden
	Header  string // sticky | static
	Schemes string // "light dark"、"light" 或 "dark"
}

func (p *Package) Look() Look {
	l := p.Manifest.Layout
	return Look{ID: p.Manifest.ID, Sidebar: or(l.Sidebar, "left"), TOC: or(l.TOC, "right"), Header: or(l.Header, "sticky"), Schemes: strings.Join(p.Manifest.ColorSchemes, " ")}
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

var (
	contentWidths = map[string]string{"narrow": "680px", "normal": "800px", "wide": "960px"}
	// 控件（按钮、输入框、树节点）与卡片的圆角。
	radii = map[string][2]string{"none": {"0", "0"}, "small": {".375rem", ".5rem"}, "medium": {".75rem", "1rem"}, "large": {"1rem", "1.5rem"}}
)

// HeadCSS 把配色、字体、正文排版和圆角渲染成放进公开站 <style> 的 CSS。base 是这个外观文件的公开地址前缀（以 / 结尾），
// 只用于 @font-face；theme.css 另以 <link> 引入。没写的项不输出，沿用站点默认样式。
func (p *Package) HeadCSS(base string) string {
	m := p.Manifest
	var out bytes.Buffer
	for _, font := range m.Fonts {
		format := "woff2"
		if strings.HasSuffix(font.Src, ".woff") {
			format = "woff"
		}
		fmt.Fprintf(&out, `@font-face{font-family:"%s";src:url(%s%s) format("%s");font-weight:%s;font-style:%s;font-display:swap}`,
			font.Family, base, font.Src, format, or(font.Weight, "400"), or(font.Style, "normal"))
	}
	root := []string{}
	t := m.Typography
	if t.Sans != "" {
		root = append(root, "--font-sans:"+t.Sans)
	}
	if t.Body != "" {
		root = append(root, "--nw-font-body:"+t.Body)
	}
	if t.Mono != "" {
		root = append(root, "--font-mono:"+t.Mono)
	}
	if t.BodySize != 0 {
		root = append(root, "--nw-prose-size:"+strconv.FormatFloat(t.BodySize, 'f', -1, 64)+"px")
	}
	if t.LineHeight != 0 {
		root = append(root, "--nw-prose-leading:"+strconv.FormatFloat(t.LineHeight, 'f', -1, 64))
	}
	if width := contentWidths[m.Layout.ContentWidth]; width != "" {
		root = append(root, "--nw-content-width:"+width)
	}
	if radius, ok := radii[m.Shape.Radius]; ok {
		root = append(root, "--nw-radius-control:"+radius[0], "--nw-radius-card:"+radius[1])
	}
	if len(root) > 0 {
		out.WriteString(":root{" + strings.Join(root, ";") + "}")
	}
	// 和 palette.css 用同样的选择器；这里的规则不在层里，覆盖默认配色。
	if palette := m.Tokens["light"]; len(palette) > 0 {
		out.WriteString(`:root,[data-theme="light"]{` + declarations(palette) + "}")
	}
	if palette := m.Tokens["dark"]; len(palette) > 0 {
		out.WriteString(`:is(.dark,[data-theme="dark"]){` + declarations(palette) + "}")
	}
	return out.String()
}

func declarations(palette Palette) string {
	keys := make([]string, 0, len(palette))
	for key := range palette {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, PaletteKeys[key]+":"+palette[key])
	}
	return strings.Join(parts, ";")
}
