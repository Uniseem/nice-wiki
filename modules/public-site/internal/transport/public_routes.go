package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	publicsite "nice-wiki/modules/public-site/internal"
	"nice-wiki/modules/public-site/internal/gen"
)

// BindRoutes 把已实现的公开页面、导航、robots、sitemap 与 llms 入口挂到 mux；
// 尚未实现的操作（预览、屏障、快照、.md、分片 sitemap）从规格里剔除，请求落到 404。
func BindRoutes(app *Application, anchors platform.TrustAnchors, directory publicsite.LinkDirectory) error {
	if app == nil || app.runtime == nil || directory.SiteConfig == nil || directory.Content == nil || directory.Localization == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "公开站点权威依赖未装配")
	}
	text := func(contentType string, run func(ctx context.Context, op platform.OperationContext, r *http.Request) (int, string, error)) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			status, body, err := run(publicsite.WithLinkDirectory(r.Context(), directory), op, r)
			if err != nil {
				writeProblem(w, err)
				return
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		})
	}
	htmlType := "text/html; charset=utf-8"
	handlers := map[string]http.Handler{
		"health":               healthHandler(app),
		"ready":                readyHandler(app),
		"resolvePublishedLink": publishedLinkHandler(directory),
		"handleRootRedirect": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			in := publicsite.HandleRootRedirectInput{AcceptLanguage: r.Header.Get("Accept-Language"), DataEpoch: string(op.DataEpoch)}
			if cookie, err := r.Cookie("lang"); err == nil {
				in.LanguageCookie = cookie.Value
			}
			result, err := publicsite.HandleRootRedirect(publicsite.WithLinkDirectory(r.Context(), directory), op, in)
			if err != nil {
				writeProblem(w, err)
				return
			}
			for _, header := range result.Headers {
				w.Header().Set(header.Name, header.Value)
			}
			http.Redirect(w, r, result.Location, result.Status)
		}),
		"handleHome": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := publicsite.HandleHome(publicsite.WithLinkDirectory(r.Context(), directory), op, publicsite.HandleHomeInput{Locale: r.PathValue("locale"), DataEpoch: string(op.DataEpoch)})
			if err != nil {
				writePageProblem(w, r, err)
				return
			}
			if result.Location != "" {
				w.Header().Set("Cache-Control", "no-cache")
				http.Redirect(w, r, result.Location, result.Status)
				return
			}
			w.Header().Set("Content-Type", htmlType)
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write([]byte(result.HTML))
		}),
		"handlePage": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			// 分片 sitemap 的路径 /sitemap/<语言>/<序号>.xml 带段内后缀，OpenAPI 路径模板挂不上，在这里按页面通配路由认出来。
			if r.PathValue("locale") == "sitemap" {
				if language, file, ok := strings.Cut(r.PathValue("slug"), "/"); ok && strings.HasSuffix(file, ".xml") {
					if shard, convErr := strconv.Atoi(strings.TrimSuffix(file, ".xml")); convErr == nil {
						result, sitemapErr := publicsite.HandleSitemapShard(publicsite.WithLinkDirectory(r.Context(), directory), op, publicsite.HandleSitemapShardInput{Locale: language, Shard: shard, DataEpoch: string(op.DataEpoch)})
						if sitemapErr != nil {
							writeProblem(w, sitemapErr)
							return
						}
						w.Header().Set("Content-Type", "application/xml")
						w.WriteHeader(result.Status)
						_, _ = w.Write([]byte(result.Xml))
						return
					}
				}
			}
			result, err := publicsite.HandlePage(publicsite.WithLinkDirectory(r.Context(), directory), op, publicsite.HandlePageInput{Locale: r.PathValue("locale"), Slug: r.PathValue("slug"), DataEpoch: string(op.DataEpoch)})
			if err != nil {
				writePageProblem(w, r, err)
				return
			}
			w.Header().Set("Content-Type", htmlType)
			if result.ContentType != "" {
				w.Header().Set("Content-Type", result.ContentType)
			}
			w.Header().Set("Cache-Control", "no-cache")
			if result.Status == http.StatusMovedPermanently {
				http.Redirect(w, r, result.Location, result.Status)
				return
			}
			if result.Status == http.StatusGone {
				w.WriteHeader(result.Status)
				_, _ = w.Write([]byte(result.HTML))
				return
			}
			w.Header().Set("ETag", result.ETag)
			if r.Header.Get("If-None-Match") == result.ETag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			_, _ = w.Write([]byte(result.HTML))
		}),
		"issuePreviewToken": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input publicsite.IssuePreviewTokenInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "预览请求无法转换"))
				return
			}
			result, err := publicsite.IssuePreviewToken(publicsite.WithLinkDirectory(r.Context(), directory), op, input)
			if err != nil {
				writeProblem(w, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"handlePreview": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := publicsite.HandlePreview(publicsite.WithLinkDirectory(r.Context(), directory), op, publicsite.HandlePreviewInput{PreviewID: r.URL.Query().Get("previewId"), Token: r.PathValue("token")})
			if err != nil {
				writeProblem(w, err)
				return
			}
			for _, header := range result.Headers {
				w.Header().Set(header.Name, header.Value)
			}
			w.Header().Set("Content-Type", htmlType)
			w.WriteHeader(result.Status)
			_, _ = w.Write([]byte(result.HTML))
		}),
		"handleNavigation": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := publicsite.HandleNavigation(publicsite.WithLinkDirectory(r.Context(), directory), op, publicsite.HandleNavigationInput{Locale: r.PathValue("locale"), DataEpoch: string(op.DataEpoch)})
			if err != nil {
				writeProblem(w, err)
				return
			}
			w.Header().Set("ETag", result.ETag)
			w.Header().Set("Cache-Control", "no-cache")
			writeJSON(w, http.StatusOK, result.Tree)
		}),
		"handleRobots": text("text/plain; charset=utf-8", func(ctx context.Context, op platform.OperationContext, r *http.Request) (int, string, error) {
			result, err := publicsite.HandleRobots(ctx, op, publicsite.HandleRobotsInput{DataEpoch: string(op.DataEpoch)})
			return result.Status, result.Text, err
		}),
		"handleLLMsRoot": text("text/plain; charset=utf-8", func(ctx context.Context, op platform.OperationContext, r *http.Request) (int, string, error) {
			result, err := publicsite.HandleLLMsRoot(ctx, op, publicsite.HandleLLMsRootInput{DataEpoch: string(op.DataEpoch)})
			return result.Status, result.Text, err
		}),
		"handleLLMsIndex": text("text/plain; charset=utf-8", func(ctx context.Context, op platform.OperationContext, r *http.Request) (int, string, error) {
			result, err := publicsite.HandleLLMsIndex(ctx, op, publicsite.HandleLLMsIndexInput{Locale: r.PathValue("locale"), DataEpoch: string(op.DataEpoch)})
			return result.Status, result.Text, err
		}),
		"handleSitemapIndex": text("application/xml", func(ctx context.Context, op platform.OperationContext, r *http.Request) (int, string, error) {
			result, err := publicsite.HandleSitemapIndex(ctx, op, publicsite.HandleSitemapIndexInput{DataEpoch: string(op.DataEpoch)})
			return result.Status, result.Xml, err
		}),
		"handleSitemapShard": text("application/xml", func(ctx context.Context, op platform.OperationContext, r *http.Request) (int, string, error) {
			shard, _ := strconv.Atoi(r.PathValue("shard"))
			result, err := publicsite.HandleSitemapShard(ctx, op, publicsite.HandleSitemapShardInput{Locale: r.PathValue("locale"), Shard: shard, DataEpoch: string(op.DataEpoch)})
			return result.Status, result.Xml, err
		}),
		"handleLanguageSitemap": text("application/xml", func(ctx context.Context, op platform.OperationContext, r *http.Request) (int, string, error) {
			result, err := publicsite.HandleLanguageSitemap(ctx, op, publicsite.HandleLanguageSitemapInput{Locale: r.PathValue("locale"), DataEpoch: string(op.DataEpoch)})
			return result.Status, result.Xml, err
		}),
	}
	spec, err := gen.GetSwagger()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "公开站点接口规格不可读")
	}
	// 嵌套文档的公开路径含多段，{slug*} 让路由匹配剩余整段路径。
	if item := spec.Paths.Value("/{locale}/{slug}"); item != nil {
		spec.Paths.Delete("/{locale}/{slug}")
		spec.Paths.Set("/{locale}/{slug*}", item)
	}
	type route struct{ method, path string }
	var mounted []route
	for path, item := range spec.Paths.Map() {
		if path == "/sitemap/{locale}/{shard}.xml" || path == "/{locale}/{slug}.md" {
			spec.Paths.Delete(path)
			continue
		}
		for method, operation := range item.Operations() {
			if handlers[strings.ToLower(operation.OperationID[:1])+operation.OperationID[1:]] == nil {
				item.SetOperation(method, nil)
				continue
			}
			mounted = append(mounted, route{method, path})
		}
		if len(item.Operations()) == 0 {
			spec.Paths.Delete(path)
		}
	}
	// 访客页面都经 portal 代理进来；只有已发布链接解析给 site-config 与 qa 用。
	callers := map[string][]string{}
	for id := range handlers {
		callers[id] = []string{"portal"}
	}
	callers["resolvePublishedLink"] = []string{"site-config", "qa"}
	callers["issuePreviewToken"] = []string{"portal"}
	callers["handlePreview"] = []string{"portal"}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self:                moduleName,
		Anchors:             anchors,
		Runtime:             app.runtime.ReadRuntimeState,
		OpenedDataEpoch:     app.openedEpoch,
		Authorize:           apivalidate.NewIdentityAuthorizer(directory.Identity),
		AdmitBusiness:       lifecycle.SimpleAdmission(app.runtime.ReadRuntimeState, app.openedEpoch),
		AllowedCallers:      callers,
		ServiceActorCallers: map[string][]string{"resolvePublishedLink": {"site-config", "qa"}},
	}, handlers)
	if err != nil {
		return err
	}
	router := apivalidate.NewPathRouter(app.httpServer.Handler)
	for _, r := range mounted {
		router.Handle(r.method, r.path, handler)
	}
	app.httpServer.Handler = router
	return nil
}
