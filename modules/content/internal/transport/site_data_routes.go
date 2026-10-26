package transport

import (
	"io"
	"net/http"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	content "nice-wiki/modules/content/internal"
	"nice-wiki/modules/content/internal/gen"
)

// RegisterSiteDataRoutes 注册站点数据接口：导入、导出；备份目标和 Git 同步在同一处登记。
// 权限由边界按合同里的 x-permission 向身份服务核对（核心函数不重复核对）。
func RegisterSiteDataRoutes(app *Application, data content.SiteData, identity *gen.IdentityClient, anchors platform.TrustAnchors) error {
	spec, err := gen.GetSwagger()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "站点数据HTTP规格不可读")
	}
	keep := map[string]bool{"/v1/site-import": true, "/v1/site-export": true}
	for p := range spec.Paths.Map() {
		if !keep[p] {
			spec.Paths.Delete(p)
		}
	}
	handlers := map[string]http.Handler{
		"importSite": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			if r.ContentLength < 1 || r.ContentLength > 16<<20 {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "压缩包必须有 1 字节至 16MiB 的明确长度"))
				return
			}
			raw, err := io.ReadAll(io.LimitReader(r.Body, r.ContentLength))
			if err != nil || int64(len(raw)) != r.ContentLength {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "压缩包读取不完整"))
				return
			}
			result, err := data.ImportSite(r.Context(), op, raw, content.ImportOptions{Format: r.URL.Query().Get("format"), ParentID: r.URL.Query().Get("parent")})
			w.Header().Set("Cache-Control", "no-store")
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"exportSite": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			bundle, err := data.BuildExport(r.Context(), op)
			if err != nil {
				writeProblem(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Cache-Control", "no-store")
			_ = bundle.WriteZip(w)
		}),
	}
	portal := []string{"portal"}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{Self: moduleName, Anchors: anchors, Runtime: app.runtime.ReadRuntimeState, OpenedDataEpoch: app.openedEpoch,
		AdmitBusiness: lifecycle.SimpleAdmission(app.runtime.ReadRuntimeState, app.openedEpoch), Authorize: apivalidate.NewIdentityAuthorizer(identity),
		AllowedCallers: map[string][]string{"importSite": portal, "exportSite": portal}, OperationBodyLimits: map[string]int64{"importSite": 16 << 20}}, handlers)
	if err != nil {
		return err
	}
	app.router.Handle("POST", "/v1/site-import", handler)
	app.router.Handle("GET", "/v1/site-export", handler)
	return nil
}
