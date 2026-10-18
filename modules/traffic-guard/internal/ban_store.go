package trafficguard

import (
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "time"

 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/serviceauth"
)

type CreateBanInput struct {
 SubjectRef string `json:"subjectRef"`
 Scope string `json:"scope"`
 TTLSeconds int64 `json:"ttlSeconds"`
 ReasonCode string `json:"reasonCode"`
 DataEpoch string `json:"-"`
}
type CreateBanResult struct { BanID string `json:"banId"`; Revision int64 `json:"revision"` }
type RevokeBanInput struct { BanID string; ExpectedRevision int64; DataEpoch string }
type RevokeBanResult struct { Revision int64 `json:"revision"`; State string `json:"state"` }
type ListBansInput struct { Cursor string; Limit int; DataEpoch string }
type BanRecord struct {
 BanID string `json:"banId"`
 SubjectRef string `json:"subjectRef"`
 Scope string `json:"scope"`
 ReasonCode string `json:"reasonCode"`
 StartsAt string `json:"startsAt"`
 ExpiresAt string `json:"expiresAt"`
 State string `json:"state"`
 Revision int64 `json:"revision"`
}
type ListBansResult struct {
 BanIDs []string `json:"banIds"`
 Items []BanRecord `json:"items"`
 NextCursor string `json:"nextCursor"`
}

// CreateBan 只封匿名网络的AI功能，不接收原IP，也不触碰费用账本。
func CreateBan(ctx context.Context,op OperationContext,in CreateBanInput)(CreateBanResult,error) {
 if err:=authorizeGuardManagement(ctx,op,"createBan");err!=nil { return CreateBanResult{},err }
 t,err:=beginGuardTransaction(ctx,op,true)
 if err!=nil { return CreateBanResult{},err }
 defer t.close()
 var result CreateBanResult
 if found,err:=readGuardReceipt(t,"ban.create",in,&result);err!=nil { return CreateBanResult{},err } else if found { return result,nil }
 var known int
 err=t.tx.QueryRowContext(t.ctx,`SELECT COUNT(*) FROM guard_network_aliases WHERE site_id=? AND data_epoch=? AND network_key=?`,t.site,op.DataEpoch,in.SubjectRef).Scan(&known)
 if err!=nil { return CreateBanResult{},loginDBError() }
 if known==0 { return CreateBanResult{},platform.NewError(platform.CodeNotFound,false,"匿名网络引用不存在") }
 ttl:=in.TTLSeconds
 if ttl==0 { ttl=600 }
 if in.Scope!="ai"||ttl<1||ttl>2592000||(in.ReasonCode!="manual"&&in.ReasonCode!="abuse") { return CreateBanResult{},platform.NewError(platform.CodeValidation,false,"INVALID_BAN") }
 result,err=createNetworkBan(t,in.SubjectRef,ttl,in.ReasonCode)
 if err!=nil { return CreateBanResult{},err }
 if err=saveGuardReceipt(t,"ban.create",in,result);err!=nil { return CreateBanResult{},err }
 if err=t.commit();err!=nil { return CreateBanResult{},err }
 return result,nil
}

// createNetworkBan 与租约撤销及outbox共享调用方事务，模型已发生费用不倒退。
func createNetworkBan(t *guardTransaction,network string,ttl int64,reason string)(CreateBanResult,error) {
 result:=CreateBanResult{BanID:platform.NewIdentifier(),Revision:1}
 _,err:=t.tx.ExecContext(t.ctx,`INSERT INTO guard_bans(ban_id,site_id,data_epoch,network_key,scope,reason_code,starts_at_ns,expires_at_ns,state,revision)
  VALUES(?,?,?,?,'ai',?,?,?,'active',1)`,result.BanID,t.site,t.op.DataEpoch,network,reason,t.now.UnixNano(),t.now.Add(time.Duration(ttl)*time.Second).UnixNano())
 if err!=nil { return CreateBanResult{},loginDBError() }
 _,err=t.tx.ExecContext(t.ctx,`UPDATE guard_permits SET state='revoked',revision=revision+1 WHERE site_id=? AND data_epoch=? AND state='active'
  AND admission_id IN (SELECT admission_id FROM guard_permit_networks WHERE network_key=?)`,t.site,t.op.DataEpoch,network)
 if err!=nil { return CreateBanResult{},loginDBError() }
 if err=enqueueGuardEvent(t,"traffic-guard.ban-changed.v1",result.BanID,1,map[string]any{"banId":result.BanID,"banRevision":int64(1),"state":"active"});err!=nil { return CreateBanResult{},err }
 return result,nil
}

func RevokeBan(ctx context.Context,op OperationContext,in RevokeBanInput)(RevokeBanResult,error) {
 if err:=authorizeGuardManagement(ctx,op,"revokeBan");err!=nil { return RevokeBanResult{},err }
 t,err:=beginGuardTransaction(ctx,op,true)
 if err!=nil { return RevokeBanResult{},err }
 defer t.close()
 var result RevokeBanResult
 if found,err:=readGuardReceipt(t,"ban.revoke",in,&result);err!=nil { return RevokeBanResult{},err } else if found { return result,nil }
 var state string
 var revision int64
 err=t.tx.QueryRowContext(t.ctx,`SELECT state,revision FROM guard_bans WHERE ban_id=? AND site_id=? AND data_epoch=?`,in.BanID,t.site,op.DataEpoch).Scan(&state,&revision)
 if errors.Is(err,sql.ErrNoRows) { return RevokeBanResult{},platform.NewError(platform.CodeNotFound,false,"禁令不存在") }
 if err!=nil { return RevokeBanResult{},loginDBError() }
 if revision!=in.ExpectedRevision||state!="active"||revision==9223372036854775807 { return RevokeBanResult{},platform.NewError(platform.CodeVersionMismatch,false,"禁令状态或修订已变化") }
 result=RevokeBanResult{Revision:revision+1,State:"revoked"}
 _,err=t.tx.ExecContext(t.ctx,`UPDATE guard_bans SET state='revoked',revision=? WHERE ban_id=? AND site_id=? AND data_epoch=? AND revision=?`,result.Revision,in.BanID,t.site,op.DataEpoch,revision)
 if err!=nil { return RevokeBanResult{},loginDBError() }
 if err=saveGuardReceipt(t,"ban.revoke",in,result);err!=nil { return RevokeBanResult{},err }
 if err=enqueueGuardEvent(t,"traffic-guard.ban-changed.v1",in.BanID,result.Revision,map[string]any{"banId":in.BanID,"banRevision":result.Revision,"state":result.State});err!=nil { return RevokeBanResult{},err }
 if err=t.commit();err!=nil { return RevokeBanResult{},err }
 return result,nil
}

func ListBans(ctx context.Context,op OperationContext,in ListBansInput)(ListBansResult,error) {
 if err:=authorizeGuardManagement(ctx,op,"listBans");err!=nil { return ListBansResult{},err }
 t,err:=beginGuardTransaction(ctx,op,false)
 if err!=nil { return ListBansResult{},err }
 defer t.close()
 limit:=in.Limit
 if limit==0 { limit=20 }
 if limit<1||limit>100 { return ListBansResult{},platform.NewError(platform.CodeValidation,false,"INVALID_LIMIT") }
 scope:="guard.bans:"+t.site+":"+op.DataEpoch+":"+op.Actor.SubjectID
 after:=""
 if in.Cursor!="" {
  raw,err:=serviceauth.VerifyCursor(t.runtime.SigningKey,scope,in.Cursor)
  if err!=nil { return ListBansResult{},err }
  if json.Unmarshal(raw,&after)!=nil { return ListBansResult{},platform.NewError(platform.CodeValidation,false,"INVALID_CURSOR") }
 }
 rows,err:=t.tx.QueryContext(t.ctx,`SELECT ban_id,network_key,scope,reason_code,starts_at_ns,expires_at_ns,state,revision FROM guard_bans
  WHERE site_id=? AND data_epoch=? AND ban_id>? ORDER BY ban_id LIMIT ?`,t.site,op.DataEpoch,after,limit+1)
 if err!=nil { return ListBansResult{},loginDBError() }
 defer rows.Close()
 items:=make([]BanRecord,0,limit+1)
 for rows.Next() {
  var item BanRecord
  var starts,expires int64
  if rows.Scan(&item.BanID,&item.SubjectRef,&item.Scope,&item.ReasonCode,&starts,&expires,&item.State,&item.Revision)!=nil { return ListBansResult{},loginDBError() }
  item.StartsAt=time.Unix(0,starts).UTC().Format(time.RFC3339Nano)
  item.ExpiresAt=time.Unix(0,expires).UTC().Format(time.RFC3339Nano)
  items=append(items,item)
 }
 if rows.Err()!=nil { return ListBansResult{},loginDBError() }
 result:=ListBansResult{BanIDs:[]string{},Items:items,NextCursor:""}
 if len(items)>limit {
  result.Items=items[:limit]
  raw,_:=json.Marshal(result.Items[limit-1].BanID)
  result.NextCursor,err=serviceauth.SignCursor(t.runtime.SigningKey,scope,raw)
  if err!=nil { return ListBansResult{},err }
 }
 for _,item:=range result.Items { result.BanIDs=append(result.BanIDs,item.BanID) }
 return result,nil
}
