package doc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"nice-wiki/internal/doc/nodes"
	"regexp"
)

var blockID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var hashID = regexp.MustCompile(`^[a-f0-9]{64}$`)

func Parse(data []byte) (Document, error) {
	if len(data) > MaxDocumentBytes {
		return Document{}, errors.New("文档超过大小限制")
	}
	var result Document
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("解析文档: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return result, errors.New("文档含尾随数据")
	}
	if err := Validate(result); err != nil {
		return result, err
	}
	return result, nil
}
func IsBlock(name string) bool {
	switch name {
	case "doc", "text", "hardBreak", "inlineMath":
		return false
	default:
		return true
	}
}
func Validate(document Document) error {
	if document.SchemaVersion != SchemaVersion {
		return fmt.Errorf("不支持文档版本 %d", document.SchemaVersion)
	}
	if document.Content.Type != "doc" {
		return errors.New("根节点必须是 doc")
	}
	seen := map[string]bool{}
	count := 0
	var visit func(Node, string, int, bool) error
	visit = func(node Node, parent string, depth int, inColumns bool) error {
		count++
		if count > MaxNodes || depth > MaxDepth {
			return errors.New("文档节点数量或深度超限")
		}
		known := false
		for _, name := range NodeNames {
			if node.Type == name {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("未知节点类型: %s", node.Type)
		}
		if IsBlock(node.Type) {
			id := nodes.String(node, "id")
			if !blockID.MatchString(id) || seen[id] {
				return errors.New("块 ID 缺失、无效或重复")
			}
			seen[id] = true
		}
		if !allowedChild(parent, node.Type) {
			return fmt.Errorf("%s 不能包含 %s", parent, node.Type)
		}
		if node.Type != "text" && node.Text != "" {
			return errors.New("只有 text 节点可以含 text")
		}
		if node.Type == "text" && len(node.Content) != 0 {
			return errors.New("文本节点不能包含子节点")
		}
		if node.Type == "heading" {
			level := nodes.Number(node, "level", 0)
			if level < 2 || level > 4 {
				return errors.New("正文标题仅允许 H2 到 H4")
			}
		}
		if node.Type == "columns" {
			if inColumns {
				return errors.New("超级块不能嵌套")
			}
			if len(node.Content) < 2 || len(node.Content) > 4 {
				return errors.New("超级块需要 2 到 4 栏")
			}
			inColumns = true
		}
		if node.Type == "callout" {
			switch nodes.String(node, "kind") {
			case "info", "tip", "warning", "danger":
			default:
				return errors.New("提示框类型无效")
			}
		}
		if node.Type == "orderedList" && nodes.Number(node, "start", 1) < 1 {
			return errors.New("有序列表起点无效")
		}
		if node.Type == "image" || node.Type == "attachment" || node.Type == "drawing" {
			for _, key := range []string{"hash", "lightHash", "darkHash", "sceneHash"} {
				if value := nodes.String(node, key); value != "" && !hashID.MatchString(value) {
					return errors.New("资源哈希无效")
				}
			}
		}
		if raw, ok := node.Attrs["width"]; ok {
			v, ok := raw.(float64)
			if !ok || math.IsNaN(v) || v < 10 || v > 100 {
				return errors.New("宽度应在 10 到 100 之间")
			}
		}
		if node.Type == "columns" {
			if raw, ok := node.Attrs["widths"]; ok {
				values, ok := raw.([]any)
				if !ok || len(values) != len(node.Content) {
					return errors.New("栏宽数量无效")
				}
				for _, raw := range values {
					v, ok := raw.(float64)
					if !ok || v <= 0 || v > 100 {
						return errors.New("栏宽比例无效")
					}
				}
			}
		}
		for _, mark := range node.Marks {
			known := false
			for _, name := range MarkNames {
				if mark.Type == name {
					known = true
				}
			}
			if !known {
				return fmt.Errorf("未知行内格式: %s", mark.Type)
			}
			if node.Type != "text" {
				return errors.New("行内格式只能附在文本节点")
			}
		}
		for _, child := range node.Content {
			if err := visit(child, node.Type, depth+1, inColumns); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(document.Content, "", 0, false)
}
func allowedChild(parent, child string) bool {
	if parent == "" {
		return child == "doc"
	}
	inline := child == "text" || child == "hardBreak" || child == "inlineMath"
	switch parent {
	case "paragraph", "heading":
		return inline
	case "codeBlock":
		return child == "text"
	case "text", "hardBreak", "horizontalRule", "image", "attachment", "drawing", "video", "inlineMath", "blockMath":
		return false
	case "columns":
		return child == "column"
	case "table":
		return child == "tableRow"
	case "tableRow":
		return child == "tableCell" || child == "tableHeader"
	case "bulletList", "orderedList":
		return child == "listItem"
	case "taskList":
		return child == "taskItem"
	default:
		if inline {
			return false
		}
		switch child {
		case "doc", "column", "tableRow", "tableCell", "tableHeader", "listItem", "taskItem":
			return false
		}
		return true
	}
}
