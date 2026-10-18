// Package transport 提供已完成用例的 HTTP 映射；完整应用仍须绑定全部 OpenAPI 操作。
package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/knowledge/internal/admin"
	"nice-wiki/modules/knowledge/internal/search"
)

// NewOperationHandlers 返回真实已完成处理函数，交给平台 NewServiceHandler 完整校验。
// bind 由服务装配注入存储、网关、基准配置及签名器；缺失的操作不伪造占位。
func NewOperationHandlers(bind func(context.Context)context.Context) map[string]http.Handler {
	return map[string]http.Handler{
		"searchKnowledge":jsonOperation(bind,search.Retrieve),
		"readSections":jsonOperation(bind,search.ReadSections),
		"createQueryEmbedding":jsonOperation(bind,search.CreateQueryEmbedding),
		"getBuild":http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
			ctx,op,err:=operationContext(r,bind);if err!=nil{writeResult(w,op,nil,err);return}
			result,err:=admin.GetBuild(ctx,op,admin.GetBuildInput{BuildID:r.PathValue("buildId")});writeResult(w,op,result,err)
		}),
		"listBuilds":http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
			ctx,op,err:=operationContext(r,bind);if err!=nil{writeResult(w,op,nil,err);return}
			limit,_:=strconv.Atoi(r.URL.Query().Get("limit"))
			result,err:=admin.ListBuilds(ctx,op,admin.ListBuildsInput{DocumentID:r.URL.Query().Get("documentId"),Cursor:r.URL.Query().Get("cursor"),Limit:limit});writeResult(w,op,result,err)
		}),
		"redactDiagnostics":http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
			ctx,op,err:=operationContext(r,bind);if err!=nil{writeResult(w,op,nil,err);return}
			counts,_:=strconv.ParseBool(r.URL.Query().Get("includeCounts"))
			result,err:=admin.RedactDiagnostics(ctx,op,admin.RedactDiagnosticsInput{BuildID:r.PathValue("buildId"),IncludeCounts:counts});writeResult(w,op,result,err)
		}),
	}
}

func jsonOperation[I,R any](bind func(context.Context)context.Context,run func(context.Context,platform.OperationContext,I)(R,error))http.Handler{
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		ctx,op,err:=operationContext(r,bind);if err!=nil{writeResult(w,op,nil,err);return}
		var input I
		if json.NewDecoder(r.Body).Decode(&input)!=nil{writeResult(w,op,nil,platform.NewError(platform.CodeValidation,false,"请求正文不可读"));return}
		result,err:=run(ctx,op,input);writeResult(w,op,result,err)
	})
}
func operationContext(r *http.Request,bind func(context.Context)context.Context)(context.Context,platform.OperationContext,error){
	op,ok:=platform.OperationContextFrom(r.Context())
	if !ok||bind==nil{return nil,op,platform.NewError(platform.CodeUnauthenticated,false,"请求缺少可信服务入口")}
	return bind(r.Context()),op,nil
}
func writeResult(w http.ResponseWriter,op platform.OperationContext,result any,err error){
	if err!=nil{_=problem.WriteProblem(w,problem.FromError(err,op.RequestID));return}
	raw,err:=json.Marshal(result);if err!=nil{_=problem.WriteProblem(w,problem.FromError(platform.NewError(platform.CodeInternal,false,"结果编码失败"),op.RequestID));return}
	w.Header().Set("Content-Type","application/json");w.WriteHeader(http.StatusOK);_,_=w.Write(raw)
}
