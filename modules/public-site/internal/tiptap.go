package publicsite

import (
	"context"
	"encoding/json"
	"sync"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/modules/public-site/internal/gen"
)

// TiptapDocument 保留生产方节点和属性；渲染只读，不改写正文文件。
type TiptapDocument struct {
	SchemaVersion int          `json:"schemaVersion"`
	Type          string       `json:"type"`
	Content       []TiptapNode `json:"content"`
}
type TiptapNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Text    string         `json:"text,omitempty"`
	Content []TiptapNode   `json:"content,omitempty"`
	Marks   []TiptapMark   `json:"marks,omitempty"`
}
type TiptapMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}
type AssetVersionRef struct {
	AssetID   string `json:"assetId"`
	Version   int    `json:"version"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"mediaType"`
	ByteSize  int64  `json:"byteSize"`
}
type ParseTiptapDocumentInput struct {
	Document json.RawMessage
	MaxBytes int64
}
type ParseTiptapDocumentResult struct {
	Document  TiptapDocument
	Headings  []Heading
	MediaRefs []AssetVersionRef
}

var documentValidator = sync.OnceValues(func() (func(json.RawMessage) error, error) {
	return apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "PublishedDocumentBody")
})

// ParseTiptapDocument 在正文边界按 content 唯一 schema 验证一次，并核树深度及稳定标识。
func ParseTiptapDocument(ctx context.Context, op platform.OperationContext, in ParseTiptapDocumentInput) (ParseTiptapDocumentResult, error) {
	if int64(len(in.Document)) > in.MaxBytes {
		return ParseTiptapDocumentResult{}, platform.NewError(platform.CodeValidation, false, "BODY_TOO_LARGE")
	}
	validate, err := documentValidator()
	if err != nil {
		return ParseTiptapDocumentResult{}, err
	}
	if err = validate(in.Document); err != nil {
		return ParseTiptapDocumentResult{}, err
	}
	var document TiptapDocument
	if err = json.Unmarshal(in.Document, &document); err != nil {
		return ParseTiptapDocumentResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_DOCUMENT")
	}
	headings := make([]Heading, 0)
	assets := make([]AssetVersionRef, 0)
	seenIDs := make(map[string]bool)
	seenAssets := make(map[string]bool)
	var collectAsset func(any) error
	collectAsset = func(value any) error {
		switch item := value.(type) {
		case map[string]any:
			if id, ok := item["assetId"].(string); ok {
				raw, err := json.Marshal(item)
				if err != nil {
					return err
				}
				var asset AssetVersionRef
				if err = json.Unmarshal(raw, &asset); err != nil {
					return err
				}
				key := id + ":" + asset.SHA256
				if !seenAssets[key] {
					seenAssets[key] = true
					assets = append(assets, asset)
				}
				return nil
			}
			for _, child := range item {
				if err := collectAsset(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range item {
				if err := collectAsset(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	var visit func(TiptapNode, int, bool) error
	visit = func(node TiptapNode, depth int, inColumns bool) error {
		if depth > 64 {
			return platform.NewError(platform.CodeValidation, false, "DOCUMENT_TOO_DEEP")
		}
		for _, key := range []string{"blockId", "columnId", "cellId"} {
			if id := attrString(node.Attrs, key); id != "" {
				if seenIDs[id] {
					return platform.NewError(platform.CodeIntegrityError, false, "DUPLICATE_BLOCK_ID")
				}
				seenIDs[id] = true
			}
		}
		if node.Type == "heading" {
			headings = append(headings, Heading{Level: attrInt(node.Attrs, "level"), Text: nodeText(node), BlockID: attrString(node.Attrs, "blockId")})
		}
		if node.Type == "columns" && inColumns {
			return platform.NewError(platform.CodeValidation, false, "NESTED_COLUMNS")
		}
		if err := collectAsset(node.Attrs); err != nil {
			return err
		}
		for _, child := range node.Content {
			if err := visit(child, depth+1, inColumns || node.Type == "columns"); err != nil {
				return err
			}
		}
		return nil
	}
	for _, node := range document.Content {
		if err = visit(node, 1, false); err != nil {
			return ParseTiptapDocumentResult{}, err
		}
	}
	return ParseTiptapDocumentResult{document, headings, assets}, nil
}

func attrString(attrs map[string]any, key string) string {
	value, _ := attrs[key].(string)
	return value
}
func attrInt(attrs map[string]any, key string) int {
	value, _ := attrs[key].(float64)
	return int(value)
}
func nodeText(node TiptapNode) string {
	result := node.Text
	if node.Type == "inlineMath" || node.Type == "mathBlock" {
		result += attrString(node.Attrs, "latex")
	}
	for _, child := range node.Content {
		result += nodeText(child)
	}
	return result
}
func assetFrom(value any) (AssetVersionRef, bool) {
	raw, err := json.Marshal(value)
	if err != nil {
		return AssetVersionRef{}, false
	}
	var asset AssetVersionRef
	if json.Unmarshal(raw, &asset) != nil || asset.AssetID == "" {
		return AssetVersionRef{}, false
	}
	return asset, true
}
