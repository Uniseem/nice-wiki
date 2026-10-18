package runtime

import (
 "context"
 "database/sql"
 "encoding/json"
 "slices"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/lifecycle"
 "nice-wiki/libs/platform/outbox"
 "nice-wiki/modules/model-gateway/internal/gen"
)

type ExecutionOptions struct{WriteWindow platform.WriteWindowConfig;Anchors platform.TrustAnchors;Jobs *gen.JobsClient}
type ExecutionTransaction struct{Tx *sql.Tx;Context context.Context;cancel context.CancelFunc}
type FeeScopeBinding struct{InvocationID string `json:"invocationId"`;RootOperationID string `json:"rootOperationId"`;AdmissionID string `json:"admissionId"`;SessionBudgetScope string `json:"sessionBudgetScope"`;NetworkBudgetScope string `json:"networkBudgetScope"`;DataEpoch string `json:"dataEpoch"`;ExpiresAt time.Time `json:"expiresAt"`}
type InvocationRecord struct{Invocation gen.Invocation;Request gen.CreateInvocationInput;Operation OperationContext;Profile gen.Profile;Route json.RawMessage;Binding *FeeScopeBinding;ActiveAttemptID string;LeaseID string;LeaseUntilNS int64;CancelRequested bool;CreatedNS int64;TerminalNS int64;Output *platform.ModelOutput}
type StreamEvent struct{Sequence int `json:"sequence"`;Event string `json:"event"`;Delta *string `json:"delta,omitempty"`;Invocation *gen.Invocation `json:"invocation,omitempty"`;Code *string `json:"code,omitempty"`;Output *platform.ModelOutput `json:"output,omitempty"`}
type AttemptRecord struct{Attempt gen.Attempt;Model gen.Model;Provider gen.Provider;CredentialVersion int;Price gen.PriceSnapshot;LeaseID string;LeaseUntilNS int64;DispatchedNS int64;UsageHash string;InputTokenBudget int;OutputTokenBudget int}

// InstallExecutionOptions 只装配公共门禁及真实客户端，不执行请求或读取秘密。
func InstallExecutionOptions(options ExecutionOptions){store:=CurrentStore();if store!=nil{store.mu.Lock();store.execution=options;store.mu.Unlock()}}
func ExecutionDependencies()(ExecutionOptions,error){store:=CurrentStore();if store==nil{return ExecutionOptions{},platform.NewError(platform.CodeDependencyUnavailable,false,"执行私有存储未装配")};store.mu.RLock();defer store.mu.RUnlock();return store.execution,nil}
func BeginExecution(ctx context.Context,op OperationContext,write bool)(*ExecutionTransaction,error){
 store:=CurrentStore();if store==nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"模型执行存储未装配")};work:=ctx;cancel:=func(){}
 if write{options,err:=ExecutionDependencies();if err!=nil{return nil,err};if options.WriteWindow.Runtime==nil||options.WriteWindow.WriteWindow==nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"执行权威写窗口未装配")};var view platform.RuntimeView;work,view,cancel,err=lifecycle.NewWriteContext(platform.WithOperationContext(ctx,op),options.WriteWindow);if err!=nil{return nil,err};if view.DataEpoch!=op.DataEpoch{cancel();return nil,platform.NewError(platform.CodeStaleResult,false,"执行权威代际已变化")}}
 tx,err:=store.BeginTx(work);if err!=nil{cancel();return nil,err};var epoch string
 if err:=tx.QueryRowContext(work,`SELECT data_epoch FROM runtime_meta WHERE singleton=1`).Scan(&epoch);err!=nil{tx.Rollback();cancel();return nil,ExecutionStorageError(err)};if epoch==""||epoch!=op.DataEpoch{tx.Rollback();cancel();return nil,platform.NewError(platform.CodeStaleResult,false,"执行本地代际不匹配")}
 if write{var blocked bool;if err:=tx.QueryRowContext(work,`SELECT EXISTS(SELECT 1 FROM barriers WHERE state<>'released')`).Scan(&blocked);err!=nil{tx.Rollback();cancel();return nil,ExecutionStorageError(err)};if blocked{tx.Rollback();cancel();return nil,platform.NewError(platform.CodeLocked,false,"模型执行被本地屏障阻止")}}
 return &ExecutionTransaction{Tx:tx,Context:work,cancel:cancel},nil
}
func(t *ExecutionTransaction)Close(){_=t.Tx.Rollback();t.cancel()}
func(t *ExecutionTransaction)Commit()error{return ExecutionStorageError(t.Tx.Commit())}
func ExecutionStorageError(err error)error{return platform.NormalizeIOError(err,platform.CodeDependencyUnavailable,true,"模型执行持久记录操作失败")}

func ReadInvocationRecord(tx *ExecutionTransaction,op OperationContext,id string)(InvocationRecord,error){
 var record InvocationRecord;var invocationRaw,requestRaw,operationRaw,profileRaw,bindingRaw,outputRaw []byte;var hash,owner,site,epoch,state string;var cancelled int
 err:=tx.Tx.QueryRowContext(tx.Context,`SELECT invocation_json,invocation_hash,request_json,operation_json,profile_json,route_json,fee_binding_json,active_attempt_id,lease_id,lease_until_ns,cancel_requested,created_ns,terminal_ns,terminal_output_json,owner_service,site_id,data_epoch,state FROM invocations WHERE invocation_id=?`,id).Scan(&invocationRaw,&hash,&requestRaw,&operationRaw,&profileRaw,&record.Route,&bindingRaw,&record.ActiveAttemptID,&record.LeaseID,&record.LeaseUntilNS,&cancelled,&record.CreatedNS,&record.TerminalNS,&outputRaw,&owner,&site,&epoch,&state)
 if err==sql.ErrNoRows{return record,platform.NewError(platform.CodeNotFound,false,"模型调用不存在")};if err!=nil{return record,ExecutionStorageError(err)}
 admin:=op.CallerService=="portal"&&op.Actor.Kind=="admin"&&(slices.Contains(op.Actor.Grants,"model.manage")||slices.Contains(op.Actor.Grants,"superadmin"))
 if site!=op.Actor.SiteID||epoch!=op.DataEpoch||(!admin&&owner!=op.CallerService){return InvocationRecord{},platform.NewError(platform.CodeForbidden,false,"不能读取其他站点或owner的模型调用")}
 if len(invocationRaw)==0||platform.HashBytes(invocationRaw)!=hash||json.Unmarshal(invocationRaw,&record.Invocation)!=nil||json.Unmarshal(requestRaw,&record.Request)!=nil||json.Unmarshal(operationRaw,&record.Operation)!=nil||json.Unmarshal(profileRaw,&record.Profile)!=nil{return InvocationRecord{},platform.NewError(platform.CodeIntegrityError,false,"模型调用持久快照不完整")}
 if len(bindingRaw)>0&&json.Unmarshal(bindingRaw,&record.Binding)!=nil{return InvocationRecord{},platform.NewError(platform.CodeIntegrityError,false,"模型调用费用绑定损坏")};if len(outputRaw)>0&&json.Unmarshal(outputRaw,&record.Output)!=nil{return InvocationRecord{},platform.NewError(platform.CodeIntegrityError,false,"模型调用结构输出损坏")}
 if record.Invocation.InvocationId!=id||string(record.Invocation.State)!=state||record.Operation.RootOperationID!=record.Invocation.RootOperationId||record.Operation.CallerService!=owner||record.Operation.Actor.SiteID!=site||record.Operation.DataEpoch!=epoch{return InvocationRecord{},platform.NewError(platform.CodeIntegrityError,false,"模型调用归属或状态矛盾")};if !admin&&(record.Operation.RootOperationID!=op.RootOperationID||record.Operation.CostClass!=op.CostClass){return InvocationRecord{},platform.NewError(platform.CodeForbidden,false,"模型调用根操作或费用类别不能变化")};record.CancelRequested=cancelled!=0;return record,nil
}
func SaveInvocationRecord(tx *ExecutionTransaction,record InvocationRecord)error{
 raw,err:=platform.CanonicalJSON(record.Invocation);if err!=nil{return platform.NewError(platform.CodeIntegrityError,false,"调用状态不能编码")};var output any;if record.Output!=nil{output,_=json.Marshal(record.Output)}
 _,err=tx.Tx.ExecContext(tx.Context,`UPDATE invocations SET invocation_json=?,invocation_hash=?,state=?,active_attempt_id=?,lease_id=?,lease_until_ns=?,cancel_requested=?,terminal_ns=?,terminal_output_json=?,updated_at=? WHERE invocation_id=?`,raw,platform.HashBytes(raw),record.Invocation.State,record.ActiveAttemptID,record.LeaseID,record.LeaseUntilNS,record.CancelRequested,record.TerminalNS,output,platform.Now(),record.Invocation.InvocationId);return ExecutionStorageError(err)
}
func ReadAttemptRecord(tx *ExecutionTransaction,id string)(AttemptRecord,error){
 var record AttemptRecord;var attemptRaw,modelRaw,providerRaw,priceRaw []byte;var usage sql.NullString
 err:=tx.Tx.QueryRowContext(tx.Context,`SELECT attempt_json,model_json,provider_json,credential_version,price_snapshot_json,lease_id,lease_until_ns,dispatched_ns,usage_hash,input_token_budget,output_token_budget FROM invocation_attempts WHERE attempt_id=?`,id).Scan(&attemptRaw,&modelRaw,&providerRaw,&record.CredentialVersion,&priceRaw,&record.LeaseID,&record.LeaseUntilNS,&record.DispatchedNS,&usage,&record.InputTokenBudget,&record.OutputTokenBudget)
 if err==sql.ErrNoRows{return record,platform.NewError(platform.CodeNotFound,false,"独立模型尝试不存在")};if err!=nil{return record,ExecutionStorageError(err)}
 if json.Unmarshal(attemptRaw,&record.Attempt)!=nil||json.Unmarshal(modelRaw,&record.Model)!=nil||json.Unmarshal(providerRaw,&record.Provider)!=nil||json.Unmarshal(priceRaw,&record.Price)!=nil||record.Attempt.AttemptId!=id||record.Attempt.ModelId!=record.Model.ModelId||record.Model.ProviderId!=record.Provider.ProviderId{return AttemptRecord{},platform.NewError(platform.CodeIntegrityError,false,"独立尝试快照损坏")};record.UsageHash=usage.String;return record,nil
}
func SaveAttemptRecord(tx *ExecutionTransaction,record AttemptRecord)error{raw,_:=platform.CanonicalJSON(record.Attempt);_,err:=tx.Tx.ExecContext(tx.Context,`UPDATE invocation_attempts SET attempt_json=?,lease_id=?,lease_until_ns=?,dispatched_ns=?,usage_hash=? WHERE attempt_id=?`,raw,record.LeaseID,record.LeaseUntilNS,record.DispatchedNS,record.UsageHash,record.Attempt.AttemptId);return ExecutionStorageError(err)}
func ReplayExecutionReceipt(tx *ExecutionTransaction,op OperationContext,function,key string,input,result any)(bool,error){
 raw,err:=platform.CanonicalJSON(input);if err!=nil{return false,platform.NewError(platform.CodeValidation,false,"执行输入不能编码")};var inputHash,resultHash string;var stored []byte
 err=tx.Tx.QueryRowContext(tx.Context,`SELECT input_hash,result_json,result_hash FROM execution_receipts WHERE site_id=? AND data_epoch=? AND owner_service=? AND function_id=? AND idempotency_key=?`,op.Actor.SiteID,op.DataEpoch,op.CallerService,function,key).Scan(&inputHash,&stored,&resultHash);if err==sql.ErrNoRows{return false,nil};if err!=nil{return false,ExecutionStorageError(err)};if inputHash!=platform.HashBytes(raw){return false,platform.NewError(platform.CodeConflict,false,"同执行幂等键对应不同输入")};if platform.HashBytes(stored)!=resultHash||json.Unmarshal(stored,result)!=nil{return false,platform.NewError(platform.CodeIntegrityError,false,"执行原回执损坏")};return true,nil
}
func SaveExecutionReceipt(tx *ExecutionTransaction,op OperationContext,function,key string,input,result any)error{raw,_:=platform.CanonicalJSON(input);output,_:=platform.CanonicalJSON(result);_,err:=tx.Tx.ExecContext(tx.Context,`INSERT INTO execution_receipts(site_id,data_epoch,owner_service,function_id,idempotency_key,input_hash,result_json,result_hash,created_ns) VALUES(?,?,?,?,?,?,?,?,?)`,op.Actor.SiteID,op.DataEpoch,op.CallerService,function,key,platform.HashBytes(raw),output,platform.HashBytes(output),time.Now().UnixNano());return ExecutionStorageError(err)}
func SaveInvocationEvent(tx *ExecutionTransaction,record *InvocationRecord,event StreamEvent)error{event.Sequence=record.Invocation.LastSequence+1;if event.Invocation!=nil{copy:=*event.Invocation;copy.LastSequence=event.Sequence;event.Invocation=&copy};raw,err:=platform.CanonicalJSON(event);if err!=nil{return platform.NewError(platform.CodeIntegrityError,false,"模型事件不能编码")};if _,err:=tx.Tx.ExecContext(tx.Context,`INSERT INTO stream_events(invocation_id,sequence,event,payload_json,created_at) VALUES(?,?,?,?,?)`,record.Invocation.InvocationId,event.Sequence,event.Event,raw,platform.Now());err!=nil{return ExecutionStorageError(err)};record.Invocation.LastSequence=event.Sequence;return SaveInvocationRecord(tx,*record)}
func EnqueueExecutionEvent(tx *ExecutionTransaction,op OperationContext,eventType,subject string,revision int,payload any)error{raw,err:=platform.CanonicalJSON(payload);if err!=nil{return platform.NewError(platform.CodeIntegrityError,false,"费用事件不能编码")};return outbox.EnqueueEvent(tx.Tx,platform.EventEnvelope{EventID:eventType+"."+platform.HashBytes(raw),EventType:eventType,EventVersion:1,Producer:"model-gateway",SubjectID:subject,Revision:int64(revision),OccurredAt:platform.Now(),DataEpoch:op.DataEpoch,Payload:raw})}
