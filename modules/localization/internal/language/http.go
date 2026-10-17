package language

import (
	"encoding/json"
	"net/http"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
)

// NewLanguageHandlers 只绑定已实现的三个语言配置操作。
// 服务身份、维护阶段和请求结构由公共入口校验一次。
func NewLanguageHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"putLanguage": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, ok := operationFrom(w, r)
			if !ok {
				return
			}
			var input PutLanguageInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeLanguageProblem(w, op.RequestID, platform.NewError(platform.CodeValidation, false, "语言配置输入不可读"))
				return
			}
			if r.PathValue("tag") != input.Language.Tag {
				writeLanguageProblem(w, op.RequestID, platform.NewError(platform.CodeValidation, false, "语言路径与正文不一致"))
				return
			}
			if key := r.Header.Get("Idempotency-Key"); key != "" && key != input.IdempotencyKey {
				writeLanguageProblem(w, op.RequestID, platform.NewError(platform.CodeValidation, false, "幂等头与正文不一致"))
				return
			}
			result, err := PutLanguage(r.Context(), op, input)
			if err != nil {
				writeLanguageProblem(w, op.RequestID, err)
				return
			}
			writeLanguageJSON(w, result)
		}),
		"setLanguageEnabled": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, ok := operationFrom(w, r)
			if !ok {
				return
			}
			var input SetLanguageEnabledInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeLanguageProblem(w, op.RequestID, platform.NewError(platform.CodeValidation, false, "语言启停输入不可读"))
				return
			}
			if r.PathValue("tag") != input.Tag {
				writeLanguageProblem(w, op.RequestID, platform.NewError(platform.CodeValidation, false, "语言路径与正文不一致"))
				return
			}
			if key := r.Header.Get("Idempotency-Key"); key != "" && key != input.IdempotencyKey {
				writeLanguageProblem(w, op.RequestID, platform.NewError(platform.CodeValidation, false, "幂等头与正文不一致"))
				return
			}
			result, err := SetLanguageEnabled(r.Context(), op, input)
			if err != nil {
				writeLanguageProblem(w, op.RequestID, err)
				return
			}
			writeLanguageJSON(w, result)
		}),
		"listLanguages": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, ok := operationFrom(w, r)
			if !ok {
				return
			}
			includeDisabled := false
			if raw := r.URL.Query().Get("includeDisabled"); raw == "true" {
				includeDisabled = true
			}
			result, err := ListLanguages(r.Context(), op, ListLanguagesInput{IncludeDisabled: includeDisabled})
			if err != nil {
				writeLanguageProblem(w, op.RequestID, err)
				return
			}
			writeLanguageJSON(w, result)
		}),
	}
}

func operationFrom(w http.ResponseWriter, r *http.Request) (OperationContext, bool) {
	w.Header().Set("Cache-Control", "no-store")
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeLanguageProblem(w, "", platform.NewError(platform.CodeUnauthenticated, false, "缺少可信服务操作上下文"))
		return OperationContext{}, false
	}
	return op, true
}

func writeLanguageJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

func writeLanguageProblem(w http.ResponseWriter, requestID platform.Identifier, err error) {
	if requestID == "" {
		requestID = platform.NewIdentifier()
	}
	_ = problem.WriteProblem(w, problem.FromError(err, requestID))
}
