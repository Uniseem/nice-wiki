package internal

import (
	"context"
	"strings"

	"nice-wiki/libs/platform"
)

// RedactedAuditRequest 只含合同结果类型写明的访问审计字段。
type RedactedAuditRequest struct {
	RequestID Identifier
	RouteID   Identifier
	Method    string
	BodyBytes int64
}

// RedactRequestInput 是纯函数输入，不携带可调用的外部客户端。
type RedactRequestInput struct {
	Request ValidatedEnvelope
	Route   ResolvedRoute
}

// RedactRequestResult 在无错误时才有效。
type RedactRequestResult struct {
	Audit RedactedAuditRequest
}

// RedactRequest 按静态路由生成最小访问审计字段。
// 保留 requestId、routeId、method 与体字节数，剥除 cookie 和 authorization。
// query 敏感键、路径变量敏感标识以及其他请求头没有可执行规则，出现时失败关闭。
func RedactRequest(input RedactRequestInput) (RedactRequestResult, error) {
	requestID := input.Request.RequestID
	routeID := input.Route.Definition.RouteID
	method := input.Route.Definition.Method
	bodyBytes := input.Request.BodyBytes
	if !platform.ValidIdentifier(requestID) || !platform.ValidIdentifier(routeID) {
		return RedactRequestResult{}, platform.NewError(platform.CodeValidation, false, "请求或路由标识不合法")
	}
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		return RedactRequestResult{}, platform.NewError(platform.CodeValidation, false, "路由方法不合法")
	}
	if bodyBytes < 0 || bodyBytes > 2147483648 {
		return RedactRequestResult{}, platform.NewError(platform.CodeValidation, false, "请求体字节数不合法")
	}
	if err := classifyAuditHeaders(input.Request.Headers); err != nil {
		return RedactRequestResult{}, err
	}
	if err := classifyAuditQuery(input.Route); err != nil {
		return RedactRequestResult{}, err
	}
	if err := classifyAuditPathVariables(input.Route); err != nil {
		return RedactRequestResult{}, err
	}
	return RedactRequestResult{Audit: RedactedAuditRequest{
		RequestID: requestID,
		RouteID:   routeID,
		Method:    method,
		BodyBytes: bodyBytes,
	}}, nil
}

func classifyAuditHeaders(headers HeaderMap) error {
	if len(headers.Entries) > 100 {
		return platform.NewError(platform.CodeValidation, false, "请求头数量超限")
	}
	for _, entry := range headers.Entries {
		if entry.Name == "" || len(entry.Name) > 128 || strings.ContainsAny(entry.Name, "\r\n:") {
			return platform.NewError(platform.CodeValidation, false, "请求头名称不合法")
		}
		if len(entry.Values) == 0 || len(entry.Values) > 20 {
			return platform.NewError(platform.CodeValidation, false, "请求头值数量不合法")
		}
		for _, value := range entry.Values {
			if len(value) > 8192 || strings.ContainsAny(value, "\r\n") {
				return platform.NewError(platform.CodeValidation, false, "请求头值不合法")
			}
		}
		if strings.EqualFold(entry.Name, "cookie") || strings.EqualFold(entry.Name, "authorization") {
			continue
		}
		return platform.NewError(platform.CodeValidation, false, "存在合同未写明应剥除或保留的请求头")
	}
	return nil
}

func classifyAuditQuery(route ResolvedRoute) error {
	if route.RelativePath == "" || len(route.RelativePath) > 2048 {
		return platform.NewError(platform.CodeValidation, false, "上游路径不合法")
	}
	if route.Definition.PathTemplate == "" || len(route.Definition.PathTemplate) > 512 {
		return platform.NewError(platform.CodeValidation, false, "路由路径模板不合法")
	}
	if strings.Contains(route.RelativePath, "?") || strings.Contains(route.Definition.PathTemplate, "?") {
		return platform.NewError(platform.CodeValidation, false, "query 的敏感键合同未写明")
	}
	return nil
}

func classifyAuditPathVariables(route ResolvedRoute) error {
	if len(route.Parameters) > 10 {
		return platform.NewError(platform.CodeValidation, false, "路径变量数量超限")
	}
	if len(route.Parameters) > 0 || strings.Contains(route.Definition.PathTemplate, "{") {
		return platform.NewError(platform.CodeValidation, false, "路径变量的敏感标识合同未写明")
	}
	return nil
}

// RequestOutcome 是已结束请求的结果。成功时 Code 为 nil。
type RequestOutcome struct {
	Status     int
	DurationMs int64
	Code       *string
}

// EmitAccessAuditInput 只接受已经脱敏的访问字段和结果。
type EmitAccessAuditInput struct {
	Request RedactedAuditRequest
	Outcome RequestOutcome
}

// EmitAccessAuditResult 只有 accepted。合同里的 degraded 没有对应字段。
type EmitAccessAuditResult struct {
	Accepted bool
}

// EmitAccessAudit 投递入口访问结果。
// 输入已经是 RedactedAuditRequest，不再读取原始 HTTP，也不把 cookie、authorization、正文或原始地址写入结果。
// analytics:appendAudit 的请求体必填 auditId、action、resourceId、resultCode、metadata，
// 与 requestId、routeId、method、bodyBytes、status、durationMs、code 没有已写明的一一映射。
// 因此不构造请求体，也不调用客户端。requestId 只在映射写明后才能作为 idempotencyKey。
// 结果没有 degraded 字段；失败返回错误，不用 accepted=false 表示成功。
func EmitAccessAudit(ctx context.Context, op OperationContext, input EmitAccessAuditInput) (EmitAccessAuditResult, error) {
	return EmitAccessAuditResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "appendAudit 请求体与 RedactedAuditRequest、RequestOutcome 没有已写明的一一映射")
}
