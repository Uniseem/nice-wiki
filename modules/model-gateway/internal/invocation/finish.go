package invocation

import (
 "context"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/model-gateway/internal/billing"
 "nice-wiki/modules/model-gateway/internal/gen"
 gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

// ReleaseUnacceptedAttempt只凭未派发或持久明确未接收证明释放，不凭超时推零。
func ReleaseUnacceptedAttempt(ctx context.Context,op OperationContext,invocationID,attemptID string)error{
 tx,err:=gatewayruntime.BeginFinalization(ctx,op);if err!=nil{return err};defer tx.Close();record,err:=gatewayruntime.ReadInvocationRecord(tx,op,invocationID);if err!=nil{return err};attempt,err:=gatewayruntime.ReadAttemptRecord(tx,attemptID);if err!=nil{return err};if attempt.Attempt.InvocationId!=invocationID{return platform.NewError(platform.CodeForbidden,false,"未收费证明不属于本调用")}
 if attempt.DispatchedNS>0{evidence,err:=ReadAttemptEvidence(tx,attemptID);if err!=nil{return err};if evidence.AcceptanceProof!="not_accepted"{return platform.NewError(platform.CodeConflict,false,"供应方接收未知不能释放held")}}
 reservation,err:=billing.ReadReservation(tx,attempt.Attempt.ReservationId);if err!=nil{return err};if reservation.State!="held"{return nil};var zero any;if reservation.PricingKnown{zero=0}
 if _,err:=tx.Tx.ExecContext(tx.Context,`UPDATE reservations SET state='released',held_micro_usd=?,version=version+1 WHERE reservation_id=?`,zero,reservation.ReservationId);err!=nil{return gatewayruntime.ExecutionStorageError(err)};if _,err:=tx.Tx.ExecContext(tx.Context,`UPDATE fee_commitments SET state='released',amount_micro_usd=0,pricing_known=1 WHERE reservation_id=?`,reservation.ReservationId);err!=nil{return gatewayruntime.ExecutionStorageError(err)}
 attempt.Attempt.State=gen.AttemptState("failed");if err:=gatewayruntime.SaveAttemptRecord(tx,attempt);err!=nil{return err};for i:=range record.Invocation.Attempts{if record.Invocation.Attempts[i].AttemptId==attemptID{record.Invocation.Attempts[i]=attempt.Attempt}};if err:=gatewayruntime.SaveInvocationRecord(tx,record);err!=nil{return err};return tx.Commit()
}
func FinishCancelledInvocation(ctx context.Context,op OperationContext,record gatewayruntime.InvocationRecord)(string,error){
 if record.ActiveAttemptID!=""{work,cancel:=context.WithTimeout(context.WithoutCancel(ctx),5*time.Second);defer cancel();attempt,err:=ReadExecutionAttempt(work,op,record.ActiveAttemptID);if err!=nil{return "",err};if attempt.DispatchedNS==0{if err:=ReleaseUnacceptedAttempt(ctx,op,record.Invocation.InvocationId,attempt.Attempt.AttemptId);err!=nil{return "",err}}else{
  tx,err:=gatewayruntime.BeginFinalization(ctx,op);if err!=nil{return "",err};reservation,err:=billing.ReadReservation(tx,attempt.Attempt.ReservationId);tx.Close();if err!=nil{return "",err};if reservation.State=="held"{if _,err:=billing.MarkUnknownCharge(ctx,op,billing.MarkUnknownChargeInput{InvocationID:record.Invocation.InvocationId,AttemptID:attempt.Attempt.AttemptId,Reason:"cancel_after_dispatch",ProviderRequestID:attempt.Attempt.ProviderRequestId});err!=nil{return "",err}}
 }}
 if err:=FinishInvocation(ctx,op,record.Invocation.InvocationId,"cancelled",platform.CodeCancelled);err!=nil{return "",err};return "cancelled",nil
}
func FinishInvocation(ctx context.Context,op OperationContext,id,state,code string)error{
 tx,err:=gatewayruntime.BeginFinalization(ctx,op);if err!=nil{return err};defer tx.Close();record,err:=gatewayruntime.ReadInvocationRecord(tx,op,id);if err!=nil{return err}
 if record.Invocation.State=="succeeded"||record.Invocation.State=="cancelled"||record.Invocation.State=="failed"||record.Invocation.State=="reconciled"{return nil}
 if record.CancelRequested{state="cancelled";code=platform.CodeCancelled}
 if record.ActiveAttemptID!=""{attempt,err:=gatewayruntime.ReadAttemptRecord(tx,record.ActiveAttemptID);if err!=nil{return err}
  if state=="succeeded"{evidence,err:=ReadAttemptEvidence(tx,attempt.Attempt.AttemptId);if err!=nil{return err};if !evidence.Completed||evidence.Code!=""||record.Invocation.ResultAttemptId==nil||*record.Invocation.ResultAttemptId!=attempt.Attempt.AttemptId{return platform.NewError(platform.CodeIntegrityError,false,"未完成校验的供应方结果不能成功交付")}}
  if attempt.Attempt.State!="charge_unknown"&&attempt.Attempt.State!="reconciled"{if state=="succeeded"{attempt.Attempt.State=gen.AttemptState("succeeded")}else{attempt.Attempt.State=gen.AttemptState("failed")};if err:=gatewayruntime.SaveAttemptRecord(tx,attempt);err!=nil{return err};for i:=range record.Invocation.Attempts{if record.Invocation.Attempts[i].AttemptId==attempt.Attempt.AttemptId{record.Invocation.Attempts[i]=attempt.Attempt}}}
 }
 record.Invocation.State=gen.InvocationState(state);record.TerminalNS=time.Now().UnixNano();record.LeaseID="";record.LeaseUntilNS=0
 event:=gatewayruntime.StreamEvent{Event:"error",Invocation:&record.Invocation,Code:Nonempty(code)};if state=="succeeded"{event.Event="completed";event.Code=nil;event.Output=record.Output}else if state=="cancelled"{event.Event="cancelled"}
 if err:=gatewayruntime.SaveInvocationEvent(tx,&record,event);err!=nil{return err};if err:=gatewayruntime.ReleaseResolvedReferences(tx,record);err!=nil{return err};return tx.Commit()
}

// FinishInterruptedInvocation在本地IO失败后仅收敛已持久派发事实，不重发供应方请求。
func FinishInterruptedInvocation(ctx context.Context,op OperationContext,id,state,code string)error{
 tx,err:=gatewayruntime.BeginFinalization(ctx,op);if err!=nil{return err};record,err:=gatewayruntime.ReadInvocationRecord(tx,op,id);if err!=nil{tx.Close();return err};var attempt gatewayruntime.AttemptRecord;var held bool
 if record.ActiveAttemptID!=""{attempt,err=gatewayruntime.ReadAttemptRecord(tx,record.ActiveAttemptID);if err==nil{reservation,readErr:=billing.ReadReservation(tx,attempt.Attempt.ReservationId);err=readErr;held=reservation.State=="held"}};tx.Close();if err!=nil{return err}
 if record.ActiveAttemptID!=""&&held{if attempt.DispatchedNS==0{if err:=ReleaseUnacceptedAttempt(ctx,op,id,attempt.Attempt.AttemptId);err!=nil{return err}}else{if _,err:=billing.MarkUnknownCharge(ctx,op,billing.MarkUnknownChargeInput{InvocationID:id,AttemptID:attempt.Attempt.AttemptId,Reason:"local_evidence_unavailable",ProviderRequestID:attempt.Attempt.ProviderRequestId});err!=nil{return err};state="charge_unknown"}}
 return FinishInvocation(ctx,op,id,state,code)
}
