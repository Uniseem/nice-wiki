package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

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

// RedactAuditMetadataInput 是严格安全JSON元数据输入。
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

// RedactAuditMetadata只输出固定动作的安全结构化元数据。
// metadata采用严格白名单JSON对象；未知字段拒绝，原文不进入输出。
func RedactAuditMetadata(ctx context.Context, op OperationContext, in RedactAuditMetadataInput) (RedactAuditMetadataResult, error) {
	if _, ok := auditActionProducer[in.Action]; !ok {
		return RedactAuditMetadataResult{}, platform.NewError(platform.CodeForbidden, false, "审计动作未登记")
	}
	if len(in.Metadata) > 2048 {
		return RedactAuditMetadataResult{}, platform.NewError(platform.CodeValidation, false, "审计元数据超过限制")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(in.Metadata))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return RedactAuditMetadataResult{}, platform.NewError(platform.CodeValidation, false, "审计元数据必须为对象")
	}
	values := map[string]any{}
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return RedactAuditMetadataResult{}, platform.NewError(platform.CodeValidation, false, "审计元数据不合法")
		}
		key := token.(string)
		if _, exists := values[key]; exists {
			return RedactAuditMetadataResult{}, platform.NewError(platform.CodeValidation, false, "审计元数据字段重复")
		}
		var raw json.RawMessage
		if err = decoder.Decode(&raw); err != nil {
			return RedactAuditMetadataResult{}, platform.NewError(platform.CodeValidation, false, "审计元数据不合法")
		}
		switch key {
		case "revision", "count":
			var n int64
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &n) != nil || n < 0 {
				return RedactAuditMetadataResult{}, platform.NewError(platform.CodeValidation, false, "审计数值不合法")
			}
			values[key] = n
		case "fromState", "toState":
			var state string
			if json.Unmarshal(raw, &state) != nil || !auditStateAllowed(state) {
				return RedactAuditMetadataResult{}, platform.NewError(platform.CodeValidation, false, "审计状态未登记")
			}
			values[key] = state
		default:
			return RedactAuditMetadataResult{}, platform.NewError(platform.CodeForbidden, false, "审计元数据字段未登记")
		}
	}
	if _, err = decoder.Token(); err != nil {
		return RedactAuditMetadataResult{}, platform.NewError(platform.CodeValidation, false, "审计元数据不完整")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return RedactAuditMetadataResult{}, platform.NewError(platform.CodeValidation, false, "审计元数据包含尾随内容")
	}
	safe, err := platform.CanonicalJSON(values)
	if err != nil {
		return RedactAuditMetadataResult{}, err
	}
	return RedactAuditMetadataResult{SafeMetadata: string(safe)}, nil
}

func auditStateAllowed(state string) bool {
	switch state {
	case "requested", "staging", "switching", "blocked", "rolling-back", "rolled-back", "archived", "retired", "backup_barrier", "restore_staging", "restore_switching", "restore_rolling_back", "restore_blocked", "planned", "queued", "leased", "running", "succeeded", "failed", "cancelled", "completed", "active", "disabled", "draft", "published", "withdrawn", "deleted", "restored", "ready", "expired", "settled", "reversed", "held", "charge_unknown", "enabled", "pending", "approved", "rejected", "draining", "safe", "released", "staged", "switched", "rolled_back":
		return true
	}
	return false
}
