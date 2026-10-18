package trafficguard

import (
 "context"
 "encoding/json"
 "net/http"
 "strconv"
 "nice-wiki/libs/platform"
)

// NewGuardHandlers 只交给公共NewServiceHandler；权限、服务身份、epoch和schema由入口建立。
// 管理动作的私有marker表示本次公共边界已即时查权，领域函数不再重复查询identity。
func NewGuardHandlers(runtime GuardRuntime)map[string]http.Handler {
 handlers:=NewLoginHandlers(runtime.Store)
 for _,operation:=range []string{"getPolicy","updatePolicy","createBan","revokeBan","listBans","listRiskEvents","reportModelRisk","queueSessionAdmission"} {
  handlers[operation]=http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
   op,_:=platform.OperationContextFrom(r.Context())
   ctx:=WithGuardRuntime(r.Context(),runtime)
   switch operation { case "getPolicy","updatePolicy","createBan","revokeBan","listBans","listRiskEvents":ctx=context.WithValue(ctx,guardAuthorizationKey{},guardAuthorizationMark{op.OperationID,operation}) }
   bind:=func(value any)error { if json.NewDecoder(r.Body).Decode(value)!=nil { return platform.NewError(platform.CodeValidation,false,"风控请求绑定失败") };return nil }
   var result any
   var err error
   switch operation {
   case "getPolicy":result,err=GetPolicy(ctx,op,GetPolicyInput{op.Actor.SiteID,op.DataEpoch})
   case "updatePolicy":var input UpdatePolicyInput;if err=bind(&input);err==nil { input.DataEpoch=op.DataEpoch;result,err=UpdatePolicy(ctx,op,input) }
   case "createBan":var input CreateBanInput;if err=bind(&input);err==nil { input.DataEpoch=op.DataEpoch;result,err=CreateBan(ctx,op,input) }
   case "revokeBan":var input struct{ExpectedRevision int64 `json:"expectedRevision"`};if err=bind(&input);err==nil { result,err=RevokeBan(ctx,op,RevokeBanInput{r.PathValue("banId"),input.ExpectedRevision,op.DataEpoch}) }
   case "listBans":limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));result,err=ListBans(ctx,op,ListBansInput{r.URL.Query().Get("cursor"),limit,op.DataEpoch})
   case "listRiskEvents":limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));result,err=ListRiskEvents(ctx,op,ListRiskEventsInput{Page:GuardPageRequest{Limit:limit,Cursor:r.URL.Query().Get("cursor")},Since:r.URL.Query().Get("since")})
   case "reportModelRisk":var input ReportQuestionSignalsInput;if err=bind(&input);err==nil { input.DataEpoch=op.DataEpoch;result,err=ReportQuestionSignals(ctx,op,input) }
   case "queueSessionAdmission":var input QueueSessionAdmissionInput;if err=bind(&input);err==nil { input.DataEpoch=op.DataEpoch;result,err=QueueSessionAdmission(ctx,op,input) }
   }
   writeLoginResponse(w,op.RequestID,result,err)
  })
 }
 return handlers
}
