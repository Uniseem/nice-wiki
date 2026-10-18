package platform

import (
 "context"
 "encoding/json"
 "strings"
 "time"
)

// CleanupScope仅服务断言携带；不接受HTTP正文或未签请求头自报。它不包含或延长付费许可。
type CleanupScope struct {
 Version int `json:"version"`
 BarrierID Identifier `json:"barrierId"`
 DataEpoch Identifier `json:"dataEpoch"`
 Deadline Timestamp `json:"deadline"`
 SourceService string `json:"sourceService"`
 TargetService string `json:"targetService"`
 Action string `json:"action"`
 ResourceID string `json:"resourceId"`
 RootOperationID Identifier `json:"rootOperationId"`
 SiteID Identifier `json:"siteId"`
 CostClass string `json:"costClass,omitempty"`
 QueryHash Hash `json:"queryHash"`
}
// Resource规则仅由可信启动配置；只支持一个固定路径参数、JSON字符串叶或固定批次资源名。
type CleanupResource struct{PathParameter string;BodyPointer string;Literal string}
type CleanupRoute struct{TargetService string;Action string;Method string;PathTemplate string;Resource CleanupResource;Control bool;BrokerControl bool}
type CleanupOperationPolicy struct{Callers []string;Resource CleanupResource}
type CleanupRequest struct{SourceService string;TargetService string;Method string;Path string;RawQuery string;Body []byte;BodyHash Hash}
type CleanupScopeIssuer interface{IssueCleanup(context.Context,CleanupRequest)(*CleanupScope,error)}
type cleanupIssuerKey struct{}
func WithCleanupIssuer(ctx context.Context,issuer CleanupScopeIssuer)context.Context{return context.WithValue(ctx,cleanupIssuerKey{},issuer)}
func CleanupIssuerFrom(ctx context.Context)CleanupScopeIssuer{issuer,_:=ctx.Value(cleanupIssuerKey{}).(CleanupScopeIssuer);return issuer}
// WithoutCleanupIssuer防止运行权威只读客户端再次进入同一发行器递归。
func WithoutCleanupIssuer(ctx context.Context)context.Context{return WithCleanupIssuer(ctx,(*noCleanupIssuer)(nil))}
type noCleanupIssuer struct{}
func(*noCleanupIssuer)IssueCleanup(context.Context,CleanupRequest)(*CleanupScope,error){return nil,nil}

// MatchCleanupRoute只做公共固定路径与真实资源提取，业务原对象归属由目标权威回调证明。
func MatchCleanupRoute(route CleanupRoute,request CleanupRequest)(string,bool,error){
 if route.TargetService!=request.TargetService||route.Method!=request.Method{return "",false,nil};pattern:=strings.Split(strings.Trim(route.PathTemplate,"/"),"/");actual:=strings.Split(strings.Trim(request.Path,"/"),"/");if len(pattern)!=len(actual){return "",false,nil};params:=map[string]string{};for i,part:=range pattern{if strings.HasPrefix(part,"{")&&strings.HasSuffix(part,"}"){params[part[1:len(part)-1]]=actual[i]}else if part!=actual[i]{return "",false,nil}}
 if route.Control{return "",true,nil};resource,err:=CleanupResourceID(route.Resource,params,request.Body);return resource,true,err
}
func CleanupResourceID(rule CleanupResource,params map[string]string,body []byte)(string,error){
 count:=0;if rule.PathParameter!=""{count++};if rule.BodyPointer!=""{count++};if rule.Literal!=""{count++};if count!=1{return "",NewError(CodeIntegrityError,false,"清理资源提取策略必须唯一")};resource:=rule.Literal;if rule.PathParameter!=""{resource=params[rule.PathParameter]};if rule.BodyPointer!=""{if !strings.HasPrefix(rule.BodyPointer,"/"){return "",NewError(CodeIntegrityError,false,"清理JSON资源指针不合法")};var value any;if err:=json.Unmarshal(body,&value);err!=nil{return "",NewError(CodeValidation,false,"清理资源正文不是JSON")};for _,part:=range strings.Split(rule.BodyPointer[1:],"/"){object,ok:=value.(map[string]any);if !ok{return "",NewError(CodeValidation,false,"清理资源指针不是对象叶")};value=object[strings.ReplaceAll(strings.ReplaceAll(part,"~1","/"),"~0","~")]};resource,_=value.(string)};if resource==""||len(resource)>2048{return "",NewError(CodeValidation,false,"清理资源缺失或过长")};return resource,nil
}
// ValidateCleanupScope验证已签作用域自身的完整绑定；deadline无时钟偏差延长。
func ValidateCleanupScope(scope *CleanupScope,issuer,audience string,now time.Time)error{
 if scope==nil{return nil};until,err:=ParseTimestamp(scope.Deadline);if err!=nil||!until.After(now)||until.After(now.Add(30*time.Second))||scope.Version!=1||scope.SourceService!=issuer||scope.TargetService!=audience||!ValidIdentifier(scope.BarrierID)||!ValidIdentifier(scope.DataEpoch)||!ValidIdentifier(scope.RootOperationID)||!ValidIdentifier(scope.SiteID)||!ValidHash(scope.QueryHash)||scope.Action==""||scope.ResourceID==""||len(scope.ResourceID)>2048{return NewError(CodeUnauthenticated,false,"清理签名作用域不完整、过期或绑定错误")};switch scope.CostClass{case "",CostClassQA,CostClassBackground,CostClassAdminProbe:default:return NewError(CodeUnauthenticated,false,"清理费用类别不合法")};return nil
}
