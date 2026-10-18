package conversation

import (
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/qa/internal/gen"
 qaruntime "nice-wiki/modules/qa/internal/runtime"
 "nice-wiki/modules/qa/internal/turn"
)

type DeleteConversationInput struct{ConversationId string `json:"conversationId"`;IdempotencyKey string `json:"idempotencyKey"`}
type DeleteConversationResult=gen.DeleteConversationResult

func DeleteConversation(ctx context.Context,op OperationContext,input DeleteConversationInput)(DeleteConversationResult,error){
 tx,err:=qaruntime.BeginWrite(ctx,op);if err!=nil{return DeleteConversationResult{},err}
 var result DeleteConversationResult
 replay,err:=qaruntime.ReplayReceipt(tx,op,"qa.DeleteConversation",input.IdempotencyKey,input,&result);if err!=nil{tx.Close();return result,err};if replay{tx.Close();return result,nil}
 if _,err:=qaruntime.ReadConversation(tx,op,input.ConversationId,true);err!=nil{tx.Close();return result,err}
 rows,err:=tx.Tx.QueryContext(tx.Context,`SELECT turn_id FROM turns WHERE conversation_id=? AND state IN ('admitted','queued','retrieving','generating','cancelled') ORDER BY created_ns,turn_id`,input.ConversationId);if err!=nil{tx.Close();return result,qaruntime.StorageError(err)}
 ids:=make([]string,0);for rows.Next(){var id string;if err:=rows.Scan(&id);err!=nil{rows.Close();tx.Close();return result,qaruntime.StorageError(err)};ids=append(ids,id)};err=rows.Err();rows.Close();tx.Close();if err!=nil{return result,qaruntime.StorageError(err)}
 for _,id:=range ids{cancelled,err:=turn.CancelTurn(ctx,op,turn.CancelTurnInput{TurnId:id,Reason:"conversation_deleted",IdempotencyKey:"qa.delete-turn."+id});if err!=nil{return result,err};if string(cancelled.Turn.State)=="cancelled"{result.CancelledTurns++}}
 tx,err=qaruntime.BeginWrite(ctx,op);if err!=nil{return result,err};defer tx.Close()
 replay,err=qaruntime.ReplayReceipt(tx,op,"qa.DeleteConversation",input.IdempotencyKey,input,&result);if err!=nil{return result,err};if replay{return result,nil}
 if _,err:=qaruntime.ReadConversation(tx,op,input.ConversationId,true);err!=nil{return result,err}
 var active int;if err:=tx.Tx.QueryRowContext(tx.Context,`SELECT COUNT(*) FROM turns WHERE conversation_id=? AND state IN ('admitted','queued','retrieving','generating')`,input.ConversationId).Scan(&active);err!=nil{return result,qaruntime.StorageError(err)};if active!=0{return result,platform.NewError(platform.CodeConflict,false,"会话在清理期间又创建了活动轮次")}
 if err:=qaruntime.RedactConversation(tx,input.ConversationId,"deleted");err!=nil{return result,err}
 result.Deleted=true
 if err:=qaruntime.SaveReceipt(tx,op,"qa.DeleteConversation",input.IdempotencyKey,input,result);err!=nil{return DeleteConversationResult{},err};if err:=tx.Commit();err!=nil{return DeleteConversationResult{},err};return result,nil
}
