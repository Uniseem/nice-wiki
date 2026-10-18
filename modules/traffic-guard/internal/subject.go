package trafficguard

import (
 "context"
 "database/sql"
 "errors"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/serviceauth"
)

type DeriveSubjectInput struct { VisitorToken string;SiteID string;KeyEpoch string }
type DeriveSubjectResult struct { SubjectKey string;NetworkKey string;SessionKey string;SessionID string }

// DeriveSubject 不重新解析代理链；只用公共入口已签名的30日网络预算作用域。
func DeriveSubject(ctx context.Context,op OperationContext,in DeriveSubjectInput)(DeriveSubjectResult,error) {
 runtime,ok:=ctx.Value(guardRuntimeKey{}).(GuardRuntime)
 if !ok||runtime.BudgetKey.KeyID==""||len(op.NetworkBudgetScopes)==0 { return DeriveSubjectResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"可信网络或独立预算key未装配") }
 if op.CallerService!="portal"||in.SiteID!=op.Actor.SiteID||in.KeyEpoch!=runtime.BudgetKey.KeyID { return DeriveSubjectResult{},platform.NewError(platform.CodeForbidden,false,"匿名作用域只接受可信入口") }
 sessionID:=""
 material:="anonymous:"+op.NetworkBudgetScopes[0]
 expectedSubject:=""
 if in.VisitorToken!="" {
  claims,err:=serviceauth.VerifyGuardSession(runtime.Anchors,in.VisitorToken)
  if err!=nil { return DeriveSubjectResult{},err }
  if claims.SiteID!=op.Actor.SiteID||claims.DataEpoch!=op.DataEpoch { return DeriveSubjectResult{},platform.NewError(platform.CodeVersionMismatch,false,"访客证明不属于当前站点代次") }
  sessionID=claims.SessionID
  material=sessionID
  expectedSubject=claims.SubjectID
 }
 subject,session,err:=deriveSessionScopes(runtime,op.Actor.SiteID,material,expectedSubject)
 if err!=nil { return DeriveSubjectResult{},err }
 t,err:=beginGuardTransaction(ctx,op,true)
 if err!=nil { return DeriveSubjectResult{},err }
 defer t.close()
 if sessionID!="" {
  var stored string
  err=t.tx.QueryRowContext(t.ctx,`SELECT session_key FROM guard_subjects WHERE site_id=? AND data_epoch=? AND subject_key=?`,t.site,op.DataEpoch,subject).Scan(&stored)
  if errors.Is(err,sql.ErrNoRows) { return DeriveSubjectResult{},platform.NewError(platform.CodeUnauthenticated,false,"访客证明的本域状态不存在，请重新验证") }
  if err!=nil { return DeriveSubjectResult{},loginDBError() }
  if stored!=session { return DeriveSubjectResult{},platform.NewError(platform.CodeIntegrityError,false,"匿名会话scope与持久身份不一致") }
 }
 _,err=t.tx.ExecContext(t.ctx,`INSERT INTO guard_subjects(site_id,data_epoch,subject_key,session_key,network_key,updated_at_ns)
  VALUES(?,?,?,?,?,?) ON CONFLICT(site_id,data_epoch,subject_key) DO UPDATE SET network_key=excluded.network_key,updated_at_ns=excluded.updated_at_ns`,t.site,op.DataEpoch,subject,session,op.NetworkBudgetScopes[0],t.now.UnixNano())
 if err!=nil { return DeriveSubjectResult{},loginDBError() }
 _,err=t.tx.ExecContext(t.ctx,`DELETE FROM guard_network_aliases WHERE site_id=? AND data_epoch=? AND subject_key=?`,t.site,op.DataEpoch,subject)
 if err!=nil { return DeriveSubjectResult{},loginDBError() }
 seen:=map[string]bool{}
 for _,network:=range op.NetworkBudgetScopes {
  if seen[network] { continue };seen[network]=true
  _,err=t.tx.ExecContext(t.ctx,`INSERT INTO guard_network_aliases(site_id,data_epoch,subject_key,network_key,last_seen_at_ns) VALUES(?,?,?,?,?)`,t.site,op.DataEpoch,subject,network,t.now.UnixNano())
  if err!=nil { return DeriveSubjectResult{},loginDBError() }
 }
 if err=t.commit();err!=nil { return DeriveSubjectResult{},err }
 return DeriveSubjectResult{SubjectKey:subject,NetworkKey:op.NetworkBudgetScopes[0],SessionKey:session,SessionID:sessionID},nil
}

// deriveSessionScopes 保留前期预算key验证旧会话，绝不因服务签名key轮换重置预算身份。
func deriveSessionScopes(runtime GuardRuntime,site,sessionID,expectedSubject string)(string,string,error) {
 payload,_:=platform.CanonicalJSON(map[string]string{"siteId":site,"sessionId":sessionID})
 keys:=append([]platform.SigningKey{runtime.BudgetKey},runtime.PreviousBudgetKeys...)
 for _,key:=range keys {
  subject,err:=serviceauth.PrivateOperationDigest(key,"guard.subject.v1",payload)
  if err!=nil { return "","",err }
  if expectedSubject!=""&&subject!=expectedSubject { continue }
  session,err:=serviceauth.PrivateOperationDigest(key,"guard.session.v1",payload)
  if err!=nil { return "","",err }
  return subject,session,nil
 }
 return "","",platform.NewError(platform.CodeVersionMismatch,false,"旧会话预算key过渡材料未保留")
}
