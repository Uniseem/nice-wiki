// Package language 实现语言配置的标识、写入与列表。
// 本文件只拒绝已写明的空标签和长度；大小写、脚本与私用归一化没有可执行规则。
package language

import (
	"context"

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

// NormalizeLanguageTag 拒绝空标签和超过 35 的标签。
// 合同没有写明大小写、脚本子标签和私用用法如何变成规范形式，
// 因此其余输入失败关闭，不返回改写后的标签或方向。
func NormalizeLanguageTag(ctx context.Context, op OperationContext, input NormalizeLanguageTagInput) (NormalizeLanguageTagResult, error) {
	_ = ctx
	_ = op
	if input.Tag == "" || len(input.Tag) > 35 {
		return NormalizeLanguageTagResult{}, platform.NewError(platform.CodeValidation, false, "语言标签为空或超过35")
	}
	return NormalizeLanguageTagResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "语言标签大小写、脚本与私用归一化尚未在合同写明")
}
