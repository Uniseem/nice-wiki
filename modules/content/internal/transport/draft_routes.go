package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	content "nice-wiki/modules/content/internal"
	"nice-wiki/modules/content/internal/gen"
)

// RegisterDraftRoutes 注册建文档、编辑锁与共享草稿读写；授权由各核心函数按 content.edit/content.read 即时核对。
func RegisterDraftRoutes(app *Application, drafts content.DraftRuntime, locks content.EditLockRuntime, tree content.TreeRuntime, anchors platform.TrustAnchors) error {
	spec, err := gen.GetSwagger()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "草稿HTTP规格不可读")
	}
	keep := map[string]bool{
		"/v1/documents":                                 true,
		"/v1/documents/{documentId}/lock":               true,
		"/v1/documents/{documentId}/lock:force-release": true,
		"/v1/documents/{documentId}/draft":              true,
		"/v1/previews":                                  true,
		"/v1/documents/{documentId}/history:rollback":   true,
		"/v1/previews:validate":                         true,
	}
	for path := range spec.Paths.Map() {
		if !keep[path] {
			spec.Paths.Delete(path)
		}
	}
	// /v1/documents 的 GET 已由文档树路由注册。
	spec.Paths.Value("/v1/documents").Get = nil
	decode := func(w http.ResponseWriter, r *http.Request, into any) bool {
		if json.NewDecoder(r.Body).Decode(into) != nil {
			writeProblem(w, platform.NewError(platform.CodeValidation, false, "请求参数无法转换"))
			return false
		}
		return true
	}
	reply := func(w http.ResponseWriter, result any, err error) {
		w.Header().Set("Cache-Control", "no-store")
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
	draftCtx := func(r *http.Request) context.Context {
		return content.WithTreeRuntime(content.WithEditLockRuntime(content.WithDraftRuntime(r.Context(), drafts), locks), tree)
	}
	handlers := map[string]http.Handler{
		"createDocument": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input content.CreateDocumentInput
			if !decode(w, r, &input) {
				return
			}
			result, err := content.CreateDocument(draftCtx(r), op, input)
			reply(w, result, err)
		}),
		"acquireEditLock": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.AcquireEditLockRequest
			if !decode(w, r, &body) {
				return
			}
			result, err := content.AcquireEditLock(draftCtx(r), op, content.AcquireEditLockInput{DocumentId: r.PathValue("documentId"), TabId: body.TabId})
			reply(w, result, err)
		}),
		"renewEditLock": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.RenewEditLockRequest
			if !decode(w, r, &body) {
				return
			}
			result, err := content.RenewEditLock(draftCtx(r), op, content.RenewEditLockInput{DocumentId: r.PathValue("documentId"), LeaseId: body.LeaseId, FencingToken: body.FencingToken, ServerBootId: body.ServerBootId, LastInputAt: body.LastInputAt, HeartbeatSequence: body.HeartbeatSequence, InputSequence: body.InputSequence})
			reply(w, result, err)
		}),
		"releaseEditLock": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.ReleaseEditLockRequest
			if !decode(w, r, &body) {
				return
			}
			result, err := content.ReleaseEditLock(draftCtx(r), op, content.ReleaseEditLockInput{DocumentId: r.PathValue("documentId"), LeaseId: body.LeaseId, ServerBootId: body.ServerBootId})
			reply(w, result, err)
		}),
		"forceReleaseEditLock": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.ForceReleaseEditLockRequest
			if !decode(w, r, &body) {
				return
			}
			result, err := content.ForceReleaseEditLock(draftCtx(r), op, content.ForceReleaseEditLockInput{DocumentId: r.PathValue("documentId"), ExpectedLeaseId: body.ExpectedLeaseId})
			reply(w, result, err)
		}),
		"rollbackPublishedRevision": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.RollbackPublishedRevisionRequest
			if !decode(w, r, &body) {
				return
			}
			result, err := content.RollbackPublishedRevision(draftCtx(r), op, content.RollbackPublishedRevisionInput{DocumentId: r.PathValue("documentId"), HistoryId: body.HistoryId, ExpectedRevision: body.ExpectedRevision, AcknowledgeMissingAssets: body.AcknowledgeMissingAssets})
			reply(w, result, err)
		}),
		"createPreviewGrant": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input content.CreatePreviewGrantInput
			if !decode(w, r, &input) {
				return
			}
			result, err := content.CreatePreviewGrant(draftCtx(r), op, input)
			reply(w, result, err)
		}),
		"validatePreviewGrant": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input content.ValidatePreviewGrantInput
			if !decode(w, r, &input) {
				return
			}
			result, err := content.ValidatePreviewGrant(draftCtx(r), op, input)
			reply(w, result, err)
		}),
		"readDraft": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := content.ReadDraft(draftCtx(r), op, content.ReadDraftInput{DocumentId: r.PathValue("documentId")})
			reply(w, result, err)
		}),
		"saveDraft": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.SaveDraftRequest
			if !decode(w, r, &body) {
				return
			}
			result, err := content.SaveDraft(draftCtx(r), op, content.SaveDraftInput{DocumentId: r.PathValue("documentId"), Body: body.Body, ExpectedDraftVersion: body.ExpectedDraftVersion, ExpectedDraftRevisionId: body.ExpectedDraftRevisionId, LeaseId: body.LeaseId, FencingToken: body.FencingToken, ServerBootId: body.ServerBootId})
			reply(w, result, err)
		}),
	}
	// 各核心函数自带 content.edit/content.read/superadmin 即时授权，边界不重复远程核对。
	authorize := func(ctx context.Context, op platform.OperationContext, permission, action string) error { return nil }
	portal := []string{"portal"}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{Self: moduleName, Anchors: anchors, Runtime: app.runtime.ReadRuntimeState, OpenedDataEpoch: app.openedEpoch, AdmitBusiness: lifecycle.SimpleAdmission(app.runtime.ReadRuntimeState, app.openedEpoch), Authorize: authorize,
		AllowedCallers:        map[string][]string{"createDocument": portal, "acquireEditLock": portal, "renewEditLock": portal, "releaseEditLock": portal, "forceReleaseEditLock": portal, "readDraft": portal, "saveDraft": portal, "createPreviewGrant": {"public-site"}, "rollbackPublishedRevision": portal, "validatePreviewGrant": {"public-site"}},
		MaintenanceOperations: map[string]bool{"readDraft": true}}, handlers)
	if err != nil {
		return err
	}
	app.router.Handle("POST", "/v1/documents", handler)
	app.router.Handle("POST", "/v1/documents/{documentId}/lock", handler)
	app.router.Handle("PUT", "/v1/documents/{documentId}/lock", handler)
	app.router.Handle("DELETE", "/v1/documents/{documentId}/lock", handler)
	app.router.Handle("POST", "/v1/documents/{documentId}/lock:force-release", handler)
	app.router.Handle("POST", "/v1/documents/{documentId}/history:rollback", handler)
	app.router.Handle("POST", "/v1/previews", handler)
	app.router.Handle("POST", "/v1/previews:validate", handler)
	app.router.Handle("GET", "/v1/documents/{documentId}/draft", handler)
	app.router.Handle("PUT", "/v1/documents/{documentId}/draft", handler)
	return nil
}
