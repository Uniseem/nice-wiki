package assets

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"nice-wiki/internal/fsutil"
	"nice-wiki/internal/store/sqlc"
	"nice-wiki/internal/textutil"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func safeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	if len(name) > 240 {
		name = name[:240]
	}
	if name == "" || name == "." {
		return "attachment"
	}
	return name
}
func (s *Store) Upload(ctx context.Context, reader io.Reader, filename, kind string) (Metadata, error) {
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	data, err := io.ReadAll(io.LimitReader(reader, (128<<20)+1))
	if err != nil {
		return Metadata{}, err
	}
	if len(data) == 0 || len(data) > 128<<20 {
		return Metadata{}, invalid("资源为空或超过上传上限")
	}
	spec, err := Detect(filename, kind, http.DetectContentType(data))
	if err != nil {
		return Metadata{}, err
	}
	if int64(len(data)) > spec.Limit {
		return Metadata{}, invalid("资源超过对应类型的大小限制")
	}
	if kind == "drawings" {
		if spec.Extension == ".svg" {
			if err := validateSVG(data); err != nil {
				return Metadata{}, err
			}
		} else {
			var scene struct {
				Type     string            `json:"type"`
				Version  int               `json:"version"`
				Elements []json.RawMessage `json:"elements"`
				AppState json.RawMessage   `json:"appState"`
				Files    json.RawMessage   `json:"files"`
				Source   string            `json:"source"`
			}
			if err := json.Unmarshal(data, &scene); err != nil || scene.Type != "excalidraw" || len(scene.Elements) > 20000 {
				return Metadata{}, invalid("画板场景格式无效")
			}
		}
	}
	value := Metadata{Hash: textutil.Hash(data), Kind: kind, Extension: spec.Extension, MIME: spec.MIME, Size: int64(len(data)), Name: safeName(filename), CreatedAt: time.Now().Unix()}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, err := s.Get(ctx, value.Hash); err == nil {
		if err := s.exists(existing); err != nil {
			return Metadata{}, err
		}
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return Metadata{}, err
	}
	path, err := s.path(value)
	if err != nil {
		return Metadata{}, err
	}
	if err := fsutil.WriteFile(path, data, 0600); err != nil {
		return Metadata{}, err
	}
	if err := sqlc.New(s.db).UpsertAsset(ctx, sqlc.UpsertAssetParams{Hash: value.Hash, Kind: value.Kind, Extension: value.Extension, MimeType: value.MIME, Size: value.Size, OriginalName: value.Name, CreatedAt: value.CreatedAt}); err != nil {
		return Metadata{}, errors.Join(err, os.Remove(path))
	}
	return value, nil
}
func validateSVG(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	root := false
	nodes := 0
	forbidden := map[string]bool{"script": true, "foreignobject": true, "iframe": true, "object": true, "embed": true, "audio": true, "video": true, "animate": true, "set": true}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return invalid("SVG XML 格式无效")
		}
		switch value := token.(type) {
		case xml.Directive:
			return invalid("SVG 不允许声明外部实体")
		case xml.ProcInst:
			if value.Target != "xml" {
				return invalid("SVG 不允许处理指令")
			}
		case xml.StartElement:
			nodes++
			depth++
			if depth > 64 || nodes > 50000 {
				return invalid("SVG 结构超过上限")
			}
			if !root {
				if value.Name.Local != "svg" {
					return invalid("SVG 根元素无效")
				}
				root = true
			}
			if forbidden[strings.ToLower(value.Name.Local)] {
				return invalid("SVG 含可执行或外部内容")
			}
			for _, attribute := range value.Attr {
				name := strings.ToLower(attribute.Name.Local)
				content := strings.TrimSpace(strings.ToLower(attribute.Value))
				if strings.HasPrefix(name, "on") || name == "src" {
					return invalid("SVG 含事件或外部资源")
				}
				if name == "href" && !strings.HasPrefix(content, "#") {
					return invalid("SVG 不允许外部链接")
				}
				if strings.Contains(content, "javascript:") || strings.Contains(content, "data:") || strings.Contains(content, "http:") || strings.Contains(content, "https:") {
					if name != "xmlns" {
						return invalid("SVG 不允许外部地址")
					}
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	if !root || depth != 0 {
		return invalid("SVG 根元素不完整")
	}
	return nil
}
