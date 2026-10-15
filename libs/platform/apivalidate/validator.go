// Package apivalidate 在 HTTP 入口按本域生成代码内嵌的 OpenAPI 规格统一校验请求
// （ADR0005 kin-openapi；ADR0006 边界一次校验）。入口之后的函数信任已校验的输入。
package apivalidate

import (
	"errors"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
)

// maxIssues 限制一次返回的字段问题数量。
const maxIssues = 20

// NewRequestValidator 用本域生成代码 gen.GetSwagger() 返回的规格构建请求校验中间件。
// 路径、查询、请求头参数与请求体按规格校验（additionalProperties:false 拒绝未知字段，
// 长度、范围、枚举与必填按 schema），失败返回 VALIDATION Problem，字段问题只给位置与
// schema 关键字，不回显取值。服务身份、委托与运行代次由 opctx.BuildOperationContext
// 校验，这里不重复；规格未登记的路径交给后续路由返回 404/405。
func NewRequestValidator(spec *openapi3.T) (func(http.Handler) http.Handler, error) {
	if spec == nil {
		return nil, platform.NewError(platform.CodeValidation, false, "接口规格缺失")
	}
	// 内嵌规格的 servers 是部署地址，入口只按路径匹配。
	spec.Servers = nil
	router, err := legacyrouter.NewRouter(spec, openapi3.DisableExamplesValidation(), openapi3.DisableSchemaDefaultsValidation())
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "接口规格无法建立校验路由")
	}
	options := &openapi3filter.Options{
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		MultiError:         true,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, params, err := router.FindRoute(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			input := &openapi3filter.RequestValidationInput{Request: r, PathParams: params, Route: route, Options: options}
			if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
				requestID := platform.Identifier(r.Header.Get(platform.HeaderRequestID))
				if !platform.ValidIdentifier(requestID) {
					requestID = platform.NewIdentifier()
				}
				p := problem.FromError(platform.NewError(platform.CodeValidation, false, "请求不符合接口规格"), requestID)
				p.Issues = fieldIssues(err)
				_ = problem.WriteProblem(w, p)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// fieldIssues 把校验错误转为字段位置与原因，不回显请求取值。
func fieldIssues(err error) []platform.FieldIssue {
	errs := []error{err}
	var multi openapi3.MultiError
	if errors.As(err, &multi) {
		errs = multi
	}
	issues := make([]platform.FieldIssue, 0, len(errs))
	for _, e := range errs {
		field, reason := "request", "invalid"
		var requestErr *openapi3filter.RequestError
		if errors.As(e, &requestErr) {
			switch {
			case requestErr.Parameter != nil:
				field = requestErr.Parameter.In + "." + requestErr.Parameter.Name
			case requestErr.RequestBody != nil:
				field = "body"
			}
		}
		var schemaErr *openapi3.SchemaError
		if errors.As(e, &schemaErr) {
			if pointer := schemaErr.JSONPointer(); len(pointer) > 0 {
				field += "/" + strings.Join(pointer, "/")
			}
			if schemaErr.SchemaField != "" {
				reason = schemaErr.SchemaField
			}
		}
		issues = append(issues, platform.FieldIssue{Field: field, Reason: reason})
		if len(issues) == maxIssues {
			break
		}
	}
	return issues
}
