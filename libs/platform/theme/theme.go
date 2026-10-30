// Package theme 定义站点外观包的标准结构（docs/theme-standard.md）：theme.json 加可选的 theme.css、预览图和 assets/。
// 这里负责解析与校验压缩包、计算内容摘要、把配色/字体/版式渲染成公开站 <head> 里的样式，并内置几套外观。
// 单站点版与多用户版共用这一份；外观只能换样式，不能带脚本或模板。
package theme

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"nice-wiki/libs/platform"
)

const (
	SchemaVersion   = 1
	MaxPackageBytes = 2 << 20   // 压缩包本身
	MaxTotalBytes   = 4 << 20   // 解压后合计
	MaxFileBytes    = 512 << 10 // 单个文件
	MaxStylesheet   = 64 << 10
	MaxManifest     = 32 << 10
	MaxFiles        = 64
)

// Manifest 是 theme.json。未写的配色、字体、版式沿用默认外观（classic）。
type Manifest struct {
	SchemaVersion int                `json:"schemaVersion"`
	ID            string             `json:"id"`
	Version       string             `json:"version"`
	Name          map[string]string  `json:"name"`
	Description   map[string]string  `json:"description,omitempty"`
	Author        string             `json:"author"`
	License       string             `json:"license"`
	Homepage      string             `json:"homepage,omitempty"`
	ColorSchemes  []string           `json:"colorSchemes"`
	Tokens        map[string]Palette `json:"tokens,omitempty"`
	Typography    Typography         `json:"typography,omitzero"`
	Shape         Shape              `json:"shape,omitzero"`
	Layout        Layout             `json:"layout,omitzero"`
	Fonts         []Font             `json:"fonts,omitempty"`
	Stylesheet    string             `json:"stylesheet,omitempty"`
	Preview       string             `json:"preview,omitempty"`
}

// Palette 是一种配色（light 或 dark）里的颜色，键见 PaletteKeys。
type Palette map[string]string

type Typography struct {
	Sans       string  `json:"sans,omitempty"`       // 界面、导航与标题
	Body       string  `json:"body,omitempty"`       // 正文
	Mono       string  `json:"mono,omitempty"`       // 代码
	BodySize   float64 `json:"bodySize,omitempty"`   // 正文字号，单位 px
	LineHeight float64 `json:"lineHeight,omitempty"` // 正文行高
}

type Shape struct {
	Radius string `json:"radius,omitempty"` // none | small | medium | large
}

type Layout struct {
	ContentWidth string `json:"contentWidth,omitempty"` // narrow | normal | wide
	Sidebar      string `json:"sidebar,omitempty"`      // left | right
	TOC          string `json:"toc,omitempty"`          // right | hidden
	Header       string `json:"header,omitempty"`       // sticky | static
}

type Font struct {
	Family string `json:"family"`
	Src    string `json:"src"`              // assets/ 下的 woff2 或 woff
	Weight string `json:"weight,omitempty"` // "400" 或可变字体的 "100 900"
	Style  string `json:"style,omitempty"`  // normal | italic
}

// Package 是校验过的外观包。Files 含 theme.json 在内的全部文件；Digest 只由文件路径和内容决定，与压缩方式无关。
type Package struct {
	Manifest Manifest
	Files    map[string][]byte
	Digest   string
	Builtin  bool
}

// PaletteKeys 是配色里允许的键与对应的 CSS 变量（公开站与后台共用的 HeroUI 变量）。
var PaletteKeys = map[string]string{
	"background": "--background", "foreground": "--foreground", "surface": "--surface", "surfaceSecondary": "--surface-secondary",
	"surfaceTertiary": "--surface-tertiary", "overlay": "--overlay", "muted": "--muted", "default": "--default",
	"defaultForeground": "--default-foreground", "fieldBackground": "--field-background", "border": "--border",
	"separator": "--separator", "accent": "--accent", "accentForeground": "--accent-foreground", "focus": "--focus",
}

var (
	idPattern      = regexp.MustCompile(`^[a-z][a-z0-9-]{1,38}[a-z0-9]$`)
	versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,3})\.(0|[1-9][0-9]{0,3})\.(0|[1-9][0-9]{0,3})$`)
	localePattern  = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8}){0,2}$`)
	licensePattern = regexp.MustCompile(`^[A-Za-z0-9.+() -]{1,40}$`)
	hexColor       = regexp.MustCompile(`^#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	number         = `[+-]?[0-9]*\.?[0-9]+(%|deg)?`
	funcColor      = regexp.MustCompile(`^(rgb|rgba|hsl|hsla|oklch|oklab)\(\s*` + number + `(\s*,?\s*` + number + `){2}(\s*[,/]\s*` + number + `)?\s*\)$`)
	fontStack      = regexp.MustCompile(`^[\p{L}\p{N} ,"'_.-]{1,300}$`)
	fontFamily     = regexp.MustCompile(`^[\p{L}\p{N} _-]{1,60}$`)
	fontWeight     = regexp.MustCompile(`^[1-9]00( [1-9]00)?$`)
	assetPattern   = regexp.MustCompile(`^assets/[a-z0-9][a-z0-9._-]{0,63}$`)
)

var assetTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".gif": "image/gif", ".avif": "image/avif",
	".woff2": "font/woff2", ".woff": "font/woff",
}

func invalid(format string, args ...any) error {
	return platform.NewError(platform.CodeValidation, false, "外观包不合法："+format, args...)
}

// Parse 读取并校验 zip 外观包。整个目录打包（压缩包里只有一个顶层文件夹）也接受；系统生成的 __MACOSX、.DS_Store 忽略。
func Parse(raw []byte) (*Package, error) {
	if len(raw) == 0 || len(raw) > MaxPackageBytes {
		return nil, invalid("压缩包必须在 2MiB 以内")
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, invalid("不是有效的 zip 文件")
	}
	entries := make([]*zip.File, 0, len(reader.File))
	for _, file := range reader.File {
		base := path.Base(file.Name)
		if file.FileInfo().IsDir() || strings.HasPrefix(file.Name, "__MACOSX/") || base == ".DS_Store" || base == "Thumbs.db" {
			continue
		}
		entries = append(entries, file)
	}
	prefix := ""
	if !hasEntry(entries, "theme.json") {
		for _, file := range entries {
			if dir, name, ok := strings.Cut(file.Name, "/"); ok && name == "theme.json" {
				prefix = dir + "/"
			}
		}
	}
	files := map[string][]byte{}
	total := 0
	for _, file := range entries {
		name, ok := strings.CutPrefix(file.Name, prefix)
		if !ok {
			return nil, invalid("顶层文件夹之外还有文件 %s", file.Name)
		}
		if len(files) >= MaxFiles {
			return nil, invalid("文件不能超过 %d 个", MaxFiles)
		}
		if _, dup := files[name]; dup {
			return nil, invalid("文件 %s 重复", name)
		}
		if file.UncompressedSize64 > MaxFileBytes {
			return nil, invalid("文件 %s 超过 512KiB", name)
		}
		body, err := file.Open()
		if err != nil {
			return nil, invalid("文件 %s 无法解压", name)
		}
		data, err := io.ReadAll(io.LimitReader(body, MaxFileBytes+1))
		body.Close()
		if err != nil || len(data) > MaxFileBytes {
			return nil, invalid("文件 %s 无法解压或超过 512KiB", name)
		}
		if total += len(data); total > MaxTotalBytes {
			return nil, invalid("解压后合计不能超过 4MiB")
		}
		files[name] = data
	}
	return FromFiles(files)
}

func hasEntry(entries []*zip.File, name string) bool {
	for _, file := range entries {
		if file.Name == name {
			return true
		}
	}
	return false
}

// FromFiles 校验一组已展开的文件（路径相对外观包根目录）。
func FromFiles(files map[string][]byte) (*Package, error) {
	for name, data := range files {
		if err := checkFile(name, data); err != nil {
			return nil, err
		}
	}
	raw, ok := files["theme.json"]
	if !ok {
		return nil, invalid("缺少 theme.json")
	}
	if len(raw) > MaxManifest {
		return nil, invalid("theme.json 不能超过 32KiB")
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, invalid("theme.json 无法解析或有未知字段（%s）", jsonProblem(err))
	}
	if err := checkManifest(manifest, files); err != nil {
		return nil, err
	}
	if css, ok := files["theme.css"]; ok {
		if manifest.Stylesheet != "theme.css" {
			return nil, invalid("有 theme.css 时 theme.json 的 stylesheet 必须写 \"theme.css\"")
		}
		if err := CheckStylesheet(css, files); err != nil {
			return nil, err
		}
	}
	return &Package{Manifest: manifest, Files: files, Digest: digestOf(files)}, nil
}

func jsonProblem(err error) string {
	message := err.Error()
	if len(message) > 120 {
		message = message[:120]
	}
	return strings.TrimPrefix(message, "json: ")
}

func checkFile(name string, data []byte) error {
	switch {
	case name == "theme.json" || name == "theme.css" || name == "README.md" || name == "LICENSE" || name == "LICENSE.md" || name == "LICENSE.txt":
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return invalid("%s 必须是 UTF-8 文本", name)
		}
	case name == "preview.png" || name == "preview.jpg" || name == "preview.webp":
		if !magicMatches(path.Ext(name), data) {
			return invalid("%s 的内容与扩展名不符", name)
		}
	case assetPattern.MatchString(name) && assetTypes[path.Ext(name)] != "":
		if !magicMatches(path.Ext(name), data) {
			return invalid("%s 的内容与扩展名不符", name)
		}
	default:
		return invalid("不允许的文件 %s（只能有 theme.json、theme.css、preview.png/jpg/webp、README.md、LICENSE 和 assets/ 下的图片与字体）", name)
	}
	return nil
}

// magicMatches 按文件头核对图片与字体的实际格式，防止把别的内容改个扩展名放进来。
func magicMatches(ext string, data []byte) bool {
	switch ext {
	case ".png":
		return bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
	case ".jpg", ".jpeg":
		return bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF})
	case ".gif":
		return bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))
	case ".webp":
		return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	case ".avif":
		return len(data) >= 12 && string(data[4:8]) == "ftyp" && (string(data[8:12]) == "avif" || string(data[8:12]) == "avis")
	case ".woff2":
		return bytes.HasPrefix(data, []byte("wOF2"))
	case ".woff":
		return bytes.HasPrefix(data, []byte("wOFF"))
	}
	return false
}

func checkManifest(m Manifest, files map[string][]byte) error {
	if m.SchemaVersion != SchemaVersion {
		return invalid("schemaVersion 必须是 %d", SchemaVersion)
	}
	if !idPattern.MatchString(m.ID) {
		return invalid("id 只能用小写字母、数字和连字符，3–40 个字符，以字母开头")
	}
	if !versionPattern.MatchString(m.Version) {
		return invalid("version 必须是 主.次.修订 形式，如 1.0.0")
	}
	if err := checkLocalized("name", m.Name, 60, true); err != nil {
		return err
	}
	if err := checkLocalized("description", m.Description, 280, false); err != nil {
		return err
	}
	if !plainText(m.Author, 80) {
		return invalid("author 必须是 1–80 个字符的纯文本")
	}
	if !licensePattern.MatchString(m.License) {
		return invalid("license 请写 SPDX 许可证标识，如 MIT")
	}
	if m.Homepage != "" && (!strings.HasPrefix(m.Homepage, "https://") || len(m.Homepage) > 200 || !plainText(m.Homepage, 200) || strings.ContainsAny(m.Homepage, " \"'<>")) {
		return invalid("homepage 必须是 200 字符以内的 https 地址")
	}
	if len(m.ColorSchemes) == 0 || len(m.ColorSchemes) > 2 {
		return invalid("colorSchemes 必须是 [\"light\"]、[\"dark\"] 或两者")
	}
	seen := map[string]bool{}
	for _, scheme := range m.ColorSchemes {
		if (scheme != "light" && scheme != "dark") || seen[scheme] {
			return invalid("colorSchemes 只能是 light、dark，不能重复")
		}
		seen[scheme] = true
	}
	for scheme, palette := range m.Tokens {
		if !seen[scheme] {
			return invalid("tokens.%s 不在 colorSchemes 里", scheme)
		}
		for key, value := range palette {
			if PaletteKeys[key] == "" {
				return invalid("tokens.%s.%s 不是标准配色键", scheme, key)
			}
			if !ValidColor(value) {
				return invalid("tokens.%s.%s 不是可用的颜色（#十六进制、rgb()、hsl()、oklch() 或 oklab()）", scheme, key)
			}
		}
	}
	t := m.Typography
	for field, stack := range map[string]string{"sans": t.Sans, "body": t.Body, "mono": t.Mono} {
		if stack != "" && (!fontStack.MatchString(stack) || strings.Count(stack, `"`)%2 != 0 || strings.Count(stack, `'`)%2 != 0) {
			return invalid("typography.%s 只能是字体名列表（字母、数字、空格、引号、逗号、连字符）", field)
		}
	}
	if t.BodySize != 0 && (t.BodySize < 14 || t.BodySize > 22) {
		return invalid("typography.bodySize 必须在 14 到 22 之间")
	}
	if t.LineHeight != 0 && (t.LineHeight < 1.3 || t.LineHeight > 2.2) {
		return invalid("typography.lineHeight 必须在 1.3 到 2.2 之间")
	}
	if !oneOf(m.Shape.Radius, "", "none", "small", "medium", "large") {
		return invalid("shape.radius 只能是 none、small、medium、large")
	}
	l := m.Layout
	if !oneOf(l.ContentWidth, "", "narrow", "normal", "wide") || !oneOf(l.Sidebar, "", "left", "right") || !oneOf(l.TOC, "", "right", "hidden") || !oneOf(l.Header, "", "sticky", "static") {
		return invalid("layout 的取值不合法（contentWidth: narrow|normal|wide，sidebar: left|right，toc: right|hidden，header: sticky|static）")
	}
	if len(m.Fonts) > 8 {
		return invalid("fonts 最多 8 项")
	}
	for i, font := range m.Fonts {
		ext := path.Ext(font.Src)
		if !fontFamily.MatchString(font.Family) || (ext != ".woff2" && ext != ".woff") || files[font.Src] == nil || !assetPattern.MatchString(font.Src) ||
			(font.Weight != "" && !fontWeight.MatchString(font.Weight)) || !oneOf(font.Style, "", "normal", "italic") {
			return invalid("fonts[%d] 不合法：family 为字体名，src 指向包里 assets/ 下的 woff2/woff，weight 如 400 或 100 900，style 为 normal/italic", i)
		}
	}
	if m.Stylesheet != "" && (m.Stylesheet != "theme.css" || files["theme.css"] == nil) {
		return invalid("stylesheet 只能是 \"theme.css\"，且包里要有这个文件")
	}
	if m.Preview != "" && (!strings.HasPrefix(m.Preview, "preview.") || files[m.Preview] == nil) {
		return invalid("preview 必须指向包里的 preview.png、preview.jpg 或 preview.webp")
	}
	return nil
}

func checkLocalized(field string, values map[string]string, limit int, required bool) error {
	if required && len(values) == 0 {
		return invalid("%s 至少要有一种语言", field)
	}
	if len(values) > 20 {
		return invalid("%s 最多 20 种语言", field)
	}
	for locale, text := range values {
		if !localePattern.MatchString(locale) || !plainText(text, limit) {
			return invalid("%s 的键必须是语言标签（如 zh、en、zh-Hant），值为 1–%d 个字符的纯文本", field, limit)
		}
	}
	return nil
}

func plainText(value string, limit int) bool {
	count := utf8.RuneCountInString(value)
	return count >= 1 && count <= limit && strings.IndexFunc(value, unicode.IsControl) < 0 && !strings.ContainsAny(value, "<>")
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

// ValidColor 判断配色里的一个颜色值是否可用（只认常见写法，确保写进 CSS 不会越出这一条声明）。
func ValidColor(value string) bool {
	return len(value) <= 64 && (hexColor.MatchString(value) || funcColor.MatchString(value))
}

func digestOf(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	hash := sha256.New()
	for _, name := range names {
		sum := sha256.Sum256(files[name])
		fmt.Fprintf(hash, "%s\x00%x\n", name, sum)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// ValidDigest 判断字符串是不是外观包摘要（64 位小写十六进制）。
func ValidDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// Zip 把外观包重新打成 zip（文件按路径排序、固定时间戳），同一个包每次得到相同的字节。
func (p *Package) Zip() []byte {
	names := make([]string, 0, len(p.Files))
	for name := range p.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	stamp := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, name := range names {
		entry, _ := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: stamp})
		_, _ = entry.Write(p.Files[name])
	}
	_ = writer.Close()
	return out.Bytes()
}

// Size 是解压后的总字节数。
func (p *Package) Size() int {
	total := 0
	for _, data := range p.Files {
		total += len(data)
	}
	return total
}

// Label 按界面语言取外观名称：先找完全相同的语言，再找主语言，再依次用 en、zh，最后取任意一种。
func (m Manifest) Label(locale string) string { return pick(m.Name, locale) }

func pick(values map[string]string, locale string) string {
	if value := values[locale]; value != "" {
		return value
	}
	if base, _, ok := strings.Cut(locale, "-"); ok && values[base] != "" {
		return values[base]
	}
	for _, fallback := range []string{"en", "zh"} {
		if values[fallback] != "" {
			return values[fallback]
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return values[keys[0]]
}

// Servable 判断公开站能否按 /_theme/<摘要>/<路径> 对外提供这个文件（theme.json、说明文件不对外）。
func Servable(name string) bool {
	return name == "theme.css" || strings.HasPrefix(name, "preview.") || strings.HasPrefix(name, "assets/")
}

// ContentType 是对外提供文件时的媒体类型。
func ContentType(name string) string {
	if name == "theme.css" {
		return "text/css; charset=utf-8"
	}
	if strings.HasPrefix(name, "preview.") {
		return map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".webp": "image/webp"}[path.Ext(name)]
	}
	if kind := assetTypes[path.Ext(name)]; kind != "" {
		return kind
	}
	return "application/octet-stream"
}

// ManifestObject 把 theme.json 转成通用对象，给接口原样返回。
func (p *Package) ManifestObject() map[string]any {
	var out map[string]any
	_ = json.Unmarshal(p.Files["theme.json"], &out)
	return out
}
