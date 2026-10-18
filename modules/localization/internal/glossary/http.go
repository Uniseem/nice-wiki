package glossary

import (
	"context"
	"encoding/json"
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"strconv"
)

// NewGlossaryHandlers 只做HTTP协议转换，结构与权限均由公共入口建立。
func NewGlossaryHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"putGlossaryTerm": glossaryJSON(PutGlossaryTerm, http.StatusOK, func(i *PutGlossaryTermInput, r *http.Request) error {
			if i.Term.TermId != r.PathValue("termId") {
				return platform.NewError(platform.CodeValidation, false, "路径与术语正文不一致")
			}
			return nil
		}),
		"deleteGlossaryTerm": glossaryJSON(DeleteGlossaryTerm, http.StatusOK, func(i *DeleteGlossaryTermInput, r *http.Request) error {
			i.TermId = r.PathValue("termId")
			i.ExpectedVersion, _ = strconv.Atoi(r.URL.Query().Get("expectedVersion"))
			i.IdempotencyKey = r.Header.Get("idempotencyKey")
			return nil
		}),
		"listGlossary": glossaryJSON(ListGlossary, http.StatusOK, func(i *ListGlossaryInput, r *http.Request) error {
			i.SourceLanguage = r.URL.Query().Get("sourceLanguage")
			i.TargetLanguage = r.URL.Query().Get("targetLanguage")
			i.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
			if r.URL.Query().Has("cursor") {
				v := r.URL.Query().Get("cursor")
				i.Cursor = &v
			}
			return nil
		}),
		"applyApprovedGlossaryProposal": glossaryJSON(ApplyApprovedGlossaryProposal, http.StatusAccepted),
		"rollbackAutomaticGlossaryBatch": glossaryJSON(RollbackAutomaticGlossaryBatch, http.StatusOK, func(i *RollbackAutomaticGlossaryBatchInput, r *http.Request) error {
			if i.BatchId != r.PathValue("batchId") {
				return platform.NewError(platform.CodeValidation, false, "路径与术语批次不一致")
			}
			return nil
		}),
	}
}
func glossaryJSON[I, R any](fn func(context.Context, OperationContext, I) (R, error), status int, bind ...func(*I, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := platform.OperationContextFrom(r.Context())
		var input I
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "术语请求无法转换"), op.RequestID))
				return
			}
		}
		for _, bindInput := range bind {
			if err := bindInput(&input, r); err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
		}
		result, err := fn(r.Context(), op, input)
		if err != nil {
			_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(result)
	})
}
