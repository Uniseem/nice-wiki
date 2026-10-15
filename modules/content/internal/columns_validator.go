package content

import (
	"math"

	"nice-wiki/libs/platform"
)

type ValidateColumnsInput struct {
	Columns map[string]any `json:"columns"`
}
type ValidateColumnsResult struct {
	Valid bool `json:"valid"`
}

func ValidateColumns(input ValidateColumnsInput) (ValidateColumnsResult, error) {
	if err := validateColumnsNode(input.Columns); err != nil {
		return ValidateColumnsResult{}, err
	}
	return ValidateColumnsResult{Valid: true}, nil
}

func validateColumnsNode(node map[string]any) error {
	columns := node["content"].([]any)
	ratios := node["attrs"].(map[string]any)["widthRatios"].([]any)
	if len(columns) != len(ratios) {
		return platform.NewError(platform.CodeValidation, false, "分栏比例数量与列不一致")
	}
	total := 0.0
	for _, ratio := range ratios {
		total += contentNumber(ratio)
	}
	if math.Abs(total-1) > 1e-6 {
		return platform.NewError(platform.CodeValidation, false, "分栏比例之和必须为1")
	}
	seen := map[string]bool{}
	for _, columnValue := range columns {
		column := columnValue.(map[string]any)
		id := column["attrs"].(map[string]any)["columnId"].(string)
		if seen[id] {
			return platform.NewError(platform.CodeValidation, false, "分栏ID重复")
		}
		seen[id] = true
		if hasNestedColumns(column) {
			return platform.NewError(platform.CodeValidation, false, "分栏不能嵌套分栏")
		}
	}
	return nil
}

func hasNestedColumns(node map[string]any) bool {
	pending := []map[string]any{node}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current["type"] == "columns" {
			return true
		}
		children, _ := current["content"].([]any)
		for _, child := range children {
			pending = append(pending, child.(map[string]any))
		}
	}
	return false
}
