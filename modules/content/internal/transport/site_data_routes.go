package transport

import (
	"encoding/json"
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
	keep := map[string]bool{"/v1/site-import": true, "/v1/site-export": true, "/v1/backup-targets": true, "/v1/backup-targets/{targetId}": true, "/v1/backup-targets/{targetId}/drive-token": true, "/v1/backup-targets/{targetId}:run": true,
		"/v1/backup-runs": true, "/v1/site-git": true, "/v1/site-git:push": true, "/v1/site-git:pull": true}
	for p := range spec.Paths.Map() {
		if !keep[p] {
			spec.Paths.Delete(p)
		}
	}
	reply := func(w http.ResponseWriter, result any, err error) {
		w.Header().Set("Cache-Control", "no-store")
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
	decode := func(w http.ResponseWriter, r *http.Request, into any) bool {
		if json.NewDecoder(r.Body).Decode(into) != nil {
			writeProblem(w, platform.NewError(platform.CodeValidation, false, "请求参数无法转换"))
			return false
		}
		return true
	}
	ok := map[string]bool{"ok": true}
	handlers := map[string]http.Handler{
		"listBackupTargets": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			targets, err := data.ListBackupTargets(r.Context(), op)
			reply(w, map[string]any{"targets": targets}, err)
		}),
		"putBackupTarget": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var in content.BackupTargetInput
			if !decode(w, r, &in) {
				return
			}
			target, err := data.PutBackupTarget(r.Context(), op, r.PathValue("targetId"), in)
			reply(w, target, err)
		}),
		"deleteBackupTarget": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			reply(w, ok, data.DeleteBackupTarget(r.Context(), op, r.PathValue("targetId")))
		}),
		"storeDriveToken": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var in struct{ Provider, RefreshToken string }
			if !decode(w, r, &in) {
				return
			}
			reply(w, ok, data.StoreDriveToken(r.Context(), op, r.PathValue("targetId"), in.Provider, in.RefreshToken))
		}),
		"runBackup": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			run, err := data.RunBackup(r.Context(), op, r.PathValue("targetId"))
			reply(w, map[string]any{"run": run}, err)
		}),
		"listBackupRuns": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			runs, err := data.ListBackupRuns(r.Context(), op)
			reply(w, map[string]any{"runs": runs}, err)
		}),
		"readGitRemote": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			remote, err := data.ReadGitRemote(r.Context(), op)
			reply(w, map[string]any{"remote": remote}, err)
		}),
		"putGitRemote": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var in content.GitRemoteInput
			if !decode(w, r, &in) {
				return
			}
			remote, err := data.PutGitRemote(r.Context(), op, in)
			reply(w, map[string]any{"remote": remote}, err)
		}),
		"deleteGitRemote": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			reply(w, ok, data.DeleteGitRemote(r.Context(), op))
		}),
		"pushGit": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := data.PushGit(r.Context(), op)
			reply(w, result, err)
		}),
		"pullGit": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := data.PullGit(r.Context(), op)
			reply(w, result, err)
		}),
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
			bundle, err := data.BuildExport(r.Context(), op, false)
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
		AllowedCallers: map[string][]string{"importSite": portal, "exportSite": portal, "listBackupTargets": portal, "putBackupTarget": portal, "deleteBackupTarget": portal, "runBackup": portal, "listBackupRuns": portal, "storeDriveToken": {"identity"},
			"readGitRemote": portal, "putGitRemote": portal, "deleteGitRemote": portal, "pushGit": portal, "pullGit": portal}, OperationBodyLimits: map[string]int64{"importSite": 16 << 20}}, handlers)
	if err != nil {
		return err
	}
	app.router.Handle("POST", "/v1/site-import", handler)
	app.router.Handle("GET", "/v1/site-export", handler)
	app.router.Handle("GET", "/v1/backup-targets", handler)
	app.router.Handle("PUT", "/v1/backup-targets/{targetId}", handler)
	app.router.Handle("DELETE", "/v1/backup-targets/{targetId}", handler)
	app.router.Handle("PUT", "/v1/backup-targets/{targetId}/drive-token", handler)
	app.router.Handle("POST", "/v1/backup-targets/{targetId}:run", handler)
	app.router.Handle("GET", "/v1/backup-runs", handler)
	app.router.Handle("GET", "/v1/site-git", handler)
	app.router.Handle("PUT", "/v1/site-git", handler)
	app.router.Handle("DELETE", "/v1/site-git", handler)
	app.router.Handle("POST", "/v1/site-git:push", handler)
	app.router.Handle("POST", "/v1/site-git:pull", handler)
	return nil
}
