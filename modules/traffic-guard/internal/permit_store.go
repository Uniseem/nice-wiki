package trafficguard

import (
 "context"
 "database/sql"
 "errors"
 "time"
 "nice-wiki/libs/platform"
)

type AcquireConcurrencyInput struct { SubjectKey string;Action string;RequestID string;DataEpoch string }
type AcquireConcurrencyResult struct { AdmissionID string;ExpiresAt string }
type RenewPermitInput struct { AdmissionID string;CallerService string;DataEpoch string }
type RenewPermitResult struct { Permit platform.AdmissionPermit `json:"permit"` }
type ReleasePermitInput struct { AdmissionID string;CallerService string;DataEpoch string }
type ReleasePermitResult struct { Released bool `json:"released"` }

type permitRecord struct {
 admissionID,requestID,owner,subject,session,network,action,state,inputHash string
 consumed string
 expires,hardDeadline,created,revision,policyRevision int64
 networks []string
}

// AcquireConcurrency 复用外层准入事务，绝不单独提交一半扣桶结果。
func AcquireConcurrency(ctx context.Context,op OperationContext,in AcquireConcurrencyInput)(AcquireConcurrencyResult,error) {
 if in.RequestID!=op.RootOperationID { return AcquireConcurrencyResult{},platform.NewError(platform.CodeValidation,false,"许可必须绑定原请求根") }
 t,err:=beginGuardTransaction(ctx,op,true)
 if err!=nil { return AcquireConcurrencyResult{},err }
 defer t.close()
 classification,err:=ClassifyAction(t.ctx,op,ClassifyActionInput{Action:in.Action})
 owner:=classification.OwnerCaller
 if err!=nil { return AcquireConcurrencyResult{},err }
 digestInput,_:=platform.CanonicalJSON(struct{Subject,Action,Request string}{in.SubjectKey,in.Action,in.RequestID})
 digest:=platform.HashBytes(digestInput)
 var existingID,existingHash string
 err=t.tx.QueryRowContext(t.ctx,`SELECT admission_id,input_hash FROM guard_permits WHERE site_id=? AND data_epoch=? AND owner_caller=? AND request_id=?`,t.site,op.DataEpoch,owner,in.RequestID).Scan(&existingID,&existingHash)
 if err==nil {
  if existingHash!=digest { return AcquireConcurrencyResult{},platform.NewError(platform.CodeConflict,false,"原请求已绑定其他主体或动作") }
  row,err:=readPermitRecord(t,existingID)
  if err!=nil { return AcquireConcurrencyResult{},err }
  return AcquireConcurrencyResult{row.admissionID,time.Unix(0,row.expires).UTC().Format(time.RFC3339Nano)},nil
 }
 if !errors.Is(err,sql.ErrNoRows) { return AcquireConcurrencyResult{},loginDBError() }
 policy,revision,err:=readGuardPolicy(t)
 if err!=nil { return AcquireConcurrencyResult{},err }
 var session,network string
 err=t.tx.QueryRowContext(t.ctx,`SELECT session_key,network_key FROM guard_subjects WHERE site_id=? AND data_epoch=? AND subject_key=?`,t.site,op.DataEpoch,in.SubjectKey).Scan(&session,&network)
 if errors.Is(err,sql.ErrNoRows) { return AcquireConcurrencyResult{},platform.NewError(platform.CodeNotFound,false,"已验证匿名主体不存在") }
 if err!=nil { return AcquireConcurrencyResult{},loginDBError() }
 var subjectCount,siteCount,sessionCount int
 if t.tx.QueryRowContext(t.ctx,`SELECT COUNT(*),COALESCE(SUM(subject_key=? AND owner_caller=?),0),COALESCE(SUM(session_key=? AND owner_caller='qa'),0) FROM guard_permits WHERE site_id=? AND data_epoch=? AND state='active' AND expires_at_ns>?`,in.SubjectKey,owner,session,t.site,op.DataEpoch,t.now.UnixNano()).Scan(&siteCount,&subjectCount,&sessionCount)!=nil { return AcquireConcurrencyResult{},loginDBError() }
 if subjectCount>=policy.QaSubjectConcurrency||siteCount>=policy.QaSiteConcurrency||(owner=="qa"&&sessionCount>=1) { return AcquireConcurrencyResult{},platform.NewError(platform.CodeRateLimited,true,"当前并发已满，等待原请求释放") }
 if owner=="qa" {
  var head string
  err=t.tx.QueryRowContext(t.ctx,`SELECT request_id FROM guard_waiters WHERE site_id=? AND data_epoch=? AND session_key=? ORDER BY sequence LIMIT 1`,t.site,op.DataEpoch,session).Scan(&head)
  if err!=nil&&!errors.Is(err,sql.ErrNoRows) { return AcquireConcurrencyResult{},loginDBError() }
  if err==nil&&head!=in.RequestID { return AcquireConcurrencyResult{},platform.NewError(platform.CodeRateLimited,true,"原会话按FIFO等待") }
 }
 admissionID:=platform.NewIdentifier()
 expires:=t.now.Add(30*time.Second)
 _,err=t.tx.ExecContext(t.ctx,`INSERT INTO guard_permits(admission_id,site_id,data_epoch,owner_caller,request_id,input_hash,subject_key,session_key,network_key,action,
  expires_at_ns,hard_deadline_ns,created_at_ns,state,consumed_request_id,revision,policy_revision)
  VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'active',NULL,1,?)`,admissionID,t.site,op.DataEpoch,owner,in.RequestID,digest,in.SubjectKey,session,network,in.Action,expires.UnixNano(),t.now.Add(300*time.Second).UnixNano(),t.now.UnixNano(),revision)
 if err!=nil { return AcquireConcurrencyResult{},loginDBError() }
 _,err=t.tx.ExecContext(t.ctx,`INSERT INTO guard_permit_networks(admission_id,network_key)
  SELECT ?,network_key FROM guard_network_aliases WHERE site_id=? AND data_epoch=? AND subject_key=?`,admissionID,t.site,op.DataEpoch,in.SubjectKey)
 if err!=nil { return AcquireConcurrencyResult{},loginDBError() }
 _,err=t.tx.ExecContext(t.ctx,`DELETE FROM guard_waiters WHERE site_id=? AND data_epoch=? AND session_key=? AND request_id=?`,t.site,op.DataEpoch,session,in.RequestID)
 if err!=nil { return AcquireConcurrencyResult{},loginDBError() }
 if err=t.commit();err!=nil { return AcquireConcurrencyResult{},err }
 return AcquireConcurrencyResult{admissionID,expires.Format(time.RFC3339Nano)},nil
}

func RenewPermit(ctx context.Context,op OperationContext,in RenewPermitInput)(RenewPermitResult,error) {
 t,err:=beginGuardTransaction(ctx,op,true)
 if err!=nil { return RenewPermitResult{},err }
 defer t.close()
 row,err:=readPermitRecord(t,in.AdmissionID)
 if err!=nil { return RenewPermitResult{},err }
 if in.CallerService!=op.CallerService||row.owner!=op.CallerService||row.requestID!=op.RootOperationID { return RenewPermitResult{},platform.NewError(platform.CodeForbidden,false,"续租请求不属于原根和拥有者") }
 if row.state!="active"||row.expires<=t.now.UnixNano()||row.hardDeadline<=t.now.UnixNano() { return RenewPermitResult{},platform.NewError(platform.CodeStaleResult,false,"许可已终止或到期，不能复活") }
 if row.owner=="qa" {
  if row.consumed!=row.requestID { return RenewPermitResult{},platform.NewError(platform.CodeConflict,false,"问答许可尚未消费") }
  banned,_,err:=CheckBan(t,row.networks)
  if err!=nil { return RenewPermitResult{},err }
  if banned { return RenewPermitResult{},platform.NewError(platform.CodeForbidden,false,"该网络AI功能暂被限制") }
 }
 policy,policyRevision,err:=readGuardPolicy(t)
 if err!=nil { return RenewPermitResult{},err }
 var siteCount,subjectCount,sessionCount int
 if t.tx.QueryRowContext(t.ctx,`SELECT COUNT(*),COALESCE(SUM(subject_key=? AND owner_caller=?),0),COALESCE(SUM(session_key=? AND owner_caller='qa'),0) FROM guard_permits WHERE site_id=? AND data_epoch=? AND state='active' AND expires_at_ns>?`,row.subject,row.owner,row.session,t.site,op.DataEpoch,t.now.UnixNano()).Scan(&siteCount,&subjectCount,&sessionCount)!=nil { return RenewPermitResult{},loginDBError() }
 if siteCount>policy.QaSiteConcurrency||subjectCount>policy.QaSubjectConcurrency||(row.owner=="qa"&&sessionCount>1) { return RenewPermitResult{},platform.NewError(platform.CodeRateLimited,true,"最新并发策略不允许续租") }
 var receipt struct{ Expires int64 `json:"expires"` }
 receiptInput:=struct{ AdmissionID string }{in.AdmissionID}
 found,err:=readGuardReceipt(t,"permit.renew",receiptInput,&receipt)
 if err!=nil { return RenewPermitResult{},err }
 if !found {
  receipt.Expires=t.now.Add(30*time.Second).UnixNano()
  if receipt.Expires>row.hardDeadline { receipt.Expires=row.hardDeadline }
  _,err=t.tx.ExecContext(t.ctx,`UPDATE guard_permits SET expires_at_ns=?,revision=revision+1,policy_revision=? WHERE admission_id=? AND site_id=? AND data_epoch=? AND revision=? AND state='active'`,receipt.Expires,policyRevision,row.admissionID,t.site,op.DataEpoch,row.revision)
  if err!=nil { return RenewPermitResult{},loginDBError() }
  if err=saveGuardReceipt(t,"permit.renew",receiptInput,receipt);err!=nil { return RenewPermitResult{},err }
 }
 if receipt.Expires<=t.now.UnixNano() { return RenewPermitResult{},platform.NewError(platform.CodeStaleResult,false,"原续租期限已过，不得重签复活") }
 var permit platform.AdmissionPermit
 if row.owner=="qa" {
  expiry:=time.Unix(0,receipt.Expires).UTC().Format(time.RFC3339Nano)
  signed,err:=SignPermit(t.ctx,op,SignPermitInput{AdmissionID:row.admissionID,ExpiresAt:&expiry})
  if err!=nil { return RenewPermitResult{},err }
  permit=signed.Permit
 } else {
  row.expires=receipt.Expires
  permit=permitMetadata(row,op.DataEpoch)
 }
 if err=t.commit();err!=nil { return RenewPermitResult{},err }
 return RenewPermitResult{Permit:permit},nil
}

func ReleasePermit(ctx context.Context,op OperationContext,in ReleasePermitInput)(ReleasePermitResult,error) {
 t,err:=beginGuardTransaction(ctx,op,true)
 if err!=nil { return ReleasePermitResult{},err }
 defer t.close()
 row,err:=readPermitRecord(t,in.AdmissionID)
 if err!=nil { return ReleasePermitResult{},err }
 if in.CallerService!=op.CallerService||row.owner!=op.CallerService { return ReleasePermitResult{},platform.NewError(platform.CodeForbidden,false,"仅原拥有者可释放许可") }
 var result ReleasePermitResult
 found,err:=readGuardReceipt(t,"permit.release",in,&result)
 if err!=nil { return ReleasePermitResult{},err }
 if found { return result,nil }
 result.Released=row.state=="released"||row.state=="active"
 if row.state=="active" {
  _,err=t.tx.ExecContext(t.ctx,`UPDATE guard_permits SET state='released',revision=revision+1 WHERE admission_id=? AND site_id=? AND data_epoch=? AND state='active' AND revision=?`,row.admissionID,t.site,op.DataEpoch,row.revision)
  if err!=nil { return ReleasePermitResult{},loginDBError() }
 }
 if err=saveGuardReceipt(t,"permit.release",in,result);err!=nil { return ReleasePermitResult{},err }
 if err=t.commit();err!=nil { return ReleasePermitResult{},err }
 return result,nil
}

func readPermitRecord(t *guardTransaction,id string)(permitRecord,error) {
 var row permitRecord
 var consumed sql.NullString
 err:=t.tx.QueryRowContext(t.ctx,`SELECT admission_id,request_id,owner_caller,subject_key,session_key,network_key,action,state,input_hash,consumed_request_id,
  expires_at_ns,hard_deadline_ns,created_at_ns,revision,policy_revision FROM guard_permits WHERE admission_id=? AND site_id=? AND data_epoch=?`,id,t.site,t.op.DataEpoch).Scan(&row.admissionID,&row.requestID,&row.owner,&row.subject,&row.session,&row.network,&row.action,&row.state,&row.inputHash,&consumed,&row.expires,&row.hardDeadline,&row.created,&row.revision,&row.policyRevision)
 if errors.Is(err,sql.ErrNoRows) { return permitRecord{},platform.NewError(platform.CodeNotFound,false,"许可不存在") }
 if err!=nil { return permitRecord{},loginDBError() }
 if consumed.Valid { row.consumed=consumed.String }
 rows,err:=t.tx.QueryContext(t.ctx,`SELECT network_key FROM guard_permit_networks WHERE admission_id=? ORDER BY network_key`,id)
 if err!=nil { return permitRecord{},loginDBError() }
 defer rows.Close()
 row.networks=[]string{}
 for rows.Next() { var network string;if rows.Scan(&network)!=nil { return permitRecord{},loginDBError() };row.networks=append(row.networks,network) }
 if rows.Err()!=nil { return permitRecord{},loginDBError() }
 if len(row.networks)==0 { return permitRecord{},platform.NewError(platform.CodeIntegrityError,false,"许可网络作用域关联缺失") }
 return row,nil
}

func permitMetadata(row permitRecord,epoch string)platform.AdmissionPermit {
 return platform.AdmissionPermit{AdmissionID:row.admissionID,RequestID:row.requestID,SubjectID:row.subject,Action:row.action,Token:"",ExpiresAt:time.Unix(0,row.expires).UTC().Format(time.RFC3339Nano),SessionBudgetScope:row.session,NetworkBudgetScope:row.network,DataEpoch:epoch,Audience:"model-gateway"}
}
// CheckBan 只查已经验证的current/previous网络scope，不把换cookie当成换网络。
func CheckBan(t *guardTransaction,networks []string)(bool,int64,error) {
 if len(networks)==0 { return false,0,platform.NewError(platform.CodeDependencyUnavailable,true,"可信网络作用域不可用") }
 var latest int64
 for _,network:=range networks {
  var until sql.NullInt64
  if t.tx.QueryRowContext(t.ctx,`SELECT MAX(expires_at_ns) FROM guard_bans WHERE site_id=? AND data_epoch=? AND network_key=? AND scope='ai' AND state='active' AND starts_at_ns<=? AND expires_at_ns>?`,t.site,t.op.DataEpoch,network,t.now.UnixNano(),t.now.UnixNano()).Scan(&until)!=nil { return false,0,loginDBError() }
  if until.Valid&&until.Int64>latest { latest=until.Int64 }
 }
 return latest>0,latest,nil
}
