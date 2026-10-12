package assets

import (
	"mime"
	"path/filepath"
	"strings"
)

var ErrNotFound = invalid("资源不存在")
var ErrReferenced = invalid("资源仍被当前文档、草稿或回收站引用")

type Type struct {
	Kind      string
	Extension string
	MIME      string
	Limit     int64
}
type Metadata struct {
	Hash       string `json:"hash"`
	Kind       string `json:"kind"`
	Extension  string `json:"extension"`
	MIME       string `json:"mimeType"`
	Size       int64  `json:"size"`
	Name       string `json:"name"`
	CreatedAt  int64  `json:"createdAt"`
	References int64  `json:"references"`
}

func Detect(filename, kind, detected string) (Type, error) {
	extension := strings.ToLower(filepath.Ext(filename))
	if kind == "images" {
		types := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp"}
		expected, ok := types[extension]
		if !ok || detected != expected {
			return Type{}, invalid("图片格式与内容不一致，支持 PNG、JPEG、GIF、WebP")
		}
		return Type{kind, extension, expected, 32 << 20}, nil
	}
	if kind == "videos" {
		types := map[string]string{".mp4": "video/mp4", ".webm": "video/webm"}
		expected, ok := types[extension]
		if !ok || detected != expected {
			return Type{}, invalid("视频仅支持 MP4 和 WebM，且后缀必须匹配内容")
		}
		return Type{kind, extension, expected, 128 << 20}, nil
	}
	if kind == "drawings" {
		switch extension {
		case ".svg":
			return Type{kind, extension, "image/svg+xml", 4 << 20}, nil
		case ".excalidraw":
			return Type{kind, extension, "application/json", 16 << 20}, nil
		}
		return Type{}, invalid("画板仅支持 SVG 或 Excalidraw 场景")
	}
	if kind != "files" {
		return Type{}, invalid("资源类型无效")
	}
	allowed := map[string]bool{".pdf": true, ".txt": true, ".md": true, ".csv": true, ".json": true, ".zip": true, ".docx": true, ".xlsx": true, ".pptx": true, ".tar": true, ".gz": true}
	if !allowed[extension] {
		return Type{}, invalid("附件后缀不在允许清单")
	}
	contentType := mime.TypeByExtension(extension)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	// 附件只下载，不作为网页或脚本执行。
	return Type{kind, extension, contentType, 64 << 20}, nil
}

// ValidationError 的消息只包含可向调用者展示的静态规则，不包含凭据或路径。
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
func invalid(message string) error       { return &ValidationError{Message: message} }
