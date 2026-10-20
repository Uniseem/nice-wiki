package apivalidate

import (
	"net/http"
	"regexp"
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
	method  string
	pattern *regexp.Regexp
	handler http.Handler
}

var templateParam = regexp.MustCompile(`\{[^/}]+\}`)

// NewPathRouter 创建路由器；没有匹配项时交给 fallback（可为 nil，此时返回 404）。
func NewPathRouter(fallback http.Handler) *PathRouter {
	return &PathRouter{fallback: fallback}
}

// Handle 登记方法与 OpenAPI 路径模板。
func (r *PathRouter) Handle(method, template string, handler http.Handler) {
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
	r.routes = append(r.routes, pathRoute{strings.ToUpper(method), regexp.MustCompile(pattern.String()), handler})
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
