// Package problem 按RFC9457输出公共错误响应。
//
// detail去除本地路径、堆栈与密钥；响应不含请求正文、原始IP或内部地址。
package problem

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"nice-wiki/libs/platform"
)

// WriteProblem 输出application/problem+json。
// status按errors.json登记映射复核，未登记错误码按INTERNAL处理。
func WriteProblem(w http.ResponseWriter, problem platform.Problem) error {
	status, registered := platform.ProblemHTTPStatus[problem.Code]
	if !registered {
		status = platform.ProblemHTTPStatus[platform.CodeInternal]
		problem.Code = platform.CodeInternal
	}
	problem.Status = status
	problem.Detail = sanitizeDetail(problem.Detail)
	body, err := json.Marshal(problem)
	if err != nil {
		body = []byte(`{"type":"about:blank","title":"内部错误","status":500,"code":"INTERNAL","detail":"响应编码失败","requestId":"","retryable":false}`)
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(problem.Status)
	if _, err := w.Write(body); err != nil {
		return platform.NewError(platform.CodeInternal, false, "写出错误响应失败")
	}
	return nil
}

// FromError 把库错误映射为公共Problem；未知错误按INTERNAL。
func FromError(err error, requestID platform.Identifier) platform.Problem {
	p := platform.Problem{
		Type:      "about:blank",
		Title:     "请求未能完成",
		Status:    platform.ProblemHTTPStatus[platform.CodeInternal],
		Code:      platform.CodeInternal,
		Detail:    "服务内部错误，请稍后重试。",
		RequestID: requestID,
		Retryable: false,
	}
	if libErr, ok := platform.NormalizeIOError(err, platform.CodeInternal, false, p.Detail).(*platform.Error); ok {
		p.Code = libErr.Code
		p.Retryable = libErr.Retryable
		p.Detail = sanitizeDetail(libErr.Message)
		if status, ok := platform.ProblemHTTPStatus[libErr.Code]; ok {
			p.Status = status
		}
	}
	return p
}

var (
	stackFramePattern = regexp.MustCompile(`(?m)^\s*at\s.+$|(?m)^\s*\S+\.go:\d+.*$`)
	fsPathPattern     = regexp.MustCompile(`[A-Za-z]:\\[^\s"']+|/(?:Users|home|var|tmp|usr|opt|etc)/[^\s"']+`)
	keyPattern        = regexp.MustCompile(`(?i)(?:key|secret|token|password|credential)["':= ]+[^\s"',]+`)
)

// sanitizeDetail 去除堆栈帧、文件系统路径与疑似密钥赋值。
func sanitizeDetail(detail string) string {
	detail = stackFramePattern.ReplaceAllString(detail, "")
	detail = fsPathPattern.ReplaceAllString(detail, "[路径已省略]")
	detail = keyPattern.ReplaceAllString(detail, "[已省略]")
	detail = strings.Join(strings.Fields(detail), " ")
	if detail == "" {
		detail = "请求未能完成。"
	}
	return detail
}
