package mdconv

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	hintStart    = regexp.MustCompile(`^\{%\s*hint\b(.*?)%\}\s*$`)
	hintStyle    = regexp.MustCompile(`style\s*=\s*"([^"]*)"`)
	tabStart     = regexp.MustCompile(`^\{%\s*tab\b(.*?)%\}\s*$`)
	titleAttr    = regexp.MustCompile(`title\s*=\s*"([^"]*)"`)
	embedTag     = regexp.MustCompile(`^\{%\s*embed\b(.*?)%\}\s*$`)
	urlAttr      = regexp.MustCompile(`url\s*=\s*"([^"]*)"`)
	fileTag      = regexp.MustCompile(`^\{%\s*file\b(.*?)%\}\s*$`)
	srcAttr      = regexp.MustCompile(`src\s*=\s*"([^"]*)"`)
	anyTag       = regexp.MustCompile(`\{%.*?%\}`)
	frontMatterT = regexp.MustCompile(`(?m)^title:\s*["']?(.+?)["']?\s*$`)
)

var hintMarkers = map[string]string{"info": "INFO", "success": "SUCCESS", "warning": "WARNING", "danger": "DANGER"}

// preprocess 把 GitBook 的 {% %} 标签、$$ 公式块、YAML 头改写成标准 Markdown 可处理的形态。
// 围栏代码里的内容原样保留。返回处理后的文本、公式块源码列表和 YAML 头里的 title。
func preprocess(src string) (string, []string, string) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.TrimPrefix(src, "\xef\xbb\xbf")
	frontTitle := ""
	if strings.HasPrefix(src, "---\n") {
		if end := strings.Index(src[4:], "\n---"); end >= 0 {
			head := src[4 : 4+end]
			if m := frontMatterT.FindStringSubmatch(head); m != nil {
				frontTitle = strings.TrimSpace(m[1])
			}
			rest := src[4+end+4:]
			src = strings.TrimPrefix(rest, "\n")
		}
	}
	lines := strings.Split(src, "\n")
	var out []string
	var maths []string
	inFence, inHint := false, false
	fenceMark := ""
	emit := func(line string) {
		if inHint {
			if strings.TrimSpace(line) == "" {
				line = ">"
			} else {
				line = "> " + line
			}
		}
		out = append(out, line)
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			mark := trimmed[:3]
			switch {
			case !inFence:
				inFence, fenceMark = true, mark
			case mark == fenceMark:
				inFence = false
			}
			emit(line)
			continue
		}
		if inFence {
			emit(line)
			continue
		}
		if trimmed == "$$" || (len(trimmed) > 4 && strings.HasPrefix(trimmed, "$$") && strings.HasSuffix(trimmed, "$$")) {
			var latex string
			if trimmed == "$$" {
				var body []string
				for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "$$"; i++ {
					body = append(body, lines[i])
				}
				latex = strings.TrimSpace(strings.Join(body, "\n"))
			} else {
				latex = strings.TrimSpace(trimmed[2 : len(trimmed)-2])
			}
			if latex != "" {
				maths = append(maths, latex)
				emit("")
				emit(fmt.Sprintf("NWMATHBLOCK%dEND", len(maths)-1))
				emit("")
			}
			continue
		}
		if m := hintStart.FindStringSubmatch(trimmed); m != nil {
			style := "info"
			if s := hintStyle.FindStringSubmatch(m[1]); s != nil {
				style = strings.ToLower(s[1])
			}
			marker, ok := hintMarkers[style]
			if !ok {
				marker = "INFO"
			}
			emit("")
			inHint = true
			emit("[!" + marker + "]")
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "{% endhint"):
			inHint = false
			emit("")
			continue
		case tabStart.MatchString(trimmed):
			title := ""
			if t := titleAttr.FindStringSubmatch(tabStart.FindStringSubmatch(trimmed)[1]); t != nil {
				title = t[1]
			}
			emit("")
			if title != "" {
				emit("#### " + title)
				emit("")
			}
			continue
		case embedTag.MatchString(trimmed):
			if u := urlAttr.FindStringSubmatch(embedTag.FindStringSubmatch(trimmed)[1]); u != nil {
				emit("")
				emit("[" + u[1] + "](" + u[1] + ")")
				emit("")
			}
			continue
		case fileTag.MatchString(trimmed):
			if u := srcAttr.FindStringSubmatch(fileTag.FindStringSubmatch(trimmed)[1]); u != nil {
				emit("")
				emit("[" + u[1] + "](" + u[1] + ")")
				emit("")
			}
			continue
		}
		if strings.Contains(line, "{%") {
			line = anyTag.ReplaceAllString(line, "")
			if strings.TrimSpace(line) == "" {
				continue
			}
		}
		emit(line)
	}
	return strings.Join(out, "\n"), maths, frontTitle
}
