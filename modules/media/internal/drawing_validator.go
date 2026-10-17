package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/modules/media/internal/gen"
)

type ValidateDrawingSceneInput struct {
	Scene json.RawMessage `json:"scene"`
}
type ValidateDrawingSceneResult struct {
	Valid bool   `json:"valid"`
	Text  string `json:"text"`
}

type drawingElement struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	IsDeleted     bool     `json:"isDeleted"`
	Text          string   `json:"text"`
	FrameID       *string  `json:"frameId"`
	FileID        *string  `json:"fileId"`
	ContainerID   *string  `json:"containerId"`
	Link          *string  `json:"link"`
	GroupIDs      []string `json:"groupIds"`
	BoundElements []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"boundElements"`
	StartBinding *struct {
		ElementID string `json:"elementId"`
	} `json:"startBinding"`
	EndBinding *struct {
		ElementID string `json:"elementId"`
	} `json:"endBinding"`
	Points           [][2]float64 `json:"points"`
	Pressures        []float64    `json:"pressures"`
	SimulatePressure bool         `json:"simulatePressure"`
}
type drawingFile struct {
	ID       string `json:"id"`
	MIMEType string `json:"mimeType"`
	DataURL  string `json:"dataURL"`
}

var drawingFileKey = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
var sceneSchema struct {
	sync.Once
	validate func(json.RawMessage) error
	err      error
}

// ValidateDrawingScene 先使用唯一schema，再检查图元/文件引用及嵌入资源语义。
func ValidateDrawingScene(ctx context.Context, op OperationContext, input ValidateDrawingSceneInput) (ValidateDrawingSceneResult, error) {
	sceneSchema.Do(func() {
		sceneSchema.validate, sceneSchema.err = apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "DrawingScene")
	})
	if sceneSchema.err != nil {
		return ValidateDrawingSceneResult{}, platform.NewError(platform.CodeIntegrityError, false, "画板场景schema未完整装配")
	}
	var scene struct {
		Type     string                 `json:"type"`
		Version  int                    `json:"version"`
		Elements []drawingElement       `json:"elements"`
		Files    map[string]drawingFile `json:"files"`
	}
	if json.Unmarshal(input.Scene, &scene) != nil {
		return ValidateDrawingSceneResult{}, invalidMedia("画板场景JSON不合法")
	}
	if scene.Version != 2 {
		return ValidateDrawingSceneResult{}, platform.NewError(platform.CodeUnsupportedSchema, false, "画板场景版本不受支持，不能自动丢弃字段")
	}
	if err := sceneSchema.validate(input.Scene); err != nil {
		return ValidateDrawingSceneResult{}, err
	}
	elements := make(map[string]drawingElement, len(scene.Elements))
	for _, element := range scene.Elements {
		if _, found := elements[element.ID]; found {
			return ValidateDrawingSceneResult{}, invalidMedia("画板图元ID重复")
		}
		elements[element.ID] = element
		if element.Link != nil && !safeDrawingLink(*element.Link) {
			return ValidateDrawingSceneResult{}, invalidMedia("画板图元链接不安全")
		}
		groups := make(map[string]bool, len(element.GroupIDs))
		for _, id := range element.GroupIDs {
			if groups[id] {
				return ValidateDrawingSceneResult{}, invalidMedia("画板分组引用重复")
			}
			groups[id] = true
		}
		if element.Type == "freedraw" && ((!element.SimulatePressure && len(element.Pressures) != len(element.Points)) || (element.SimulatePressure && len(element.Pressures) != 0 && len(element.Pressures) != len(element.Points))) {
			return ValidateDrawingSceneResult{}, invalidMedia("画板压力与点数量不一致")
		}
	}
	for key, file := range scene.Files {
		if !drawingFileKey.MatchString(key) || key != file.ID {
			return ValidateDrawingSceneResult{}, invalidMedia("画板文件键与ID不一致")
		}
		data, mediaType, err := decodeDrawingRaster(file.DataURL, file.MIMEType)
		if err != nil {
			return ValidateDrawingSceneResult{}, err
		}
		if err := validateDrawingRaster(ctx, data, mediaType); err != nil {
			return ValidateDrawingSceneResult{}, err
		}
	}
	var text strings.Builder
	for _, element := range scene.Elements {
		if element.FrameID != nil {
			frame, found := elements[*element.FrameID]
			if !found || frame.Type != "frame" || frame.ID == element.ID {
				return ValidateDrawingSceneResult{}, invalidMedia("画板frame引用不存在或类型不匹配")
			}
		}
		if element.ContainerID != nil {
			container, found := elements[*element.ContainerID]
			if !found || container.ID == element.ID {
				return ValidateDrawingSceneResult{}, invalidMedia("画板文字容器不存在")
			}
		}
		if element.FileID != nil {
			if _, found := scene.Files[*element.FileID]; !found {
				return ValidateDrawingSceneResult{}, invalidMedia("画板图像文件不存在")
			}
		}
		bound := map[string]bool{}
		for _, ref := range element.BoundElements {
			target, found := elements[ref.ID]
			if !found || target.Type != ref.Type || bound[ref.ID] || ref.ID == element.ID {
				return ValidateDrawingSceneResult{}, invalidMedia("画板绑定图元不存在、重复或类型不匹配")
			}
			bound[ref.ID] = true
		}
		for _, binding := range []*struct {
			ElementID string `json:"elementId"`
		}{element.StartBinding, element.EndBinding} {
			if binding != nil {
				if _, found := elements[binding.ElementID]; !found || binding.ElementID == element.ID {
					return ValidateDrawingSceneResult{}, invalidMedia("画板线绑定图元不存在")
				}
			}
		}
		if element.Type == "text" && !element.IsDeleted {
			if text.Len() > 0 {
				text.WriteByte('\n')
			}
			text.WriteString(element.Text)
		}
	}
	// frame父链单调着色，避免嵌套环；不复制schema的坐标/字段校验。
	state := map[string]uint8{}
	for id := range elements {
		path := []string{}
		cursor := id
		for state[cursor] == 0 {
			state[cursor] = 1
			path = append(path, cursor)
			frame := elements[cursor].FrameID
			if frame == nil {
				cursor = ""
				break
			}
			cursor = *frame
		}
		if cursor != "" && state[cursor] == 1 {
			return ValidateDrawingSceneResult{}, invalidMedia("画板frame引用形成环")
		}
		for _, item := range path {
			state[item] = 2
		}
	}
	if utf8.RuneCountInString(text.String()) > 1048576 {
		return ValidateDrawingSceneResult{}, invalidMedia("画板提取文字超过上限")
	}
	return ValidateDrawingSceneResult{Valid: true, Text: text.String()}, nil
}

func decodeDrawingRaster(value, expected string) ([]byte, string, error) {
	separator := strings.IndexByte(value, ',')
	if separator < 0 {
		return nil, "", invalidMedia("画板图片data URL不合法")
	}
	header := value[:separator]
	mediaType := ""
	for _, allowed := range []string{"image/png", "image/jpeg", "image/webp", "image/gif", "image/avif"} {
		if header == "data:"+allowed+";base64" {
			mediaType = allowed
			break
		}
	}
	if mediaType == "" || (expected != "" && expected != mediaType) || len(value) > 33554432 {
		return nil, "", invalidMedia("画板只允许受限raster base64图片")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(value[separator+1:])
	if err != nil || len(data) == 0 {
		return nil, "", invalidMedia("画板图片base64不合法")
	}
	return data, mediaType, nil
}

func validateDrawingRaster(ctx context.Context, data []byte, mediaType string) error {
	reader := bytes.NewReader(data)
	width, height := 0, 0
	var err error
	switch mediaType {
	case "image/png":
		if http.DetectContentType(data) != "image/png" {
			return invalidMedia("画板PNG签名不一致")
		}
		width, height, _, err = inspectPNG(ctx, reader, int64(len(data)))
	case "image/jpeg":
		if http.DetectContentType(data) != "image/jpeg" {
			return invalidMedia("画板JPEG签名不一致")
		}
		var config image.Config
		config, _, err = image.DecodeConfig(io.LimitReader(reader, maxMetadataBytes))
		width, height = config.Width, config.Height
	case "image/gif":
		if http.DetectContentType(data) != "image/gif" {
			return invalidMedia("画板GIF签名不一致")
		}
		width, height, _, err = inspectGIF(ctx, reader, int64(len(data)))
	case "image/webp":
		if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
			return invalidMedia("画板WebP签名不一致")
		}
		width, height, _, err = inspectWebP(ctx, reader, int64(len(data)))
	case "image/avif":
		var avif bool
		avif, _, err = inspectBrands(reader, int64(len(data)))
		if err == nil && !avif {
			return invalidMedia("画板AVIF签名不一致")
		}
		if err == nil {
			width, height, err = inspectBMFF(ctx, reader, int64(len(data)), true)
		}
	}
	if err != nil {
		return invalidMedia("画板图片容器元数据不合法")
	}
	if width < 1 || height < 1 || width > 100000 || height > 100000 {
		return invalidMedia("画板图片尺寸不受支持")
	}
	return nil
}
