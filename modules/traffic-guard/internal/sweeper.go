package trafficguard

import (
 "context"
 "time"
 "nice-wiki/libs/platform"
)

type ExpireLeasesInput struct { NowMs int64; BatchSize int }
type ExpireLeasesResult struct { ExpiredCount int64 `json:"expiredCount"` }
type ExpireBansInput struct { NowMs int64; BatchSize int }
type ExpireBansResult struct { ExpiredCount int64 `json:"expiredCount"` }

// ExpireLeases 只终结已到期租约；不退回频率，也不重发任何外部调用。
func ExpireLeases(ctx context.Context,op OperationContext,in ExpireLeasesInput)(ExpireLeasesResult,error) {
 t,err:=beginGuardTransaction(ctx,op,true)
 if err!=nil { return ExpireLeasesResult{},err }
 defer t.close()
 changed,err:=t.tx.ExecContext(t.ctx,`UPDATE guard_permits SET state='expired',revision=revision+1
  WHERE admission_id IN (SELECT admission_id FROM guard_permits WHERE site_id=? AND data_epoch=? AND state='active'
  AND expires_at_ns<=? ORDER BY expires_at_ns,admission_id LIMIT ?)`,t.site,op.DataEpoch,time.UnixMilli(in.NowMs).UnixNano(),in.BatchSize)
 if err!=nil { return ExpireLeasesResult{},loginDBError() }
 count,err:=changed.RowsAffected()
 if err!=nil { return ExpireLeasesResult{},loginDBError() }
 if err=t.commit();err!=nil { return ExpireLeasesResult{},err }
 return ExpireLeasesResult{count},nil
}

// ExpireBans 在同一批事务中改变状态并写安全引用outbox。
func ExpireBans(ctx context.Context,op OperationContext,in ExpireBansInput)(ExpireBansResult,error) {
 t,err:=beginGuardTransaction(ctx,op,true)
 if err!=nil { return ExpireBansResult{},err }
 defer t.close()
 rows,err:=t.tx.QueryContext(t.ctx,`SELECT ban_id,revision FROM guard_bans WHERE site_id=? AND data_epoch=? AND state='active'
  AND expires_at_ns<=? ORDER BY expires_at_ns,ban_id LIMIT ?`,t.site,op.DataEpoch,time.UnixMilli(in.NowMs).UnixNano(),in.BatchSize)
 if err!=nil { return ExpireBansResult{},loginDBError() }
 type record struct { id string; revision int64 }
 records:=make([]record,0)
 for rows.Next() { var item record;if rows.Scan(&item.id,&item.revision)!=nil { _=rows.Close();return ExpireBansResult{},loginDBError() };records=append(records,item) }
 if rows.Err()!=nil { _=rows.Close();return ExpireBansResult{},loginDBError() }
 _=rows.Close()
 for _,item:=range records {
  if item.revision==9223372036854775807 { return ExpireBansResult{},platform.NewError(platform.CodeVersionMismatch,false,"禁令修订耗尽") }
  _,err=t.tx.ExecContext(t.ctx,`UPDATE guard_bans SET state='expired',revision=revision+1 WHERE ban_id=? AND site_id=? AND data_epoch=? AND state='active' AND revision=?`,item.id,t.site,op.DataEpoch,item.revision)
  if err!=nil { return ExpireBansResult{},loginDBError() }
  if err=enqueueGuardEvent(t,"traffic-guard.ban-changed.v1",item.id,item.revision+1,map[string]any{"banId":item.id,"banRevision":item.revision+1,"state":"expired"});err!=nil { return ExpireBansResult{},err }
 }
 if err=t.commit();err!=nil { return ExpireBansResult{},err }
 return ExpireBansResult{int64(len(records))},nil
}
