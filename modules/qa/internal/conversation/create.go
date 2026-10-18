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
 tx,err:=qaruntime.BeginWrite(ctx,op);if err!=nil{return CreateConversationResult{},err}
 var result CreateConversationResult
 replay,err:=qaruntime.ReplayReceipt(tx,op,"qa.CreateConversation",input.IdempotencyKey,input,&result);tx.Close();if err!=nil{return result,err};if replay{return result,nil}
 tag,err:=language.Parse(input.Language);if err!=nil{return result,platform.NewError(platform.CodeValidation,false,"对话语言不合法")}
 // 生成DTO与平台标量同义，显式转换保留所有签名许可字段。
 permit:=platform.AdmissionPermit{AdmissionID:input.Admission.AdmissionId,RequestID:input.Admission.RequestId,SubjectID:input.Admission.SubjectId,Action:input.Admission.Action,Token:input.Admission.Token,ExpiresAt:input.Admission.ExpiresAt,SessionBudgetScope:input.Admission.SessionBudgetScope,NetworkBudgetScope:input.Admission.NetworkBudgetScope,DataEpoch:input.Admission.DataEpoch,Audience:string(input.Admission.Audience)}
 if _,err:=admission.ValidateAdmission(ctx,op,admission.ValidateAdmissionInput{Admission:permit,Action:"qa.conversation"});err!=nil{return result,err}
 tx,err=qaruntime.BeginWrite(ctx,op);if err!=nil{return result,err};defer tx.Close()
 replay,err=qaruntime.ReplayReceipt(tx,op,"qa.CreateConversation",input.IdempotencyKey,input,&result);if err!=nil{return result,err};if replay{return result,nil}
 result.Conversation=gen.Conversation{ConversationId:platform.NewIdentifier(),VisitorSubject:op.Actor.SubjectID,Language:tag.String(),State:gen.ConversationState("active"),Version:1,HistoryGeneration:0,ExpiresAt:time.Now().Add(30*24*time.Hour).UTC().Format(time.RFC3339Nano)}
 _,err=tx.Tx.ExecContext(tx.Context,`INSERT INTO conversations(conversation_id,site_id,data_epoch,visitor_subject,language,state,version,history_generation,expires_at) VALUES(?,?,?,?,?,'active',1,0,?)`,result.Conversation.ConversationId,op.Actor.SiteID,op.DataEpoch,op.Actor.SubjectID,result.Conversation.Language,result.Conversation.ExpiresAt)
 if err!=nil{return CreateConversationResult{},qaruntime.StorageError(err)}
 if err:=qaruntime.SaveReceipt(tx,op,"qa.CreateConversation",input.IdempotencyKey,input,result);err!=nil{return CreateConversationResult{},err}
 if err:=tx.Commit();err!=nil{return CreateConversationResult{},err};return result,nil
}
