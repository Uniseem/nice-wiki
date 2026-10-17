package analytics

import (
	"context"

	"nice-wiki/libs/platform"
)

// CheckPrivacyFieldsInput 是日志与导出字段名核对输入。只含字段名，不含字段值。
type CheckPrivacyFieldsInput struct {
	FieldNames []string
}

// CheckPrivacyFieldsResult 是字段名核对结果。失败时返回零值，不附带部分名单。
type CheckPrivacyFieldsResult struct {
	Allowed   bool
	Forbidden []string
}

// RedactAuditMetadataInput 是审计元数据脱敏输入。metadata 的编码合同未写明。
type RedactAuditMetadataInput struct {
	Action   string
	Metadata string
}

// RedactAuditMetadataResult 是允许输出的元数据。失败时 SafeMetadata 保持空串。
type RedactAuditMetadataResult struct {
	SafeMetadata string
}

// privacyNameAllowed 只承认 M14-R016 中已经出现的英文标识符。
// 不把“模块”“状态”“耗时”翻译成英文字段名。
func privacyNameAllowed(name string) bool {
	return name == "requestId" || name == "errorCode"
}

// privacyNameProhibited 按合同原文做区分大小写的禁止名判断。
func privacyNameProhibited(name string) bool {
	switch name {
	case "ip", "peer", "question", "prompt", "apiKey", "token", "referrer", "query":
		return true
	default:
		return false
	}
}

// CheckPrivacyFields 核对日志与导出字段名。
// 允许名只有 requestId 与 errorCode。空列表视为允许，forbidden 为空切片。
// 错误表把禁止字段定为 FORBIDDEN/PRIVACY_FIELD_FORBIDDEN，因此出现合同原文中的
// 禁止名时返回该错误和零值，不同时返回半成功名单。
// 其余非允许名写入 forbidden，只含字段名，allowed 为 false，不返回错误。
func CheckPrivacyFields(ctx context.Context, op OperationContext, in CheckPrivacyFieldsInput) (CheckPrivacyFieldsResult, error) {
	_ = ctx
	_ = op
	for _, name := range in.FieldNames {
		if privacyNameProhibited(name) {
			return CheckPrivacyFieldsResult{}, platform.NewError(platform.CodeForbidden, false, "隐私字段被禁止")
		}
	}
	forbidden := make([]string, 0)
	for _, name := range in.FieldNames {
		if privacyNameAllowed(name) {
			continue
		}
		forbidden = append(forbidden, name)
	}
	if len(forbidden) == 0 {
		return CheckPrivacyFieldsResult{Allowed: true, Forbidden: forbidden}, nil
	}
	return CheckPrivacyFieldsResult{Allowed: false, Forbidden: forbidden}, nil
}

// RedactAuditMetadata 因审计动作白名单与 metadata 编码都没有写明而保持失败关闭。
// 不猜测 JSON，不编造动作名，也不把原文 metadata 放入 safeMetadata 或错误说明。
// 对任何输入都返回登记的 FORBIDDEN。
func RedactAuditMetadata(ctx context.Context, op OperationContext, in RedactAuditMetadataInput) (RedactAuditMetadataResult, error) {
	_ = ctx
	_ = op
	_ = in
	return RedactAuditMetadataResult{}, platform.NewError(platform.CodeForbidden, false, "审计动作白名单与元数据编码未写明")
}
