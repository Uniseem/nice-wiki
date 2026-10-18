package concurrency

import (
 "context"
 "database/sql"
 "time"
 "nice-wiki/libs/platform"
 gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)
type OperationContext=platform.OperationContext
type AcquireProviderSlotInput struct{ProviderID string `json:"providerId"`;InvocationID string `json:"invocationId"`;LeaseSeconds int `json:"leaseSeconds"`}
type AcquireProviderSlotResult struct{LeaseID string `json:"leaseId"`;ExpiresAt time.Time `json:"expiresAt"`}
type ReleaseProviderSlotInput struct{LeaseID string `json:"leaseId"`;InvocationID string `json:"invocationId"`}
type ReleaseProviderSlotResult struct{Released bool `json:"released"`}
func AcquireProviderSlot(ctx context.Context,op OperationContext,input AcquireProviderSlotInput)(AcquireProviderSlotResult,error){
 if input.LeaseSeconds!=30{return AcquireProviderSlotResult{},platform.NewError(platform.CodeValidation,false,"供应方租约固定30秒")}
 tx,err:=gatewayruntime.BeginExecution(ctx,op,true);if err!=nil{return AcquireProviderSlotResult{},err};defer tx.Close();record,err:=gatewayruntime.ReadInvocationRecord(tx,op,input.InvocationID);if err!=nil{return AcquireProviderSlotResult{},err};attempt,err:=gatewayruntime.ReadAttemptRecord(tx,record.ActiveAttemptID);if err!=nil{return AcquireProviderSlotResult{},err}
 if attempt.Provider.ProviderId!=input.ProviderID||record.CancelRequested||attempt.Attempt.State!="reserved"{return AcquireProviderSlotResult{},platform.NewError(platform.CodeStaleResult,false,"槽位不属于当前待派发尝试")}
 now:=time.Now().UTC();if _,err:=tx.Tx.ExecContext(tx.Context,`DELETE FROM provider_slots WHERE expires_ns<=?`,now.UnixNano());err!=nil{return AcquireProviderSlotResult{},gatewayruntime.ExecutionStorageError(err)}
 var existing string;var expires int64
 err=tx.Tx.QueryRowContext(tx.Context,`SELECT lease_id,expires_ns FROM provider_slots WHERE provider_id=? AND invocation_id=? AND data_epoch=?`,input.ProviderID,input.InvocationID,op.DataEpoch).Scan(&existing,&expires);if err==nil{return AcquireProviderSlotResult{LeaseID:existing,ExpiresAt:time.Unix(0,expires).UTC()},nil};if err!=sql.ErrNoRows{return AcquireProviderSlotResult{},gatewayruntime.ExecutionStorageError(err)}
 var providerCount,profileCount int
 if err:=tx.Tx.QueryRowContext(tx.Context,`SELECT COUNT(*) FROM provider_slots WHERE provider_id=? AND data_epoch=?`,input.ProviderID,op.DataEpoch).Scan(&providerCount);err!=nil{return AcquireProviderSlotResult{},gatewayruntime.ExecutionStorageError(err)}
 if err:=tx.Tx.QueryRowContext(tx.Context,`SELECT COUNT(*) FROM provider_slots s JOIN invocations i ON i.invocation_id=s.invocation_id JOIN fee_commitments f ON f.attempt_id=i.active_attempt_id WHERE s.data_epoch=? AND f.site_id=? AND f.profile_id=?`,op.DataEpoch,op.Actor.SiteID,record.Profile.ProfileId).Scan(&profileCount);err!=nil{return AcquireProviderSlotResult{},gatewayruntime.ExecutionStorageError(err)}
 if providerCount>=attempt.Provider.ConcurrencyLimit||profileCount>=record.Profile.Concurrency{return AcquireProviderSlotResult{},platform.NewError(platform.CodeRateLimited,true,"供应方或档位并发槽已满")}
 until:=now.Add(30*time.Second);if record.Invocation.Deadline.Before(until){until=record.Invocation.Deadline};if !until.After(now){return AcquireProviderSlotResult{},platform.NewError(platform.CodeDeadlineExceeded,false,"模型调用原截止已到")}
 id:=platform.NewIdentifier();if _,err:=tx.Tx.ExecContext(tx.Context,`INSERT INTO provider_slots(lease_id,provider_id,invocation_id,data_epoch,expires_ns) VALUES(?,?,?,?,?)`,id,input.ProviderID,input.InvocationID,op.DataEpoch,until.UnixNano());err!=nil{return AcquireProviderSlotResult{},gatewayruntime.ExecutionStorageError(err)}
 if err:=tx.Commit();err!=nil{return AcquireProviderSlotResult{},err};return AcquireProviderSlotResult{LeaseID:id,ExpiresAt:until},nil
}
func ReleaseProviderSlot(ctx context.Context,op OperationContext,input ReleaseProviderSlotInput)(ReleaseProviderSlotResult,error){
 tx,err:=gatewayruntime.BeginExecution(ctx,op,true);if err!=nil{return ReleaseProviderSlotResult{},err};defer tx.Close();if _,err:=gatewayruntime.ReadInvocationRecord(tx,op,input.InvocationID);err!=nil{return ReleaseProviderSlotResult{},err};var invocationID string
 err=tx.Tx.QueryRowContext(tx.Context,`SELECT invocation_id FROM provider_slots WHERE lease_id=?`,input.LeaseID).Scan(&invocationID);if err==sql.ErrNoRows{return ReleaseProviderSlotResult{Released:true},nil};if err!=nil{return ReleaseProviderSlotResult{},gatewayruntime.ExecutionStorageError(err)};if invocationID!=input.InvocationID{return ReleaseProviderSlotResult{},platform.NewError(platform.CodeForbidden,false,"不能释放其他调用槽位")};if _,err:=tx.Tx.ExecContext(tx.Context,`DELETE FROM provider_slots WHERE lease_id=?`,input.LeaseID);err!=nil{return ReleaseProviderSlotResult{},gatewayruntime.ExecutionStorageError(err)};if err:=tx.Commit();err!=nil{return ReleaseProviderSlotResult{},err};return ReleaseProviderSlotResult{Released:true},nil
}
