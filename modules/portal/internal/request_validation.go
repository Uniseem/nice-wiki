package internal

import (
	"context"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
)

// forwardableHeaders 是浏览器请求中允许原样转发给拥有服务的业务头白名单。
// 身份（Authorization/Cookie/X-CSRF-Token）、转发地址（Forwarded/X-Forwarded-*/
// X-Real-Ip）、hop-by-hop 头与平台可信头（X-Request-Id、X-Operation-Id、
// X-Cost-Class、X-Deadline、X-Cancellation-Id、X-Data-Epoch、X-Portal-Proof 等）
// 一律不转发；可信头只由入口按 OperationContext 重新注入。Accept 由
// ForwardRequest 单独设置，Content-Length 由正文长度决定。
var forwardableHeaders = map[string]bool{
	"content-type": true, "accept-language": true,
	"if-match": true, "if-none-match": true, "if-modified-since": true,
	"if-unmodified-since": true, "if-range": true, "range": true,
	"last-event-id": true, "idempotency-key": true, "x-upload-offset": true, "x-chunk-sha256": true,
	"x-backup-download-token": true, "x-backup-passphrase": true,
}

// ValidateEnvelope 对入口请求执行有界检查；正文句柄只允许由入口生成。
func ValidateEnvelope(ctx context.Context, op OperationContext, input ValidateEnvelopeInput) (ValidateEnvelopeResult, error) {
	if err := ctx.Err(); err != nil {
		return ValidateEnvelopeResult{}, platform.NewError(platform.CodeCancelled, true, "请求已取消")
	}
	if input.Request.Method == "" || input.Request.EscapedPath == "" || input.Request.BodyHandle == "" {
		return ValidateEnvelopeResult{}, platform.NewError(platform.CodeValidation, false, "请求包络不完整")
	}
	if input.Limits.BodyBytes < 0 || input.Limits.HeaderBytes < 1024 || input.Limits.QueryBytes < 0 {
		return ValidateEnvelopeResult{}, platform.NewError(platform.CodeValidation, false, "请求预算不合法")
	}
	if !platform.ValidIdentifier(op.RequestID) || !platform.ValidIdentifier(op.OperationID) {
		return ValidateEnvelopeResult{}, platform.NewError(platform.CodeValidation, false, "请求标识不合法")
	}
	if op.Deadline != "" {
		deadline, err := platform.ParseTimestamp(op.Deadline)
		if err != nil || !deadline.After(time.Now()) {
			return ValidateEnvelopeResult{}, platform.NewError(platform.CodeDeadlineExceeded, true, "请求期限已过")
		}
	}
	if input.Limits.HeaderBytes > 0 && headerBytes(input.Request.Headers) > input.Limits.HeaderBytes {
		return ValidateEnvelopeResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "请求头超出上限")
	}
	bodyBytes, err := contentLength(input.Request.Headers)
	if err != nil {
		return ValidateEnvelopeResult{}, err
	}
	if bodyBytes > input.Limits.BodyBytes {
		return ValidateEnvelopeResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "请求体超出路由上限")
	}
	if bodyBytes > 0 && !supportedMediaType(firstHeader(input.Request.Headers, "Content-Type")) {
		return ValidateEnvelopeResult{}, platform.NewError(platform.CodeValidation, false, "请求媒体类型不受支持")
	}
	if query := firstHeader(input.Request.Headers, "X-Query-Bytes"); query != "" {
		value, parseErr := strconv.ParseInt(query, 10, 64)
		if parseErr != nil || value < 0 || value > input.Limits.QueryBytes {
			return ValidateEnvelopeResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "查询参数超出上限")
		}
	}
	var expected *Revision
	if raw := firstHeader(input.Request.Headers, "If-Match"); raw != "" {
		value, parseErr := strconv.ParseInt(strings.Trim(raw, "\""), 10, 64)
		if parseErr != nil || value < 0 {
			return ValidateEnvelopeResult{}, platform.NewError(platform.CodeValidation, false, "期望版本不合法")
		}
		revision := Revision(value)
		expected = &revision
	}
	return ValidateEnvelopeResult{Envelope: ValidatedEnvelope{
		RequestID:        op.RequestID,
		OperationID:      op.OperationID,
		ExpectedRevision: expected,
		Headers:          input.Request.Headers,
		BodyHandle:       input.Request.BodyHandle,
		BodyDigest:       platform.HashBytes(nil),
		BodyBytes:        bodyBytes,
	}}, nil
}

// NormalizeForwardHeaders 只保留业务头白名单，删除浏览器提交的身份、转发
// 地址、连接与平台可信头，并注入服务器 requestId/operationId；调用方随后
// 再注入服务断言。路由表当前未登记逐路由允许头，统一使用入口白名单。
func NormalizeForwardHeaders(input NormalizeForwardHeadersInput) (NormalizeForwardHeadersResult, error) {
	result := HeaderMap{Entries: make([]HeaderEntry, 0, len(input.Headers.Entries)+4)}
	for _, entry := range input.Headers.Entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" || len(entry.Values) == 0 || !forwardableHeaders[strings.ToLower(name)] {
			continue
		}
		if strings.ContainsAny(name, "\r\n:") {
			return NormalizeForwardHeadersResult{}, platform.NewError(platform.CodeValidation, false, "请求头名称不合法")
		}
		values := make([]string, len(entry.Values))
		for i, value := range entry.Values {
			if strings.ContainsAny(value, "\r\n") || len(value) > 8192 {
				return NormalizeForwardHeadersResult{}, platform.NewError(platform.CodeValidation, false, "请求头值不合法")
			}
			values[i] = value
		}
		result.Entries = append(result.Entries, HeaderEntry{Name: canonicalHeader(name), Values: values})
	}
	result.Entries = append(result.Entries,
		HeaderEntry{Name: platform.HeaderRequestID, Values: []string{input.Envelope.RequestID}},
		HeaderEntry{Name: platform.HeaderOperationID, Values: []string{input.Envelope.OperationID}},
	)
	return NormalizeForwardHeadersResult{Headers: result}, nil
}

func headerBytes(headers HeaderMap) int64 {
	var total int64
	for _, entry := range headers.Entries {
		total += int64(len(entry.Name))
		for _, value := range entry.Values {
			total += int64(len(value))
		}
	}
	return total
}

func contentLength(headers HeaderMap) (int64, error) {
	raw := firstHeader(headers, "Content-Length")
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, platform.NewError(platform.CodeValidation, false, "Content-Length不合法")
	}
	return value, nil
}

func supportedMediaType(raw string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(raw, ";", 2)[0]))
	switch mediaType {
	case "application/json", "application/problem+json", "application/octet-stream", "text/plain", "text/event-stream", "":
		return true
	default:
		return false
	}
}

func firstHeader(headers HeaderMap, wanted string) string {
	for _, entry := range headers.Entries {
		if strings.EqualFold(entry.Name, wanted) && len(entry.Values) > 0 {
			return entry.Values[0]
		}
	}
	return ""
}

func canonicalHeader(name string) string {
	parts := strings.Split(name, "-")
	for i := range parts {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
		}
	}
	return strings.Join(parts, "-")
}
