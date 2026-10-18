package outbox

import (
 "context"
 "nice-wiki/libs/platform"
)

// SQL触发器覆盖所有行变更，包括未来重试/leased/quarantine；不依赖轮询due子集。
const observationSchemaSQL = `CREATE TABLE IF NOT EXISTS nw_outbox_control(id INTEGER PRIMARY KEY CHECK(id=1), revision INTEGER NOT NULL CHECK(revision>0), sealed_hash TEXT NOT NULL DEFAULT '', last_seal_hash TEXT NOT NULL DEFAULT '');
INSERT OR IGNORE INTO nw_outbox_control(id,revision,sealed_hash) VALUES(1,1,'');
CREATE TRIGGER IF NOT EXISTS nw_outbox_gate_insert BEFORE INSERT ON nw_outbox WHEN (SELECT sealed_hash FROM nw_outbox_control WHERE id=1)<>'' BEGIN SELECT RAISE(ABORT,'outbox sealed'); END;
CREATE TRIGGER IF NOT EXISTS nw_outbox_gate_update BEFORE UPDATE ON nw_outbox WHEN (SELECT sealed_hash FROM nw_outbox_control WHERE id=1)<>'' BEGIN SELECT RAISE(ABORT,'outbox sealed'); END;
CREATE TRIGGER IF NOT EXISTS nw_outbox_gate_delete BEFORE DELETE ON nw_outbox WHEN (SELECT sealed_hash FROM nw_outbox_control WHERE id=1)<>'' BEGIN SELECT RAISE(ABORT,'outbox sealed'); END;
CREATE TRIGGER IF NOT EXISTS nw_outbox_revision_insert AFTER INSERT ON nw_outbox BEGIN UPDATE nw_outbox_control SET revision=revision+1 WHERE id=1; END;
CREATE TRIGGER IF NOT EXISTS nw_outbox_revision_update AFTER UPDATE ON nw_outbox BEGIN UPDATE nw_outbox_control SET revision=revision+1 WHERE id=1; END;
CREATE TRIGGER IF NOT EXISTS nw_outbox_revision_delete AFTER DELETE ON nw_outbox BEGIN UPDATE nw_outbox_control SET revision=revision+1 WHERE id=1; END;`

type Observation struct{Revision int64;Pending int64;Leased int64;Quarantined int64;Isolated int64;SealHash string}
func(s *Store)Observe(ctx context.Context)(Observation,error){
 var result Observation;var unknown int64
 err:=s.db.QueryRowContext(ctx,`SELECT c.revision,c.sealed_hash,COALESCE(SUM(CASE WHEN o.state NOT IN ('relayed','discarded') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN o.lease_until<>'' AND o.state NOT IN ('relayed','discarded') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN o.state='quarantined' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN o.state='discarded' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN o.state NOT IN ('pending','relayed','quarantined','discarded') THEN 1 ELSE 0 END),0) FROM nw_outbox_control c LEFT JOIN nw_outbox o ON 1=1 WHERE c.id=1 GROUP BY c.revision,c.sealed_hash`).Scan(&result.Revision,&result.SealHash,&result.Pending,&result.Leased,&result.Quarantined,&result.Isolated,&unknown)
 if err!=nil{return result,platform.NewError(platform.CodeDependencyUnavailable,true,"真实outbox水位不可读取")};if result.Revision<1||unknown!=0{return result,platform.NewError(platform.CodeIntegrityError,false,"outbox水位或行活动状态未知")};return result,nil
}
// Seal在与业务相同SQLite中原子CAS；触发器随后禁止任何插入/更新/删除，不靠一次抽样0。
func(s *Store)Seal(ctx context.Context,revision int64,proofHash string)error{
 if revision<1||!platform.ValidHash(proofHash){return platform.NewError(platform.CodeValidation,false,"outbox封口证明不完整")};result,err:=s.db.ExecContext(ctx,`UPDATE nw_outbox_control SET sealed_hash=?,last_seal_hash=? WHERE id=1 AND revision=? AND (sealed_hash='' OR sealed_hash=?) AND NOT EXISTS(SELECT 1 FROM nw_outbox WHERE state NOT IN ('relayed','discarded'))`,proofHash,proofHash,revision,proofHash);if err!=nil{return platform.NewError(platform.CodeInternal,true,"outbox事务封口失败")};count,err:=result.RowsAffected();if err!=nil||count!=1{return platform.NewError(platform.CodeConflict,true,"outbox水位改变或尚有活动行")};return nil
}
func(s *Store)Resume(ctx context.Context,proofHash string)error{if !platform.ValidHash(proofHash){return platform.NewError(platform.CodeValidation,false,"缺原outbox封口摘要")};result,err:=s.db.ExecContext(ctx,`UPDATE nw_outbox_control SET sealed_hash='' WHERE id=1 AND (sealed_hash=? OR (sealed_hash='' AND last_seal_hash=?))`,proofHash,proofHash);if err!=nil{return platform.NewError(platform.CodeInternal,true,"outbox重开旧清理失败")};count,err:=result.RowsAffected();if err!=nil||count!=1{return platform.NewError(platform.CodeConflict,false,"原outbox封口绑定不符")};return nil}
