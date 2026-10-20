package apivalidate

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// PathRouter 按 OpenAPI 路径模板分发到处理器。标准库 ServeMux 不接受
// "{documentId}:publish" 这类段内后缀，所以这里用正则匹配；真正的路径参数
// 和方法校验仍由 NewServiceHandler 完成，这里只负责选对处理器。
type PathRouter struct {
	routes   []pathRoute
	fallback http.Handler
}

type pathRoute struct {
	literal int // 模板里字面量字符数，越多越具体，优先匹配
	method  string
	pattern *regexp.Regexp
	handler http.Handler
}

var tildeAction = regexp.MustCompile(`\}/~([A-Za-z0-9-]+)$`)

var templateParam = regexp.MustCompile(`\{[^/}]+\}`)

// NewPathRouter 创建路由器；没有匹配项时交给 fallback（可为 nil，此时返回 404）。
func NewPathRouter(fallback http.Handler) *PathRouter {
	return &PathRouter{fallback: fallback}
}

// Handle 登记方法与 OpenAPI 路径模板。
func (r *PathRouter) Handle(method, template string, handler http.Handler) {
	// NewServiceHandler 会把 "{id}:action" 改写成 "{id}/~action"，这里还原成真实请求路径形态。
	template = tildeAction.ReplaceAllString(template, "}:$1")
	var pattern strings.Builder
	pattern.WriteString("^")
	last := 0
	for _, loc := range templateParam.FindAllStringIndex(template, -1) {
		pattern.WriteString(regexp.QuoteMeta(template[last:loc[0]]))
		pattern.WriteString("[^/]+")
		last = loc[1]
	}
	pattern.WriteString(regexp.QuoteMeta(template[last:]))
	pattern.WriteString("$")
	literal := len(templateParam.ReplaceAllString(template, ""))
	r.routes = append(r.routes, pathRoute{literal, strings.ToUpper(method), regexp.MustCompile(pattern.String()), handler})
	sort.SliceStable(r.routes, func(i, j int) bool { return r.routes[i].literal > r.routes[j].literal })
}

// PatternMux 让 oapi-codegen 生成的 "METHOD /path/{id}:action" 注册走 PathRouter，
// 避开标准库 ServeMux 对段内后缀的限制。
type PatternMux struct{ *PathRouter }

func NewPatternMux() *PatternMux { return &PatternMux{NewPathRouter(nil)} }

func (m *PatternMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	method, path, _ := strings.Cut(pattern, " ")
	m.Handle(method, path, http.HandlerFunc(handler))
}

func (r *PathRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.EscapedPath()
	for _, route := range r.routes {
		if route.method == req.Method && route.pattern.MatchString(path) {
			route.handler.ServeHTTP(w, req)
			return
		}
	}
	if r.fallback != nil {
		r.fallback.ServeHTTP(w, req)
		return
	}
	http.NotFound(w, req)
}
