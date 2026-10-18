package language

import (
	"context"
	"encoding/json"
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
)

// JSONHandler 仅转换已通过平台边界的请求，路径/正文关联由各处理器装配。
func JSONHandler[I, R any](execute func(context.Context, OperationContext, I) (R, error), bind ...func(*I, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := platform.OperationContextFrom(r.Context())
		var input I
		if r.Method != http.MethodGet {
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "本地化请求无法转换"), op.RequestID))
				return
			}
		}
		for _, convert := range bind {
			if err := convert(&input, r); err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
		}
		result, err := execute(r.Context(), op, input)
		if err != nil {
			_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	})
}
