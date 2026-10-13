package api

import (
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"net/http"
	"nice-wiki/internal/auth"
)

// 每个路由必须显式声明策略，未知路由默认拒绝。
var routePolicies = map[string]string{
	"POST /api/public/session":                              "anonymous",
	"GET /api/public/pow":                                   "anonymous",
	"POST /api/public/ask":                                  "anonymous",
	"GET /api/public/conversation/{id}":                     "anonymous",
	"DELETE /api/public/conversation/{id}":                  "anonymous",
	"POST /api/public/feedback":                             "anonymous",
	"POST /api/public/locale":                               "anonymous",
	"POST /api/public/beacon":                               "anonymous",
	"GET /api/admin/jobs":                                   "superadmin",
	"POST /api/admin/jobs/reindex":                          "aiSettings",
	"GET /api/admin/stats":                                  "viewStats",
	"GET /api/admin/backups":                                "superadmin",
	"POST /api/admin/backups":                               "superadmin",
	"GET /api/admin/backups/{name}":                         "superadmin",
	"GET /api/admin/memory":                                 "aiMemory",
	"PUT /api/admin/memory":                                 "aiMemory",
	"GET /api/admin/memory/tasks":                           "aiMemory",
	"PUT /api/admin/memory/tasks":                           "aiMemory",
	"POST /api/admin/memory/assistant":                      "aiMemory",
	"POST /api/admin/memory/assistant/{conversation}/apply": "aiMemory",
	"GET /api/admin/models/catalog":                         "superadmin",
	"PUT /api/admin/models/catalog":                         "superadmin",
	"GET /api/admin/qa/records":                             "manageQA",
	"PUT /api/admin/qa/records/{id}/featured":               "manageQA",
	"DELETE /api/admin/qa/records/{id}/featured":            "manageQA",
	"GET /api/admin/languages":                              "manageLanguages",
	"PUT /api/admin/languages":                              "manageLanguages",
	"GET /api/admin/translations/status":                    "manageTranslations",
	"GET /api/admin/translations/{language}/{id}":           "manageTranslations",
	"PUT /api/admin/translations/{language}/{id}":           "manageTranslations",
	"GET /api/admin/glossary":                               "manageTranslations",
	"PUT /api/admin/glossary":                               "manageTranslations",
	"POST /api/admin/translations/estimate":                 "manageLanguages",
	"GET /api/admin/guard/events":                           "aiSettings",
	"DELETE /api/admin/guard/bans/{key}":                    "aiSettings",
	"DELETE /api/admin/qa/cache":                            "manageQA",
	"GET /api/admin/qa/debug/{id}":                          "aiSettings",
	"GET /api/admin/trash":                                  "publishDocs",
	"POST /api/admin/trash/{id}/restore":                    "publishDocs",
	"DELETE /api/admin/trash/{id}":                          "superadmin",
	"GET /api/admin/docs/{id}/history":                      "publishDocs",
	"GET /api/admin/docs/{id}/history/diff":                 "publishDocs",
	"POST /api/admin/docs/{id}/history/restore":             "publishDocs",
	"GET /api/admin/docs/{id}/preview":                      "editDocs",
	"POST /api/admin/assets/upload":                         "editDocs",
	"POST /api/admin/video/resolve":                         "editDocs",
	"GET /api/admin/assets":                                 "manageAssets",
	"DELETE /api/admin/assets/{id}":                         "manageAssets",
	"DELETE /api/admin/accounts/{id}":                       "superadmin",
	"DELETE /api/admin/docs/{id}":                           "publishDocs",
	"DELETE /api/admin/docs/{id}/lock":                      "editDocs",
	"DELETE /api/admin/docs/{id}/lock/force":                "superadmin",
	"GET /api/admin/accounts":                               "superadmin",
	"GET /api/admin/audit":                                  "superadmin",
	"GET /api/admin/auth/me":                                "passwordChange",
	"GET /api/admin/docs":                                   "editDocs",
	"GET /api/admin/docs/{id}/draft":                        "editDocs",
	"GET /api/admin/models/channels":                        "superadmin",
	"PUT /api/admin/models/channels/{id}":                   "superadmin",
	"DELETE /api/admin/models/channels/{id}":                "superadmin",
	"POST /api/admin/models/channels/{id}/probe":            "superadmin",
	"GET /api/admin/settings/banner":                        "siteSettings",
	"GET /api/admin/settings/ai":                            "aiSettings",
	"GET /api/admin/settings/home":                          "siteSettings",
	"GET /api/admin/settings/site":                          "siteSettings",
	"GET /api/admin/setup":                                  "anonymous",
	"GET /api/admin/system":                                 "superadmin",
	"GET /healthz":                                          "anonymous",
	"POST /api/admin/accounts":                              "superadmin",
	"POST /api/admin/auth/login":                            "anonymous",
	"POST /api/admin/auth/logout":                           "passwordChange",
	"POST /api/admin/auth/logout-all":                       "authenticated",
	"POST /api/admin/docs":                                  "editDocs",
	"POST /api/admin/docs/{id}/lock":                        "editDocs",
	"POST /api/admin/docs/{id}/publish":                     "publishDocs",
	"POST /api/admin/docs/{id}/withdraw":                    "publishDocs",
	"POST /api/admin/models/channels":                       "superadmin",
	"POST /api/admin/setup/complete":                        "superadmin",
	"POST /api/admin/setup/register":                        "anonymous",
	"POST /api/admin/setup/restore":                         "anonymous",
	"PUT /api/admin/accounts/{id}":                          "superadmin",
	"PUT /api/admin/accounts/{id}/password":                 "superadmin",
	"PUT /api/admin/auth/password":                          "passwordChange",
	"PUT /api/admin/docs/{id}/draft":                        "editDocs",
	"PUT /api/admin/docs/{id}/lock":                         "editDocs",
	"PUT /api/admin/docs/{id}/move":                         "editDocs",
	"PUT /api/admin/settings/banner":                        "siteSettings",
	"PUT /api/admin/settings/ai":                            "aiSettings",
	"PUT /api/admin/settings/home":                          "siteSettings",
	"PUT /api/admin/settings/site":                          "siteSettings",
	"PUT /api/admin/setup/language":                         "superadmin",
	"PUT /api/admin/system":                                 "superadmin",
}

func (s *Server) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		policy, exists := routePolicies[r.Pattern]
		if !exists {
			writeError(w, http.StatusForbidden, "undeclared_policy", "接口未声明权限")
			return
		}
		if err := auth.ValidateOrigin(r, s.Settings.Site().BaseURL, s.Dev); err != nil {
			writeError(w, http.StatusForbidden, "invalid_origin", "请求来源无效")
			return
		}
		if policy == "anonymous" {
			next.ServeHTTP(w, r)
			return
		}
		principal, err := s.Auth.Authenticate(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
			return
		}
		if principal.Account.MustChangePassword && policy != "passwordChange" {
			writeError(w, http.StatusForbidden, "password_change_required", "请先修改初始密码")
			return
		}
		if policy == "superadmin" && !principal.Account.Superadmin {
			writeError(w, http.StatusForbidden, "forbidden", "需要超级管理员权限")
			return
		}
		if policy != "superadmin" && policy != "authenticated" && policy != "passwordChange" && !principal.Account.Has(auth.Permission(policy)) {
			writeError(w, http.StatusForbidden, "forbidden", "没有此功能的权限")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
	})
}

// 启动时逐一核对契约和后端策略，缺失、未知或不一致时拒绝启动。
func (s *Server) checkPolicies(contract *openapi3.T) error {
	declared := map[string]bool{}
	for path, item := range contract.Paths.Map() {
		for method, operation := range item.Operations() {
			pattern := method + " " + path
			policy, ok := operation.Extensions["x-permission"].(string)
			if !ok || policy == "" {
				return fmt.Errorf("接口缺少权限声明: %s", pattern)
			}
			known := policy == "anonymous" || policy == "authenticated" || policy == "superadmin" || policy == "passwordChange"
			for _, permission := range auth.AllPermissions {
				if string(permission) == policy {
					known = true
				}
			}
			if !known {
				return fmt.Errorf("接口声明了未知权限: %s", pattern)
			}
			if backend, ok := routePolicies[pattern]; !ok || backend != policy {
				return fmt.Errorf("接口权限与契约不一致: %s", pattern)
			}
			declared[pattern] = true
		}
	}
	for pattern := range routePolicies {
		if !declared[pattern] {
			return fmt.Errorf("后端权限声明没有对应接口: %s", pattern)
		}
	}
	return nil
}
