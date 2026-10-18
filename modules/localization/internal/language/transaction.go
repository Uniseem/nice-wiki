package language

import (
 "context"
 "database/sql"
 "encoding/json"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/outbox"
)

// ReadData 固定本域已核代次与屏障下的一致快照，不向调用方泄露其他域资源。
func ReadData(ctx context.Context,op OperationContext,read func(*sql.Tx)error)error{
 tx,err:=openLanguage(ctx,op);if err!=nil{return err};defer tx.Rollback();if err=rejectHeldBarrier(ctx,tx);err!=nil{return err};return read(tx)
}
// Mutate 复用既有语言写入的七天回执和事务；业务CAS、审计事件由用例写在同一事务。
func Mutate[I,R any](ctx context.Context,op OperationContext,functionID,key string,input I,result *R,write func(*sql.Tx)error)error{
 tx,err:=openLanguage(ctx,op);if err!=nil{return err};defer tx.Rollback()
 digest,err:=digestInput(struct{Epoch string `json:"epoch"`;Input I `json:"input"`}{op.DataEpoch,input});if err!=nil{return err}
 raw,found,err:=readReceipt(ctx,tx,op,functionID,key,digest);if err!=nil{return err};if found{if json.Unmarshal(raw,result)!=nil{return platform.NewError(platform.CodeIntegrityError,false,"本地化原回执损坏")};return nil}
 if err=rejectHeldBarrier(ctx,tx);err!=nil{return err};if err=write(tx);err!=nil{return err}
 if err=writeReceipt(ctx,tx,op,functionID,key,digest,result);err!=nil{return err};return finishTx(tx)
}
// RequireAdministrator 的功能权限由平台HTTP入口核验一次，内部只落实人工主体约束。
func RequireAdministrator(op OperationContext)error{if op.Actor.Kind!="admin"{return platform.NewError(platform.CodeForbidden,false,"此本地化操作仅接受管理员")};return nil}
// EmitReference 登记不含正文的本域引用事件。
func EmitReference(tx *sql.Tx,op OperationContext,event,subject string,revision int,payload any)error{
 raw,err:=platform.CanonicalJSON(payload);if err!=nil{return platform.NewError(platform.CodeInternal,false,"本地化事件编码失败")};return outbox.EnqueueEvent(tx,platform.EventEnvelope{EventID:platform.NewIdentifier(),EventType:event,EventVersion:1,Producer:"localization",SubjectID:subject,Revision:int64(revision),OccurredAt:platform.Now(),DataEpoch:op.DataEpoch,Payload:raw})
}
func StorageError(err error)error{if err==nil{return nil};return platform.NormalizeIOError(err,platform.CodeInternal,false,"本地化私有存储失败")}
