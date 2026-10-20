// Package chunk 提供索引入库前的确定性文本处理规则。
package chunk

import (
	"context"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"nice-wiki/libs/platform"
)

const (
	maxNormalizedSourceBytes = 4 << 20
	maxNormalizedSourceRunes = 1 << 20
	normalizeBudget          = 5 * time.Second
)

// OperationContext 是服务间调用的可信操作上下文。
type OperationContext = platform.OperationContext

// NormalizeSourceInput 是规范化纯文本的封闭输入。
type NormalizeSourceInput struct {
	Source               string `json:"source"`
	PreprocessingVersion string `json:"preprocessingVersion"`
	StartByte            int64  `json:"startByte,omitempty"`
}

// NormalizeSourceResult 是规范化文本及其源 UTF-8 字节偏移映射。
// SourceOffsets[i] 是 Normalized 的第 i 个 UTF-8 字节对应的原文起点。
type NormalizeSourceResult struct {
	Normalized      string  `json:"normalized"`
	SourceOffsets   []int64 `json:"sourceOffsets"`
	SourceStartByte int64   `json:"sourceStartByte"`
	SourceByteEnd   int64   `json:"sourceByteEnd"`
	NextByte        *int64  `json:"nextByte,omitempty"`
}

// NormalizeSource 对真实源叶执行固定NFKC和换行统一，并在稳定源边界返回有界片段。
// 非空NextByte必须续读，不能把首片冒充整个原叶的规范化结果。
//
// 处理只作用于派生的检索文本，不改变 content 的人工正文。规范化按
// norm.Iter 的稳定边界进行，每个输出字节都保留其来源片段的最早原始
// UTF-8 偏移；因此组合字符和兼容字符折叠时不会丢失引用位置。
func NormalizeSource(ctx context.Context, op OperationContext, input NormalizeSourceInput) (NormalizeSourceResult, error) {
	if ctx == nil {
		return NormalizeSourceResult{}, platform.NewError(platform.CodeValidation, false, "规范化上下文缺失")
	}
	// 截止 = min(ctx deadline, op.deadline, 本函数 5000ms)（合同步骤5）。
	limit := time.Now().Add(normalizeBudget)
	if op.Deadline != "" {
		deadline, err := platform.ParseTimestamp(op.Deadline)
		if err != nil {
			return NormalizeSourceResult{}, platform.NewError(platform.CodeValidation, false, "规范化截止时间不合法")
		}
		if deadline.Before(limit) {
			limit = deadline
		}
	}
	ctx, cancel := context.WithDeadline(ctx, limit)
	defer cancel()
	if err := checkContext(ctx); err != nil {
		return NormalizeSourceResult{}, err
	}
	if !validBoundedText(input.Source) || input.StartByte < 0 || input.StartByte > int64(len(input.Source)) {
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
	offsets := make([]int64, 0, min(len(canonical), maxNormalizedSourceBytes))
	sourceEnd := input.StartByte
	runeCount := 0
	var nextByte *int64
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
		rawEnd := int64(len(input.Source))
		if end < len(sourceMap) {
			rawEnd = sourceMap[end]
		}
		if rawEnd <= input.StartByte {
			continue
		}
		if sourceOffset < input.StartByte {
			return NormalizeSourceResult{}, platform.NewError(platform.CodeValidation, false, "规范化游标不在真实稳定边界")
		}
		segmentRunes := utf8.RuneCount(segment)
		if len(normalized) > maxNormalizedSourceBytes-len(segment) || runeCount > maxNormalizedSourceRunes-segmentRunes {
			if len(normalized) == 0 {
				return NormalizeSourceResult{}, platform.NewError(platform.CodeValidation, false, "单规范化段超过有界分词预算")
			}
			next := sourceOffset
			nextByte = &next
			break
		}
		sourceEnd = rawEnd
		runeCount += segmentRunes

		normalized = append(normalized, segment...)
		for range segment {
			offsets = append(offsets, sourceOffset)
		}
	}

	if (len(normalized) == 0 && input.StartByte != int64(len(input.Source))) || len(normalized) != len(offsets) || !utf8.Valid(normalized) {
		return NormalizeSourceResult{}, platform.NewError(platform.CodeUnsupportedSchema, false, "规范化结果不合法")
	}
	if err := checkContext(ctx); err != nil {
		return NormalizeSourceResult{}, err
	}
	return NormalizeSourceResult{
		Normalized:      string(normalized),
		SourceOffsets:   offsets,
		SourceStartByte: input.StartByte, SourceByteEnd: sourceEnd, NextByte: nextByte,
	}, nil
}

func validBoundedText(value string) bool {
	return len(value) >= 1 && len(value) <= 8<<20 && utf8.ValidString(value)
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
		for range width {
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
