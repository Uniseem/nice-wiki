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
	"strings"
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
		if err := validateNodeAttributes(node); err != nil {
			return fmt.Errorf("节点 %s: %w", node.Type, err)
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
		markNames := map[string]bool{}
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
			if markNames[mark.Type] {
				return fmt.Errorf("重复行内格式: %s", mark.Type)
			}
			markNames[mark.Type] = true
			if node.Type != "text" {
				return errors.New("行内格式只能附在文本节点")
			}
			if err := validateMark(mark); err != nil {
				return err
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

func validateNodeAttributes(node Node) error {
	switch node.Type {
	case "orderedList":
		if raw, exists := node.Attrs["start"]; exists {
			if value, ok := integer(raw); !ok || value < 1 {
				return errors.New("有序列表起点无效")
			}
		}
	case "callout":
		switch nodes.String(node, "kind") {
		case "info", "tip", "warning", "danger":
		default:
			return errors.New("提示框类型无效")
		}
	case "codeBlock":
		if !regexp.MustCompile(`^[A-Za-z0-9_+-]{0,32}$`).MatchString(nodes.String(node, "language")) {
			return errors.New("代码语言无效")
		}
	case "inlineMath", "blockMath":
		if len(nodes.String(node, "latex")) > 100000 {
			return errors.New("公式内容过长")
		}
	case "image":
		if err := requireHash(node, "hash"); err != nil {
			return err
		}
		if err := requireExtension(node); err != nil {
			return err
		}
		if err := validateImageVariants(node); err != nil {
			return err
		}
	case "attachment":
		if err := requireHash(node, "hash"); err != nil {
			return err
		}
		if err := requireExtension(node); err != nil {
			return err
		}
		if strings.TrimSpace(nodes.String(node, "name")) == "" {
			return errors.New("附件名称不能为空")
		}
		if raw, exists := node.Attrs["size"]; exists {
			if value, ok := integer(raw); !ok || value < 0 {
				return errors.New("附件大小无效")
			}
		}
	case "drawing":
		for _, key := range []string{"sceneHash", "lightHash"} {
			if err := requireHash(node, key); err != nil {
				return err
			}
		}
		if value := nodes.String(node, "darkHash"); value != "" && !hashID.MatchString(value) {
			return errors.New("资源哈希无效")
		}
	case "video":
		switch nodes.String(node, "source") {
		case "upload":
			if err := requireHash(node, "hash"); err != nil {
				return err
			}
			if err := requireExtension(node); err != nil {
				return err
			}
		case "platform":
			if strings.TrimSpace(nodes.String(node, "provider")) == "" || strings.TrimSpace(nodes.String(node, "providerId")) == "" {
				return errors.New("平台视频缺少提供方或视频 ID")
			}
		default:
			return errors.New("视频来源无效")
		}
		if value := nodes.String(node, "posterHash"); value != "" && !hashID.MatchString(value) {
			return errors.New("视频封面哈希无效")
		}
	case "tableCell", "tableHeader":
		for _, key := range []string{"colspan", "rowspan"} {
			if value, exists := node.Attrs[key]; exists {
				span, ok := integer(value)
				if !ok || span < 1 || span > 100 {
					return errors.New("表格跨度无效")
				}
			}
		}
	}
	return nil
}

func requireHash(node Node, key string) error {
	if !hashID.MatchString(nodes.String(node, key)) {
		return fmt.Errorf("%s 资源哈希无效", key)
	}
	return nil
}

func requireExtension(node Node) error {
	if !regexp.MustCompile(`^\.[a-z0-9]{1,16}$`).MatchString(nodes.String(node, "extension")) {
		return errors.New("资源扩展名无效")
	}
	return nil
}

func integer(value any) (int64, bool) {
	switch number := value.(type) {
	case float64:
		if math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number || number > math.MaxInt64 || number < math.MinInt64 {
			return 0, false
		}
		return int64(number), true
	case int:
		return int64(number), true
	case int64:
		return number, true
	default:
		return 0, false
	}
}

func validateImageVariants(node Node) error {
	raw, exists := node.Attrs["variants"]
	if !exists {
		return nil
	}
	values, ok := raw.([]any)
	if !ok || len(values) > 16 {
		return errors.New("图片尺寸列表无效")
	}
	widths := map[int64]bool{}
	for _, rawVariant := range values {
		variant, ok := rawVariant.(map[string]any)
		if !ok {
			return errors.New("图片尺寸项无效")
		}
		hash, _ := variant["hash"].(string)
		extension, _ := variant["extension"].(string)
		width, ok := integer(variant["width"])
		if !hashID.MatchString(hash) || !regexp.MustCompile(`^\.[a-z0-9]{1,16}$`).MatchString(extension) || !ok || width < 1 || width > 10000 || widths[width] {
			return errors.New("图片尺寸项内容无效")
		}
		widths[width] = true
	}
	return nil
}

func validateMark(mark Mark) error {
	switch mark.Type {
	case "link":
		href, ok := mark.Attrs["href"].(string)
		if !ok || strings.TrimSpace(href) == "" {
			return errors.New("链接地址无效")
		}
	case "internalLink":
		documentID, _ := mark.Attrs["documentId"].(string)
		block, _ := mark.Attrs["blockId"].(string)
		if !blockID.MatchString(documentID) || block != "" && !blockID.MatchString(block) {
			return errors.New("站内链接目标无效")
		}
	}
	return nil
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
