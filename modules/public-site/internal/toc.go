package publicsite

import (
	"context"

	"nice-wiki/libs/platform"
)

// Heading 是 AST 标题节点。目录锚点取 BlockID，标签取 Text。
type Heading struct {
	Level   int
	Text    string
	Anchor  string
	BlockID string
}

// TOCItem 是正文目录项。
type TOCItem struct {
	Level  int
	Label  string
	Anchor string
}

// BuildTOCInput 是生成稳定锚点目录的输入。
type BuildTOCInput struct {
	Headings []Heading
	MaxDepth int64
}

// BuildTOCResult 是生成稳定锚点目录的成功结果。失败时返回零值。
type BuildTOCResult struct {
	Items []TOCItem
}

// BuildTOC 按输入顺序生成 H2–H4 目录。
// maxDepth 不是 4，或标题级别不是 1–4 时，返回 INVALID_TOC_DEPTH。
// 级别 1 不进入目录。任意重复 blockId 在级别均合法时返回 INTEGRITY_ERROR。
func BuildTOC(ctx context.Context, op platform.OperationContext, in BuildTOCInput) (BuildTOCResult, error) {
	_ = ctx
	_ = op
	if in.MaxDepth != 4 {
		return BuildTOCResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_TOC_DEPTH")
	}
	seen := make(map[string]struct{}, len(in.Headings))
	invalidLevel := false
	duplicate := false
	for _, heading := range in.Headings {
		if heading.Level < 1 || heading.Level > 4 {
			invalidLevel = true
		}
		if _, ok := seen[heading.BlockID]; ok {
			duplicate = true
			continue
		}
		seen[heading.BlockID] = struct{}{}
	}
	if invalidLevel {
		return BuildTOCResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_TOC_DEPTH")
	}
	if duplicate {
		return BuildTOCResult{}, platform.NewError(platform.CodeIntegrityError, false, "INTEGRITY_ERROR")
	}
	items := make([]TOCItem, 0)
	for _, heading := range in.Headings {
		if heading.Level == 1 {
			continue
		}
		items = append(items, TOCItem{
			Level:  heading.Level,
			Label:  heading.Text,
			Anchor: heading.BlockID,
		})
	}
	return BuildTOCResult{Items: items}, nil
}
