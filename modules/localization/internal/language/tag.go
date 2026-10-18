// Package language 实现语言配置的标识、写入与列表。
// 本文件只拒绝已写明的空标签和长度；大小写、脚本与私用归一化没有可执行规则。
package language

import (
	"context"
	textlanguage "golang.org/x/text/language"
	"strings"

	"nice-wiki/libs/platform"
)

// OperationContext 是平台可信执行上下文的本域别名。
type OperationContext = platform.OperationContext

// NormalizeLanguageTagInput 对应 types.json。该函数没有 HTTP 操作，生成客户端不包含这个类型。
type NormalizeLanguageTagInput struct {
	Tag string `json:"tag"`
}

// NormalizeLanguageTagResult 对应 types.json。失败时结果无效。
type NormalizeLanguageTagResult struct {
	Tag       string `json:"tag"`
	Direction string `json:"direction"`
}

// NormalizeLanguageTag 用已批准的 x/text/language 规范BCP47；配置展示标签仍按用户原文保存。
func NormalizeLanguageTag(ctx context.Context, op OperationContext, input NormalizeLanguageTagInput) (NormalizeLanguageTagResult, error) {
	if input.Tag == "" || len(input.Tag) > 35 || strings.Contains(input.Tag, "_") {
		return NormalizeLanguageTagResult{}, platform.NewError(platform.CodeValidation, false, "语言标签为空、超过35或使用非BCP47分隔符")
	}
	tag, err := textlanguage.Parse(input.Tag)
	if err != nil {
		return NormalizeLanguageTagResult{}, platform.NewError(platform.CodeValidation, false, "语言标签不是有效BCP47")
	}
	normalized := tag.String()
	if len(normalized) > 35 {
		return NormalizeLanguageTagResult{}, platform.NewError(platform.CodeValidation, false, "规范语言标签超过35")
	}
	base, _ := tag.Base()
	direction := "ltr"
	switch base.String() {
	case "ar", "he", "fa", "ur":
		direction = "rtl"
	}
	return NormalizeLanguageTagResult{Tag: normalized, Direction: direction}, nil
}
