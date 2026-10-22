package publicsite

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"nice-wiki/libs/platform"
)

// 页面模板（templates/pages.html）：HeroUI v3 组件类 + Tailwind 工具类，样式表由前端构建产出到 /_site/site.css。
//
//go:embed templates/*.html
var templateFS embed.FS

var pageTemplates = template.Must(template.New("").Funcs(template.FuncMap{"asset": assetURL}).ParseFS(templateFS, "templates/*.html"))

// assetVersion 区分每次发版的前端产物：/_site/ 下的 site.css、site.js 文件名固定，CDN 和浏览器会按 URL 缓存几个小时，
// 地址里带上版本段（/_site/v/<版本>/site.js，portal 去掉版本段再找文件）后升级立刻生效。版本放在路径里而不是查询参数：
// 懒加载块按相对路径引用 site.js，查询参数会让同一个模块按两个地址各加载一次。取二进制里嵌入的提交号（发版时每个服务都重新编译），没有时用启动时间。
var assetVersion = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && len(setting.Value) >= 12 {
				return setting.Value[:12]
			}
		}
	}
	return strconv.FormatInt(time.Now().Unix(), 36)
}()

func assetURL(name string) string { return "/_site/v/" + assetVersion + "/" + name }

type ComposeHTMLInput struct {
	SafeHTML      string
	Title         string
	TOC           []TOCItem
	Navigation    []NavigationNode
	Metadata      string
	Locale        string
	DocumentID    string
	Breadcrumbs   []Breadcrumb
	Previous      []Breadcrumb
	Next          []Breadcrumb
	Announcement  string
	Languages     []LanguageLink
	SiteName      string // 站名，非空时页头品牌显示站名（和站标）
	LogoURL       string
	ThemeColor    string // #RRGGBB，作为 HeroUI 的强调色
	Footer        string // 站长写的页脚（版权、链接、ICP），已转义的 HTML
	SourceCodeURL string // 页脚“源代码”链接，空则用默认
	Untranslated  bool
	UIStrings     map[string]string // 界面字典译文（key 为中文原文），可为空
	AIEnabled     bool
	AskURL        string
	MarkdownURL   string
	UpdatedAt     time.Time // 最后发布时间，页尾显示“最后更新”
}
type LanguageLink struct {
	Tag, Href string
	// Prompt 是语言建议弹窗在这种语言下的写法（key 为中文原文）；当前语言不带。
	Prompt map[string]string
}
type ComposeHTMLResult struct{ HTML string }

type languageItem struct {
	Tag     string            `json:"tag"`
	Href    string            `json:"href"`
	Label   string            `json:"label"`
	Current bool              `json:"current"`
	Prompt  map[string]string `json:"prompt,omitempty"`
}

// shell 是所有公开页共用的外壳数据；T 把中文界面文字换成当前语言。
type shell struct {
	Locale, Dir, Kind, DocumentID                          string
	Head, ExtraHead, Announcement, Footer                  template.HTML
	Accent                                                 template.CSS
	SiteName, Initial, LogoURL, HomeURL, AskURL, SourceURL string
	AIEnabled, HasNav                                      bool
	Languages                                              []languageItem
	Config                                                 string
	catalog                                                map[string]string
}

func (s shell) T(key string) string { return uiText(s.Locale, s.catalog, key) }

type treeNode struct {
	Title, URL    string
	Current, Open bool
	Children      []treeNode
}

type tocLink struct{ Label, Anchor, Indent string }
type crumb struct {
	Label, URL string
	Current    bool
}

type docPage struct {
	shell
	Tree           []treeNode
	Title          string
	Body           template.HTML
	TOC            []tocLink
	Breadcrumbs    []crumb
	Previous, Next *Breadcrumb
	Untranslated   bool
	MarkdownURL    string
	// 最后更新日期：Updated 给人看（YYYY-MM-DD，各语言通用），UpdatedISO 给 <time datetime>。
	Updated, UpdatedISO string
}

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

var languageNames = map[string]string{
	"zh-Hans": "简体中文", "zh-Hant": "繁體中文", "zh": "中文", "en": "English", "ja": "日本語", "ko": "한국어", "fr": "Français", "de": "Deutsch",
	"es": "Español", "ru": "Русский", "pt": "Português", "it": "Italiano", "ar": "العربية", "he": "עברית", "fa": "فارسی", "vi": "Tiếng Việt", "th": "ไทย", "id": "Bahasa Indonesia",
}

// 岛（React）用到的界面文字：在服务端按页面语言取好，随页面下发。
var islandStrings = []string{
	"AI 问答", "新对话", "发送", "关闭", "停止", "问问这个站点的文档…", "来源：", "这次没有回答成功（", "），请稍后再试。", "回答连接失败（", "请求失败（",
	"人机验证没有完成", "人机验证无法在此浏览器运行", "思考中…", "思考过程", "理解问题", "阅读当前页面", "检索相关文档", "整理回答", "当前页面", "有帮助", "没帮助", "感谢反馈", "回答由 AI 生成，可能有误，请以文档原文为准。",
	"亮 / 暗", "浅色", "深色", "跟随系统", "语言", "文档树", "本页目录", "复制代码", "已复制", "复制失败，请打开 Markdown 版本", "复制为 Markdown", "视频播放器",
	"要切换到这种语言阅读吗？", "切换", "保持当前语言", "搜索", "搜索文档", "搜索文档标题…", "没有找到相关文档",
}

// languagePromptKeys 是语言建议弹窗的文字：弹窗同时用当前语言和建议语言各写一遍。
var languagePromptKeys = []string{"要切换到这种语言阅读吗？", "切换", "保持当前语言"}

func newShell(in ComposeHTMLInput, kind string) shell {
	s := shell{
		Locale: in.Locale, Dir: textDirection(in.Locale), Kind: kind, DocumentID: in.DocumentID,
		Head: template.HTML(in.Metadata), Announcement: template.HTML(in.Announcement), Footer: template.HTML(in.Footer),
		SiteName: in.SiteName, LogoURL: in.LogoURL, HomeURL: "/" + in.Locale + "/", AskURL: in.AskURL, SourceURL: in.SourceCodeURL,
		AIEnabled: in.AIEnabled, catalog: in.UIStrings,
	}
	if s.SiteName == "" {
		s.SiteName = s.T("文档")
	}
	s.Initial = strings.ToUpper(string([]rune(s.SiteName)[:1]))
	if s.SourceURL == "" {
		s.SourceURL = defaultSourceCodeURL
	}
	if hexColor.MatchString(in.ThemeColor) {
		s.Accent = template.CSS(in.ThemeColor)
	}
	for _, link := range in.Languages {
		label := languageNames[link.Tag]
		if label == "" {
			label = link.Tag
		}
		s.Languages = append(s.Languages, languageItem{Tag: link.Tag, Href: link.Href, Label: label, Current: link.Tag == in.Locale, Prompt: link.Prompt})
	}
	ui := make(map[string]string, len(islandStrings))
	for _, key := range islandStrings {
		ui[key] = s.T(key)
	}
	config, _ := json.Marshal(map[string]any{
		"kind": kind, "locale": in.Locale, "documentId": in.DocumentID, "aiEnabled": in.AIEnabled, "askUrl": in.AskURL,
		"languages": s.Languages, "ui": ui,
	})
	s.Config = string(config)
	return s
}

// buildTree 按导航顺序组装文档树；当前文档的祖先默认展开。
func buildTree(nodes []NavigationNode, current string) ([]treeNode, error) {
	ordered, err := navigationOrder(nodes)
	if err != nil {
		return nil, err
	}
	children := map[string][]NavigationNode{}
	byID := map[string]NavigationNode{}
	for _, node := range ordered {
		parent := ""
		if node.ParentID != nil {
			parent = *node.ParentID
		}
		children[parent] = append(children[parent], node)
		byID[node.DocumentID] = node
	}
	open := map[string]bool{}
	for id := current; id != ""; {
		node, ok := byID[id]
		if !ok || node.ParentID == nil {
			break
		}
		id = *node.ParentID
		open[id] = true
	}
	var build func(string) []treeNode
	build = func(parent string) []treeNode {
		out := make([]treeNode, 0, len(children[parent]))
		for _, node := range children[parent] {
			out = append(out, treeNode{Title: node.Title, URL: node.CanonicalPath, Current: node.DocumentID == current, Open: open[node.DocumentID] || node.DocumentID == current, Children: build(node.DocumentID)})
		}
		return out
	}
	return build(""), nil
}

// 代码高亮的配色：浅色 github、深色 github-dark，放进 components 层，正文排版里的背景与边框优先。
var codeHighlightCSS = func() string {
	var light, dark strings.Builder
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	_ = formatter.WriteCSS(&light, styles.Get("github"))
	_ = formatter.WriteCSS(&dark, styles.Get("github-dark"))
	keep := func(css string) string {
		var out []string
		for _, line := range strings.Split(css, "\n") {
			if strings.Contains(line, "/* Background */") || strings.Contains(line, "/* PreWrapper */") {
				continue
			}
			out = append(out, line)
		}
		return strings.Join(out, "\n")
	}
	return "<style>@layer components{" + keep(light.String()) + strings.ReplaceAll(keep(dark.String()), ".chroma", ".dark .chroma") + "}</style>"
}()

func render(name string, data any) (string, error) {
	var out bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&out, name, data); err != nil {
		return "", platform.NewError(platform.CodeInternal, false, "PAGE_TEMPLATE_FAILED")
	}
	return out.String(), nil
}

// ComposeHTML 正文、导航及目录由 Go 完整输出，脚本只增强交互。
func ComposeHTML(ctx context.Context, op platform.OperationContext, in ComposeHTMLInput) (ComposeHTMLResult, error) {
	tree, err := buildTree(in.Navigation, in.DocumentID)
	if err != nil {
		return ComposeHTMLResult{}, err
	}
	page := docPage{shell: newShell(in, "doc"), Tree: tree, Title: in.Title, Body: template.HTML(in.SafeHTML), Untranslated: in.Untranslated}
	page.HasNav = len(tree) > 0
	if strings.Contains(in.SafeHTML, "class=\"code-block\"") {
		page.ExtraHead += template.HTML(codeHighlightCSS)
	}
	for _, item := range in.TOC {
		indent := ""
		switch item.Level {
		case 3:
			indent = "ps-6"
		case 4:
			indent = "ps-9"
		}
		page.TOC = append(page.TOC, tocLink{Label: item.Label, Anchor: item.Anchor, Indent: indent})
	}
	for i, item := range in.Breadcrumbs {
		label := item.Label
		if i == 0 {
			label = page.T(label) // 第一项是固定的“首页”，其余是站长写的标题
		}
		page.Breadcrumbs = append(page.Breadcrumbs, crumb{Label: label, URL: item.URL, Current: i == len(in.Breadcrumbs)-1})
	}
	if len(page.Breadcrumbs) < 2 {
		page.Breadcrumbs = nil
	}
	if len(in.Previous) > 0 {
		page.Previous = &in.Previous[0]
	}
	if len(in.Next) > 0 {
		page.Next = &in.Next[0]
	}
	if safeLink(in.MarkdownURL) {
		page.MarkdownURL = in.MarkdownURL
	}
	if !in.UpdatedAt.IsZero() {
		page.Updated, page.UpdatedISO = in.UpdatedAt.UTC().Format("2006-01-02"), in.UpdatedAt.UTC().Format(time.RFC3339)
	}
	html, err := render("doc", page)
	return ComposeHTMLResult{html}, err
}

type statusPage struct {
	Locale, Dir, Title, Hint, Home, Back string
	Code                                 int
}

// textDirection 是语言的书写方向：阿拉伯文等从右到左的语言返回 rtl，其余 ltr。
func textDirection(locale string) string {
	switch strings.SplitN(locale, "-", 2)[0] {
	case "ar", "he", "fa", "ur":
		return "rtl"
	}
	return "ltr"
}

// RenderStatusPage 404/410/暂时不可用等说明页，沿用公开站样式。
func RenderStatusPage(locale string, code int, title, hint string) string {
	if locale == "" {
		locale = "zh-Hans"
	}
	page := statusPage{Locale: locale, Dir: textDirection(locale), Code: code, Title: uiText(locale, nil, title), Hint: uiText(locale, nil, hint), Home: "/", Back: uiText(locale, nil, "回到首页")}
	html, err := render("status", page)
	if err != nil {
		return "<!doctype html><title>" + template.HTMLEscapeString(page.Title) + "</title><h1>" + template.HTMLEscapeString(page.Title) + "</h1>"
	}
	return html
}
