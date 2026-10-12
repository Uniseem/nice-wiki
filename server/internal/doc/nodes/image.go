package nodes

import (
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var assetHash = regexp.MustCompile(`^[a-f0-9]{64}$`)
var assetExtension = regexp.MustCompile(`^\.[a-z0-9]{1,16}$`)

func assetURL(c Context, kind, hash, extension string) string {
	if !assetHash.MatchString(hash) || !assetExtension.MatchString(extension) {
		return ""
	}
	if c.AssetURL != nil {
		return c.AssetURL(kind, hash, extension)
	}
	return "/assets/" + kind + "/" + hash + extension
}

type image struct{}

func init() { Register("image", image{}) }
func (image) Render(n Node, c Context) (string, error) {
	extension := String(n, "extension")
	if extension == "" {
		extension = ".webp"
	}
	source := assetURL(c, "images", String(n, "hash"), extension)
	if source == "" {
		return "", nil
	}
	alt := String(n, "alt")
	caption := String(n, "caption")
	if c.Format == Text {
		return strings.TrimSpace(alt+" "+caption) + "\n", nil
	}
	if c.Format == Markdown {
		return "![" + EscapeMarkdown(alt) + "](" + source + ")\n\n" + EscapeMarkdown(caption) + "\n\n", nil
	}
	type variant struct {
		width int
		url   string
	}
	variants := []variant{}
	if values, ok := n.Attrs["variants"].([]any); ok {
		for _, raw := range values {
			attrs, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			hash, _ := attrs["hash"].(string)
			width, _ := attrs["width"].(float64)
			ext, _ := attrs["extension"].(string)
			if ext == "" {
				ext = ".webp"
			}
			target := assetURL(c, "images", hash, ext)
			if target != "" && width >= 1 && width <= 10000 {
				variants = append(variants, variant{int(width), target})
			}
		}
	}
	sort.Slice(variants, func(i, j int) bool { return variants[i].width < variants[j].width })
	srcset := []string{}
	previous := 0
	for _, v := range variants {
		if v.width == previous {
			continue
		}
		srcset = append(srcset, html.EscapeString(v.url)+" "+strconv.Itoa(v.width)+"w")
		previous = v.width
	}
	attrs := ""
	if len(srcset) > 0 {
		attrs = ` srcset="` + strings.Join(srcset, ", ") + `" sizes="(max-width: 48rem) 100vw, 800px"`
	}
	result := "<figure" + Anchor(n) + ` class="nw-image" style="inline-size:` + Width(n) + `"><img src="` + html.EscapeString(source) + `" alt="` + html.EscapeString(alt) + `" loading="lazy" decoding="async"` + attrs + ">"
	if caption != "" {
		result += "<figcaption>" + html.EscapeString(caption) + "</figcaption>"
	}
	return result + "</figure>", nil
}
func (image) Translate(n Node) []string {
	result := []string{}
	for _, key := range []string{"alt", "caption"} {
		if value := String(n, key); value != "" {
			result = append(result, value)
		}
	}
	return result
}
