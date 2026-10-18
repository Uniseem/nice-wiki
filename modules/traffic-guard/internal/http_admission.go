package trafficguard

import (
 "context"
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

type HandleAdmissionInput struct { RequestID string `json:"requestId"`;Action string `json:"action"`;VisitorToken string `json:"visitorToken"`;DataEpoch string `json:"-"` }
type HandleAdmissionResult struct { Permit platform.AdmissionPermit `json:"permit"` }
type admissionRateError struct { cause error;retryAfterMs int64 }
func (e *admissionRateError) Error()string { return e.cause.Error() }
func (e *admissionRateError) Unwrap()error { return e.cause }

// HandleAdmission 需挑战时只返回公共码，不创建挑战或把ID塞入Problem文本。
func HandleAdmission(ctx context.Context,op OperationContext,in HandleAdmissionInput)(HandleAdmissionResult,error) {
 if op.CallerService!="portal" { return HandleAdmissionResult{},platform.NewError(platform.CodeForbidden,false,"仅可信入口可创建匿名准入") }
 if in.RequestID!=op.RootOperationID { return HandleAdmissionResult{},platform.NewError(platform.CodeValidation,false,"准入必须使用原逻辑请求根") }
 runtime,ok:=ctx.Value(guardRuntimeKey{}).(GuardRuntime)
 if !ok { return HandleAdmissionResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"匿名风控运行配置未装配") }
 subject,err:=DeriveSubject(ctx,op,DeriveSubjectInput{VisitorToken:in.VisitorToken,SiteID:op.Actor.SiteID,KeyEpoch:runtime.BudgetKey.KeyID})
 if err!=nil { return HandleAdmissionResult{},err }
 evaluated,err:=EvaluateAdmission(ctx,op,EvaluateAdmissionInput{RequestID:in.RequestID,SubjectKey:subject.SubjectKey,Action:in.Action,DataEpoch:op.DataEpoch})
 if err!=nil { return HandleAdmissionResult{},err }
 switch evaluated.Decision {
 case "allow":return HandleAdmissionResult{Permit:*evaluated.Permit},nil
 case "challenge":return HandleAdmissionResult{},platform.NewError(platform.CodeChallengeRequired,false,"请先完成既有人机验证")
 case "deny","queue":
  if evaluated.ReasonCode==platform.CodeRateLimited { return HandleAdmissionResult{},&admissionRateError{cause:platform.NewError(platform.CodeRateLimited,true,"当前频率或并发已满"),retryAfterMs:evaluated.RetryAfterMs} }
  return HandleAdmissionResult{},platform.NewError(platform.CodeForbidden,false,"该网络AI功能暂被限制")
 default:return HandleAdmissionResult{},platform.NewError(platform.CodeIntegrityError,false,"准入决策不符合登记协议")
 }
}
