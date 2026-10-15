package content

import (
	"nice-wiki/libs/platform"
)

type ValidateTableGridInput struct {
	Table map[string]any `json:"table"`
}
type ValidateTableGridResult struct {
	Rows    int `json:"rows"`
	Columns int `json:"columns"`
}

func ValidateTableGrid(input ValidateTableGridInput) (ValidateTableGridResult, error) {
	return validateTableGridNode(input.Table)
}

// validateTableGridNode 按行放起始单元格，所有被跨行覆盖的位置也必须有唯一来源。
func validateTableGridNode(table map[string]any) (ValidateTableGridResult, error) {
	rows := table["content"].([]any)
	grid := make([][30]bool, len(rows))
	width := 0
	seen := map[string]bool{}
	for rowIndex, rowValue := range rows {
		column := 0
		for _, cellValue := range rowValue.(map[string]any)["content"].([]any) {
			cell := cellValue.(map[string]any)
			attrs := cell["attrs"].(map[string]any)
			id := attrs["cellId"].(string)
			if seen[id] {
				return ValidateTableGridResult{}, platform.NewError(platform.CodeValidation, false, "表格单元格ID重复")
			}
			seen[id] = true
			rowspan, colspan := contentInteger(attrs["rowspan"]), contentInteger(attrs["colspan"])
			for column < 30 && grid[rowIndex][column] {
				column++
			}
			if rowIndex+rowspan > len(rows) || column+colspan > 30 {
				return ValidateTableGridResult{}, platform.NewError(platform.CodeValidation, false, "表格跨度超出边界")
			}
			if widths, ok := attrs["colwidth"].([]any); ok && len(widths) != colspan {
				return ValidateTableGridResult{}, platform.NewError(platform.CodeValidation, false, "表格列宽数量与跨度不一致")
			}
			for r := rowIndex; r < rowIndex+rowspan; r++ {
				for c := column; c < column+colspan; c++ {
					if grid[r][c] {
						return ValidateTableGridResult{}, platform.NewError(platform.CodeValidation, false, "表格单元格覆盖重叠")
					}
					grid[r][c] = true
				}
			}
			column += colspan
			if column > width {
				width = column
			}
		}
	}
	if width == 0 {
		return ValidateTableGridResult{}, platform.NewError(platform.CodeValidation, false, "表格必须覆盖至少一列")
	}
	for _, row := range grid {
		for column := 0; column < width; column++ {
			if !row[column] {
				return ValidateTableGridResult{}, platform.NewError(platform.CodeValidation, false, "表格单元格覆盖存在空洞")
			}
		}
	}
	return ValidateTableGridResult{Rows: len(rows), Columns: width}, nil
}
