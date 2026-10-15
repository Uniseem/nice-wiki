package content

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"nice-wiki/libs/platform"
)

type ValidateBlockTreeInput struct {
	Body         DocumentBody  `json:"body"`
	DocumentID   Identifier    `json:"documentId"`
	PreviousBody *DocumentBody `json:"previousBody"`
}
type ValidateBlockTreeResult struct {
	Valid    bool         `json:"valid"`
	BlockIDs []Identifier `json:"blockIds"`
}
type ValidateInlineInput struct {
	Nodes []any `json:"nodes"`
}
type ValidateInlineResult struct {
	Valid bool `json:"valid"`
}

type blockTreeValidation struct {
	seen map[string]bool
	ids  []Identifier
}

// ValidateBlockTree 核本树语义与所有节点ID；历史退役ID由提交事务的ledger核验。
func ValidateBlockTree(ctx context.Context, op OperationContext, input ValidateBlockTreeInput) (ValidateBlockTreeResult, error) {
	value, err := bodyJSONValue(input.Body)
	if err != nil {
		return ValidateBlockTreeResult{}, err
	}
	state := blockTreeValidation{seen: map[string]bool{}, ids: make([]Identifier, 0)}
	for _, child := range value.(map[string]any)["content"].([]any) {
		if err := state.visit(child.(map[string]any), 1); err != nil {
			return ValidateBlockTreeResult{}, err
		}
	}
	return ValidateBlockTreeResult{Valid: true, BlockIDs: state.ids}, nil
}

func (state *blockTreeValidation) visit(node map[string]any, depth int) error {
	if depth > 32 {
		return platform.NewError(platform.CodeValidation, false, "正文块树超过32层")
	}
	attrs, _ := node["attrs"].(map[string]any)
	for _, key := range []string{"blockId", "itemId", "cellId", "rowId", "columnId"} {
		if id, ok := attrs[key].(string); ok {
			if state.seen[id] {
				return platform.NewError(platform.CodeValidation, false, "正文稳定ID重复")
			}
			state.seen[id] = true
			state.ids = append(state.ids, id)
			if len(state.ids) > 10000 {
				return platform.NewError(platform.CodeValidation, false, "正文节点总数超过10000")
			}
		}
	}
	kind := node["type"].(string)
	switch kind {
	case "paragraph", "heading":
		_, err := ValidateInline(ValidateInlineInput{Nodes: node["content"].([]any)})
		return err
	case "codeBlock":
		// 代码保留原文，不作为HTML，也不按普通行内文本改写。
		return nil
	case "mathBlock":
		return validateContentMath(attrs["latex"].(string))
	case "listItem", "taskItem":
		children := node["content"].([]any)
		if children[0].(map[string]any)["type"] != "paragraph" {
			return platform.NewError(platform.CodeValidation, false, "列表项首块必须为段落")
		}
	case "table":
		if _, err := ValidateTableGrid(ValidateTableGridInput{Table: node}); err != nil {
			return err
		}
	case "columns":
		if _, err := ValidateColumns(ValidateColumnsInput{Columns: node}); err != nil {
			return err
		}
	case "image", "drawing":
		if _, err := ValidateInline(ValidateInlineInput{Nodes: attrs["caption"].([]any)}); err != nil {
			return err
		}
	}
	children, _ := node["content"].([]any)
	for _, child := range children {
		if err := state.visit(child.(map[string]any), depth+1); err != nil {
			return err
		}
	}
	return nil
}

func ValidateInline(input ValidateInlineInput) (ValidateInlineResult, error) {
	if err := validateInlineNodes(input.Nodes); err != nil {
		return ValidateInlineResult{}, err
	}
	return ValidateInlineResult{Valid: true}, nil
}

func validateInlineNodes(nodes []any) error {
	for _, value := range nodes {
		node := value.(map[string]any)
		switch node["type"] {
		case "text":
			if strings.ContainsRune(node["text"].(string), 0) {
				return platform.NewError(platform.CodeValidation, false, "行内文本不能包含NUL")
			}
			seen := map[string]bool{}
			for _, markValue := range node["marks"].([]any) {
				mark := markValue.(map[string]any)
				kind := mark["type"].(string)
				if seen[kind] {
					return platform.NewError(platform.CodeValidation, false, "同种行内格式不能重复")
				}
				seen[kind] = true
				if kind == "link" {
					target := mark["attrs"].(map[string]any)["target"].(map[string]any)
					if target["kind"] == "external" {
						if err := validateContentLink(target["href"].(string)); err != nil {
							return err
						}
					}
				}
			}
		case "inlineMath":
			if err := validateContentMath(node["attrs"].(map[string]any)["latex"].(string)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateContentLink(href string) error {
	parsed, err := url.Parse(href)
	if err != nil || parsed.User != nil {
		return platform.NewError(platform.CodeValidation, false, "外链必须是无用户凭据的安全URL")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		if parsed.Host != "" {
			return nil
		}
	case "mailto":
		if parsed.Opaque != "" || parsed.Path != "" {
			return nil
		}
	}
	return platform.NewError(platform.CodeValidation, false, "外链只允许http、https或mailto")
}

var contentMathCommands = regexp.MustCompile(`\\([A-Za-z]+)`)

func validateContentMath(latex string) error {
	for _, match := range contentMathCommands.FindAllStringSubmatch(latex, -1) {
		switch strings.ToLower(match[1]) {
		case "input", "include", "includegraphics", "openin", "openout", "read", "write", "href", "url", "htmlclass", "htmlid", "htmlstyle", "htmldata":
			return platform.NewError(platform.CodeValidation, false, "公式不允许文件、网络或HTML宏")
		}
	}
	return nil
}

func contentInteger(value any) int    { return int(contentNumber(value)) }
func contentNumber(value any) float64 { number, _ := value.(json.Number).Float64(); return number }
