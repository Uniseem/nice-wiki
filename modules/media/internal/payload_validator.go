package media

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"unicode"

	"nice-wiki/libs/platform"
)

type ValidateAssetPayloadInput struct {
	UploadID  Identifier `json:"uploadId"`
	Kind      string     `json:"kind"`
	MediaType string     `json:"mediaType"`
}
type ValidateAssetPayloadResult struct {
	Valid      bool `json:"valid"`
	CSPSandbox bool `json:"cspSandbox"`
}

// ValidateAssetPayload 不修改字节；交付策略必须与已确证的类型一起使用。
func ValidateAssetPayload(ctx context.Context, op OperationContext, input ValidateAssetPayloadInput) (ValidateAssetPayloadResult, error) {
	if input.Kind == "attachment" {
		if input.MediaType != "application/octet-stream" {
			return ValidateAssetPayloadResult{}, invalidMedia("附件必须按二进制下载交付")
		}
		return ValidateAssetPayloadResult{Valid: true}, nil
	}
	if input.MediaType != "image/svg+xml" && input.Kind != "drawing_scene" {
		return ValidateAssetPayloadResult{Valid: true}, nil
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return ValidateAssetPayloadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储未装配")
	}
	reader, err := s.OpenUpload(ctx, input.UploadID)
	if err != nil {
		return ValidateAssetPayloadResult{}, err
	}
	defer reader.Close()
	limit := int64(maxMetadataBytes)
	if input.Kind == "drawing_scene" {
		limit = 64 << 20
	}
	file := reader.(*os.File)
	info, err := file.Stat()
	if err != nil {
		return ValidateAssetPayloadResult{}, mapMediaDBError(err, "读取资源大小失败")
	}
	if info.Size() > limit {
		return ValidateAssetPayloadResult{}, invalidMedia("SVG或画板场景超过解析预算")
	}
	if input.MediaType == "image/svg+xml" {
		if err := validateSVG(ctx, chunkContextReader{ctx, io.LimitReader(reader, limit+1)}); err != nil {
			return ValidateAssetPayloadResult{}, err
		}
		return ValidateAssetPayloadResult{Valid: true, CSPSandbox: true}, nil
	}
	raw, err := io.ReadAll(io.LimitReader(chunkContextReader{ctx, reader}, limit+1))
	if err != nil {
		return ValidateAssetPayloadResult{}, mapMediaDBError(err, "读取画板场景失败")
	}
	if _, err := ValidateDrawingScene(ctx, op, ValidateDrawingSceneInput{Scene: json.RawMessage(raw)}); err != nil {
		return ValidateAssetPayloadResult{}, err
	}
	return ValidateAssetPayloadResult{Valid: true}, nil
}

var svgCSSOpen = regexp.MustCompile(`(?i)url\s*\(`)
var svgCSSURL = regexp.MustCompile(`(?i)url\s*\(([^)]*)\)`)

func validateSVG(ctx context.Context, reader io.Reader) error {
	decoder := xml.NewDecoder(reader)
	depth, tokens, roots := 0, 0, 0
	inStyle := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return invalidMedia("SVG XML格式不合法")
		}
		tokens++
		if tokens > 200000 {
			return invalidMedia("SVG节点数量超限")
		}
		switch value := token.(type) {
		case xml.Directive:
			return invalidMedia("SVG禁止DTD与外部实体")
		case xml.ProcInst:
			if value.Target != "xml" || roots != 0 {
				return invalidMedia("SVG不接受处理指令")
			}
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots != 1 || value.Name.Local != "svg" {
					return invalidMedia("SVG必须只有一个svg根节点")
				}
			}
			depth++
			if depth > 128 {
				return invalidMedia("SVG节点嵌套超限")
			}
			if value.Name.Space != "" && value.Name.Space != "http://www.w3.org/2000/svg" {
				return invalidMedia("SVG不接受外部命名空间元素")
			}
			switch value.Name.Local {
			case "svg", "g", "defs", "path", "rect", "circle", "ellipse", "line", "polyline", "polygon", "text", "tspan", "title", "desc", "clipPath", "mask", "linearGradient", "radialGradient", "stop", "pattern", "use", "image", "a", "marker", "symbol":
			case "style":
				inStyle = true
			default:
				return invalidMedia("SVG包含不允许的元素")
			}
			for _, attr := range value.Attr {
				key := strings.ToLower(attr.Name.Local)
				text := strings.TrimSpace(attr.Value)
				if strings.HasPrefix(key, "on") || key == "src" || key == "base" || key == "srcset" {
					return invalidMedia("SVG包含可执行或网络属性")
				}
				if attr.Name.Space != "" && attr.Name.Space != "xmlns" && attr.Name.Space != "http://www.w3.org/1999/xlink" && attr.Name.Space != "http://www.w3.org/XML/1998/namespace" {
					return invalidMedia("SVG属性命名空间不受支持")
				}
				if key == "href" {
					if strings.HasPrefix(text, "#") && len(text) > 1 {
						continue
					}
					if value.Name.Local == "image" && strings.HasPrefix(text, "data:image/") {
						data, mediaType, err := decodeDrawingRaster(text, "")
						if err != nil {
							return err
						}
						if err := validateDrawingRaster(ctx, data, mediaType); err != nil {
							return err
						}
						continue
					}
					if value.Name.Local == "a" && safeDrawingLink(text) {
						continue
					}
					return invalidMedia("SVG禁止外部资源与不安全链接")
				}
				if err := validateSVGCSS(text, false); err != nil {
					return err
				}
			}
		case xml.EndElement:
			depth--
			if value.Name.Local == "style" {
				inStyle = false
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(value)) != "" {
				return invalidMedia("SVG根节点外存在内容")
			}
			if inStyle {
				if err := validateSVGCSS(string(value), true); err != nil {
					return err
				}
			}
		}
	}
	if roots != 1 || depth != 0 {
		return invalidMedia("SVG根节点不完整")
	}
	return nil
}

func validateSVGCSS(value string, style bool) error {
	lower := strings.ToLower(value)
	if strings.ContainsAny(value, "\\") || strings.Contains(lower, "/*") || strings.Contains(lower, "@import") || strings.Contains(lower, "expression(") || strings.Contains(lower, "javascript:") || strings.Contains(lower, "-moz-binding") || strings.Contains(lower, "behavior:") || strings.Contains(lower, "image-set") || strings.Contains(lower, "@namespace") {
		return invalidMedia("SVG样式含可执行或混淆内容")
	}
	for _, match := range svgCSSURL.FindAllStringSubmatch(value, -1) {
		target := strings.Trim(strings.TrimSpace(match[1]), "\"'")
		if strings.HasPrefix(target, "#") && len(target) > 1 {
			continue
		}
		if style && strings.HasPrefix(target, "data:font/woff2;base64,") {
			if len(target) > 2<<20 {
				return invalidMedia("SVG嵌入字体超过预算")
			}
			if _, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(target, "data:font/woff2;base64,")); err == nil {
				continue
			}
		}
		return invalidMedia("SVG样式禁止网络或外部资源")
	}
	if svgCSSOpen.MatchString(svgCSSURL.ReplaceAllString(value, "")) {
		return invalidMedia("SVG样式资源声明不完整")
	}
	return nil
}

func safeDrawingLink(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Hostname() != ""
	case "mailto":
		return parsed.Opaque != "" || parsed.Path != ""
	}
	return false
}
