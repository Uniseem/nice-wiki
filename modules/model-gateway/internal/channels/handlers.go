package channels

import (
	"context"
	"encoding/json"
	"net/http"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/gen"
)

func handle[B any, I any, O any](execute func(context.Context, OperationContext, I) (O, error), build func(B, *http.Request) I) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		op, _ := platform.OperationContextFrom(r.Context())
		var body B
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "输入不可解码"), op.RequestID))
				return
			}
		}
		result, err := execute(r.Context(), op, build(body, r))
		if err != nil {
			_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_ = json.NewEncoder(w).Encode(result)
	})
}

type none = struct{}

// Handlers 是用户自带渠道与额度用量的 HTTP 处理器（由网关装配时并入）。
func Handlers() map[string]http.Handler {
	return map[string]http.Handler{
		"listAiChannels":  handle(ListChannels, func(_ none, _ *http.Request) none { return none{} }),
		"createAiChannel": handle(CreateChannel, func(b gen.CreateAiChannelInput, _ *http.Request) gen.CreateAiChannelInput { return b }),
		"deleteAiChannel": handle(DeleteChannel, func(_ none, r *http.Request) ChannelRef { return ChannelRef{ChannelID: r.PathValue("channelId")} }),
		"readAiUsage":     handle(Usage, func(_ none, _ *http.Request) none { return none{} }),
	}
}
