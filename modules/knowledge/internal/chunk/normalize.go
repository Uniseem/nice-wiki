// Package chunk 提供索引入库前的确定性文本处理规则。
package chunk

import (
	"context"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"nice-wiki/libs/platform"
)

const maxNormalizedSourceBytes = 65536

// OperationContext 是服务间调用的可信操作上下文。
type OperationContext = platform.OperationContext

// NormalizeSourceInput 是规范化纯文本的封闭输入。
type NormalizeSourceInput struct {
	Source              string `json:"source"`
	PreprocessingVersion string `json:"preprocessingVersion"`
}

// NormalizeSourceResult 是规范化文本及其源 UTF-8 字节偏移映射。
// SourceOffsets[i] 是 Normalized 的第 i 个 UTF-8 字节对应的原文起点。
type NormalizeSourceResult struct {
	Normalized    string  `json:"normalized"`
	SourceOffsets []int64 `json:"sourceOffsets"`
}

// NormalizeSource 对索引输入执行固定的 NFKC 规范化和换行统一。
//
// 处理只作用于派生的检索文本，不改变 content 的人工正文。规范化按
// norm.Iter 的稳定边界进行，每个输出字节都保留其来源片段的最早原始
// UTF-8 偏移；因此组合字符和兼容字符折叠时不会丢失引用位置。
func NormalizeSource(ctx context.Context, op OperationContext, input NormalizeSourceInput) (NormalizeSourceResult, error) {
	_ = op
	if err := checkContext(ctx); err != nil {
		return NormalizeSourceResult{}, err
	}
	if !validBoundedText(input.Source) {
		return NormalizeSourceResult{}, platform.NewError(platform.CodeValidation, false, "规范化正文不合法")
	}
	if !validBoundedText(input.PreprocessingVersion) {
		return NormalizeSourceResult{}, platform.NewError(platform.CodeValidation, false, "预处理版本不合法")
	}

	canonical, sourceMap, err := canonicalizeLineEndings(input.Source)
	if err != nil {
		return NormalizeSourceResult{}, err
	}

	var iterator norm.Iter
	iterator.InitString(norm.NFKC, string(canonical))
	normalized := make([]byte, 0, len(canonical))
	offsets := make([]int64, 0, len(canonical))
	for !iterator.Done() {
		if err := checkContext(ctx); err != nil {
			return NormalizeSourceResult{}, err
		}

		start := iterator.Pos()
		segment := iterator.Next()
		end := iterator.Pos()
		if start < 0 || end <= start || end > len(sourceMap) {
			return NormalizeSourceResult{}, platform.NewError(platform.CodeUnsupportedSchema, false, "规范化边界不合法")
		}

		sourceOffset := sourceMap[start]
		for _, candidate := range sourceMap[start:end] {
			if candidate < sourceOffset {
				sourceOffset = candidate
			}
		}
		if len(normalized) > maxNormalizedSourceBytes-len(segment) {
			return NormalizeSourceResult{}, platform.NewError(platform.CodeValidation, false, "规范化正文超过大小上限")
		}
		normalized = append(normalized, segment...)
		for range segment {
			offsets = append(offsets, sourceOffset)
		}
	}

	if len(normalized) == 0 || len(normalized) != len(offsets) || !utf8.Valid(normalized) {
		return NormalizeSourceResult{}, platform.NewError(platform.CodeUnsupportedSchema, false, "规范化结果不合法")
	}
	if err := checkContext(ctx); err != nil {
		return NormalizeSourceResult{}, err
	}
	return NormalizeSourceResult{
		Normalized:    string(normalized),
		SourceOffsets: offsets,
	}, nil
}

func validBoundedText(value string) bool {
	return len(value) >= 1 && len(value) <= maxNormalizedSourceBytes && utf8.ValidString(value)
}

func canonicalizeLineEndings(source string) ([]byte, []int64, error) {
	canonical := make([]byte, 0, len(source))
	sourceMap := make([]int64, 0, len(source))
	for position := 0; position < len(source); {
		if !utf8.FullRuneInString(source[position:]) {
			return nil, nil, platform.NewError(platform.CodeUnsupportedSchema, false, "正文 UTF-8 编码不完整")
		}
		runeValue, width := utf8.DecodeRuneInString(source[position:])
		if runeValue == utf8.RuneError && width == 1 {
			return nil, nil, platform.NewError(platform.CodeUnsupportedSchema, false, "正文 UTF-8 编码不合法")
		}

		if runeValue == '\r' {
			sourceOffset := int64(position)
			if position+width < len(source) && source[position+width] == '\n' {
				position += width + 1
			} else {
				position += width
			}
			canonical = append(canonical, '\n')
			sourceMap = append(sourceMap, sourceOffset)
			continue
		}

		encoded := source[position : position+width]
		canonical = append(canonical, encoded...)
		for range encoded {
			sourceMap = append(sourceMap, int64(position))
		}
		position += width
	}
	return canonical, sourceMap, nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "规范化上下文缺失")
	}
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "规范化已超过截止时间")
		}
		return platform.NewError(platform.CodeCancelled, true, "规范化已取消")
	default:
		return nil
	}
}
