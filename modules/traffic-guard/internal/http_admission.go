package trafficguard

import (
	"encoding/json"
	"net/http"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
)

// NewLoginHandlers 只供统一NewServiceHandler装配；服务身份、当前runtime、
// 维护、schema和路径值由该入口建立。其余业务缺失时不能单独启动本组。
func NewLoginHandlers(store *PolicyStore) map[string]http.Handler {
	return map[string]http.Handler{
		"beginLoginAttempt": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input BeginLoginAttemptInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeLoginResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "登录预留请求绑定失败"))
				return
			}
			result, err := BeginLoginAttempt(WithPolicyStore(r.Context(), store), op, input)
			writeLoginResponse(w, op.RequestID, result, err)
		}),
		"finishLoginAttempt": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input FinishLoginAttemptInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeLoginResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "登录终局请求绑定失败"))
				return
			}
			result, err := FinishLoginAttempt(WithPolicyStore(r.Context(), store), op, r.PathValue("attemptId"), input)
			writeLoginResponse(w, op.RequestID, result, err)
		}),
	}
}

func writeLoginResponse(w http.ResponseWriter, requestID Identifier, result any, err error) {
	if err != nil {
		_ = problem.WriteProblem(w, problem.FromError(err, requestID))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}
