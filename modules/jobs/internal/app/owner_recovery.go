// Package app 只装配版本化生产方，不读取owner业务数据。
package app
import (
 "context"
 "encoding/json"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/apivalidate"
 jobs "nice-wiki/modules/jobs/internal"
 "nice-wiki/modules/jobs/internal/gen"
)
func ReadQueueSiteID(ctx context.Context,client *gen.SiteConfigClient,epoch string,validate func(json.RawMessage)error)(string,error){
 work,cancel:=context.WithTimeout(ctx,5*time.Second);defer cancel();deadline,_:=work.Deadline();id:=platform.NewIdentifier()
 op:=platform.OperationContext{RequestID:id,OperationID:id,RootOperationID:id,CallerService:ModuleSlug,DataEpoch:epoch,Deadline:deadline.UTC().Format(time.RFC3339Nano),Actor:platform.AuthContext{Kind:"service",SubjectID:ModuleSlug}}
 raw,err:=client.ReadSiteConfig(platform.WithOperationContext(work,op));if err!=nil{return "",err}
 if err=validate(raw);err!=nil{return "",platform.NewError(platform.CodeIntegrityError,false,"当前站点配置响应不符合合同")}
 var result struct{Config struct{SiteID string `json:"siteId"`} `json:"config"`}
 if json.Unmarshal(raw,&result)!=nil||result.Config.SiteID==""{return "",platform.NewError(platform.CodeDependencyUnavailable,true,"当前站点身份尚未建立")};return result.Config.SiteID,nil
}
func NewOwnerRecoveryChecker(config platform.ServiceConfig,key platform.SigningKey,siteID string)(jobs.OwnerRecoveryChecker,error){
 validate,err:=apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec),"KnowledgeOwnerRecoveryResult");if err!=nil{return nil,err}
 var knowledge *gen.KnowledgeClient;if config.Dependencies["knowledge"]!=""{knowledge=gen.NewKnowledgeClient(config.Dependencies["knowledge"],ModuleSlug,key)}
 return func(ctx context.Context,original platform.OperationContext,input jobs.OwnerRecoveryRequest)(jobs.OwnerRecoveryResult,error){
  result:=jobs.OwnerRecoveryResult{Status:"unknown",PaidResultRefs:[]string{}}
  if knowledge==nil||input.Owner!="knowledge"{return result,platform.NewError(platform.CodeDependencyUnavailable,true,"任务所属域恢复读取尚未装配")}
  if siteID==""||input.DataEpoch!=original.DataEpoch{return result,platform.NewError(platform.CodeVersionMismatch,false,"原任务恢复读取站点或代次不可确认")}
  work,cancel:=context.WithTimeout(ctx,5*time.Second);defer cancel();deadline,_:=work.Deadline();id:=platform.NewIdentifier();expires:=deadline.UTC().Format(time.RFC3339Nano)
  op:=platform.OperationContext{RequestID:id,OperationID:id,RootOperationID:input.RootOperationID,CallerService:ModuleSlug,DataEpoch:input.DataEpoch,Deadline:expires,Actor:platform.AuthContext{Kind:"service",SubjectID:ModuleSlug,SiteID:siteID,ExpiresAt:expires}}
  raw,err:=knowledge.ReadJobRecovery(platform.WithOperationContext(platform.WithPortalProof(work,""),op),input);if err!=nil{return result,err}
  if validate(raw)!=nil||json.Unmarshal(raw,&result)!=nil{return jobs.OwnerRecoveryResult{},platform.NewError(platform.CodeIntegrityError,false,"任务所属域恢复证明不符合合同")}
  if result.ResultRef!=nil&&(result.ResultRef.Owner!=input.Owner||result.ResultRef.Version<1){return jobs.OwnerRecoveryResult{},platform.NewError(platform.CodeIntegrityError,false,"任务所属域结果引用身份不匹配")}
  return result,nil
 },nil
}
