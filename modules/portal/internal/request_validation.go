package internal

import (
	"context"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
)

var hopByHopHeaders = map[string]bool{
	"connection": true, "keep-alive": true, "proxy-authenticate": true,
	"proxy-authorization": true, "te": true, "trailer": true,
	"transfer-encoding": true, "upgrade": true,
}

var sensitiveHeaders = map[string]bool{
	"authorization": true, "cookie": true, "set-cookie": true,
	"x-portal-proof": true, "x-csrf-token": true,
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

// NormalizeForwardHeaders 删除浏览器不能控制的身份与连接头，只保留
// 路由允许的业务头；调用方随后再注入服务断言。
func NormalizeForwardHeaders(input NormalizeForwardHeadersInput) (NormalizeForwardHeadersResult, error) {
	result := HeaderMap{Entries: make([]HeaderEntry, 0, len(input.Headers.Entries)+4)}
	for _, entry := range input.Headers.Entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" || len(entry.Values) == 0 || hopByHopHeaders[strings.ToLower(name)] || sensitiveHeaders[strings.ToLower(name)] {
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
	return result, nil
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
