package conversation

import(
 "context"
 "encoding/json"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/serviceauth"
 "nice-wiki/modules/qa/internal/gen"
 qaruntime "nice-wiki/modules/qa/internal/runtime"
)
type GetConversationInput struct{ConversationID string `json:"conversationId"`;Cursor string `json:"cursor,omitempty"`;Limit int `json:"limit"`}
type GetConversationResult=gen.GetConversationResult

func GetConversation(ctx context.Context,op OperationContext,input GetConversationInput)(GetConversationResult,error){
 if input.Limit<1||input.Limit>100{return GetConversationResult{},platform.NewError(platform.CodeValidation,false,"历史分页数量不合法")}
 tx,err:=qaruntime.BeginRead(ctx,op);if err!=nil{return GetConversationResult{},err};defer tx.Close()
 conversation,err:=qaruntime.ReadConversation(tx,op,input.ConversationID,true);if err!=nil{return GetConversationResult{},err}
 binding,_:=platform.CanonicalJSON(struct{Site,Epoch,Subject,Conversation string;Generation,Limit int}{op.Actor.SiteID,op.DataEpoch,op.Actor.SubjectID,input.ConversationID,conversation.HistoryGeneration,input.Limit})
 scope:="qa.messages.v1:"+platform.HashBytes(binding);after:=-1
 if input.Cursor!=""{raw,err:=serviceauth.VerifyCursor(tx.Runtime.SigningKey,scope,input.Cursor);if err!=nil{return GetConversationResult{},err};if json.Unmarshal(raw,&after)!=nil||after<0{return GetConversationResult{},platform.NewError(platform.CodeValidation,false,"历史游标不合法")}}
 rows,err:=tx.Tx.QueryContext(tx.Context,`SELECT message_id,ordinal,role,text,turn_id,revision FROM messages WHERE conversation_id=? AND ordinal>? ORDER BY ordinal LIMIT ?`,input.ConversationID,after,input.Limit+1)
 if err!=nil{return GetConversationResult{},qaruntime.StorageError(err)};defer rows.Close()
 messages:=make([]gen.Message,0)
 for rows.Next(){var message gen.Message;var role string;if err:=rows.Scan(&message.MessageId,&message.Ordinal,&role,&message.Text,&message.TurnId,&message.Revision);err!=nil{return GetConversationResult{},qaruntime.StorageError(err)};message.Role=gen.MessageRole(role);messages=append(messages,message)}
 if err:=rows.Err();err!=nil{return GetConversationResult{},qaruntime.StorageError(err)}
 result:=GetConversationResult{Conversation:conversation,Messages:messages[:min(len(messages),input.Limit)]}
 if len(messages)>input.Limit{raw,_:=json.Marshal(messages[input.Limit-1].Ordinal);cursor,err:=serviceauth.SignCursor(tx.Runtime.SigningKey,scope,raw);if err!=nil{return GetConversationResult{},err};result.NextCursor=&cursor}
 return result,nil
}
