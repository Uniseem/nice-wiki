package conversation

import(
 "context"
 "time"
 "golang.org/x/text/language"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/qa/internal/admission"
 "nice-wiki/modules/qa/internal/gen"
 qaruntime "nice-wiki/modules/qa/internal/runtime"
)
type OperationContext=platform.OperationContext
type CreateConversationInput=gen.CreateConversationInput
type CreateConversationResult=gen.CreateConversationResult

func CreateConversation(ctx context.Context,op OperationContext,input CreateConversationInput)(CreateConversationResult,error){
 if op.CallerService!="portal"||op.Actor.Kind!="visitor"{return CreateConversationResult{},platform.NewError(platform.CodeForbidden,false,"仅允许当前伪名访客创建对话")}
 if input.Admission.RequestId!=op.RootOperationID||input.Admission.Action!="qa.conversation"||input.Admission.SubjectId!=op.Actor.SubjectID||input.Admission.DataEpoch!=op.DataEpoch||input.Admission.Token!=""{return CreateConversationResult{},platform.NewError(platform.CodeForbidden,false,"免费会话准入与原根、主体或代际不匹配")}
 runtime,err:=qaruntime.FromContext(ctx);if err!=nil{return CreateConversationResult{},err};if runtime.Guard==nil{return CreateConversationResult{},platform.NewError(platform.CodeDependencyUnavailable,false,"会话准入权威尚未装配")}
 var result CreateConversationResult
 // 仅在本地提交/原回执读取结束后调用；重放补释放，不再次消费或创建会话。
 finish:=func()(CreateConversationResult,error){
  raw,err:=runtime.Guard.ReleaseAdmission(qaruntime.ServiceReadContext(ctx,op),input.Admission.AdmissionId,struct{}{},"qa.conversation.release."+platform.HashBytes([]byte(op.RootOperationID+":"+input.Admission.AdmissionId)));if err!=nil{return CreateConversationResult{},err}
  var released struct{Released bool `json:"released"`};if err:=qaruntime.DecodeResponse(runtime,"GuardReleasePermitResult",raw,&released);err!=nil{return CreateConversationResult{},err};if !released.Released{return CreateConversationResult{},platform.NewError(platform.CodeDependencyUnavailable,false,"会话准入占位尚未释放")};return result,nil
 }
 tx,err:=qaruntime.BeginWrite(ctx,op);if err!=nil{return CreateConversationResult{},err}
 replay,err:=qaruntime.ReplayReceipt(tx,op,"qa.CreateConversation",input.IdempotencyKey,input,&result);tx.Close();if err!=nil{return result,err};if replay{return finish()}
 tag,err:=language.Parse(input.Language);if err!=nil{return result,platform.NewError(platform.CodeValidation,false,"对话语言不合法")}
 // 生成DTO与平台标量同义，显式转换保留所有签名许可字段。
 permit:=platform.AdmissionPermit{AdmissionID:input.Admission.AdmissionId,RequestID:input.Admission.RequestId,SubjectID:input.Admission.SubjectId,Action:input.Admission.Action,Token:input.Admission.Token,ExpiresAt:input.Admission.ExpiresAt.UTC().Format(time.RFC3339Nano),SessionBudgetScope:input.Admission.SessionBudgetScope,NetworkBudgetScope:input.Admission.NetworkBudgetScope,DataEpoch:input.Admission.DataEpoch,Audience:string(input.Admission.Audience)}
 if _,err:=admission.ValidateAdmission(ctx,op,admission.ValidateAdmissionInput{Admission:permit,Action:"qa.conversation"});err!=nil{return result,err}
 tx,err=qaruntime.BeginWrite(ctx,op);if err!=nil{return result,err};defer tx.Close()
 replay,err=qaruntime.ReplayReceipt(tx,op,"qa.CreateConversation",input.IdempotencyKey,input,&result);if err!=nil{return result,err};if replay{tx.Close();return finish()}
 result.Conversation=gen.Conversation{ConversationId:platform.NewIdentifier(),VisitorSubject:op.Actor.SubjectID,Language:tag.String(),State:gen.ConversationState("active"),Version:1,HistoryGeneration:0,ExpiresAt:time.Now().Add(30*24*time.Hour).UTC()}
 _,err=tx.Tx.ExecContext(tx.Context,`INSERT INTO conversations(conversation_id,site_id,data_epoch,visitor_subject,language,state,version,history_generation,expires_at) VALUES(?,?,?,?,?,'active',1,0,?)`,result.Conversation.ConversationId,op.Actor.SiteID,op.DataEpoch,op.Actor.SubjectID,result.Conversation.Language,result.Conversation.ExpiresAt.UTC().Format(time.RFC3339Nano))
 if err!=nil{return CreateConversationResult{},qaruntime.StorageError(err)}
 if err:=qaruntime.SaveReceipt(tx,op,"qa.CreateConversation",input.IdempotencyKey,input,result);err!=nil{return CreateConversationResult{},err}
 if err:=tx.Commit();err!=nil{return CreateConversationResult{},err};tx.Close();return finish()
}
