package content

import (
	"bytes"
	"encoding/json"
	"sort"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type DocumentBody = gen.DocumentBody
type CanonicalizeBodyInput struct {
	Body DocumentBody `json:"body"`
}
type CanonicalizeBodyResult struct {
	UTF8        string `json:"utf8"`
	ContentHash Hash   `json:"contentHash"`
}
type HashBlockInput struct {
	Block gen.Block `json:"block"`
}
type HashBlockResult struct {
	BlockHash Hash `json:"blockHash"`
}

// CanonicalizeBody 保留文字与数组顺序，只规范键及可交换的mark类型顺序。
func CanonicalizeBody(input CanonicalizeBodyInput) (CanonicalizeBodyResult, error) {
	value, err := bodyJSONValue(input.Body)
	if err != nil {
		return CanonicalizeBodyResult{}, err
	}
	raw, err := canonicalContentValue(value)
	if err != nil {
		return CanonicalizeBodyResult{}, err
	}
	if len(raw) > maxBodyBytes {
		return CanonicalizeBodyResult{}, platform.NewError(platform.CodeValidation, false, "正文规范JSON超过硬上限")
	}
	return CanonicalizeBodyResult{UTF8: string(raw), ContentHash: platform.HashBytes(raw)}, nil
}

// HashBlock 包含稳定ID、全部呈现属性、不可变资源引用及有序后代。
func HashBlock(input HashBlockInput) (HashBlockResult, error) {
	value, err := bodyJSONValue(input.Block)
	if err != nil {
		return HashBlockResult{}, err
	}
	raw, err := canonicalContentValue(value)
	if err != nil {
		return HashBlockResult{}, err
	}
	return HashBlockResult{BlockHash: platform.HashBytes(raw)}, nil
}

func bodyJSONValue(input any) (any, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "正文值无法编码")
	}
	if len(raw) > maxBodyBytes {
		return nil, platform.NewError(platform.CodeValidation, false, "正文JSON超过硬上限")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	// raw刚由Marshal生成；此处只转换表示，不重复校验schema。
	_ = decoder.Decode(&value)
	return value, nil
}

func canonicalContentValue(value any) ([]byte, error) {
	orderContentMarks(value)
	return platform.CanonicalJSON(value)
}

func orderContentMarks(value any) {
	switch node := value.(type) {
	case map[string]any:
		if marks, ok := node["marks"].([]any); ok {
			sort.SliceStable(marks, func(i, j int) bool {
				return marks[i].(map[string]any)["type"].(string) < marks[j].(map[string]any)["type"].(string)
			})
		}
		for _, child := range node {
			orderContentMarks(child)
		}
	case []any:
		for _, child := range node {
			orderContentMarks(child)
		}
	}
}
