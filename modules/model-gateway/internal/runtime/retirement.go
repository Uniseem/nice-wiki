package runtime

import (
 "context"
 "database/sql"
 "nice-wiki/libs/platform"
)

// DestroyRetiredSecrets只销毁有完整引用覆盖的旧密文；版本、引用和销毁时刻审计保留。
func DestroyRetiredSecrets(ctx context.Context,op OperationContext,tx *sql.Tx)error{
 var legacy,enabled bool
 if err:=tx.QueryRowContext(ctx,`SELECT enabled FROM credential_tracking WHERE singleton=1`).Scan(&enabled);err!=nil{return ExecutionStorageError(err)}
 if !enabled{return nil}
 if err:=tx.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM invocations WHERE execution_hash IS NULL OR execution_hash='')`).Scan(&legacy);err!=nil{return ExecutionStorageError(err)}
 if legacy{return nil} // 旧行不能从“没有引用”推断不会再消费，保持密文等待明确历史迁移。
 now:=platform.Now()
 if _,err:=tx.ExecContext(ctx,`INSERT INTO credential_retirements(provider_id,credential_version,data_epoch,destroyed_at) SELECT c.provider_id,c.credential_version,c.data_epoch,? FROM credentials AS c WHERE c.data_epoch=? AND EXISTS(SELECT 1 FROM providers p WHERE p.provider_id=c.provider_id AND p.data_epoch=c.data_epoch AND p.credential_version<>c.credential_version) AND NOT EXISTS(SELECT 1 FROM route_credential_refs r WHERE r.provider_id=c.provider_id AND r.credential_version=c.credential_version AND r.data_epoch=c.data_epoch AND r.released=0) AND NOT EXISTS(SELECT 1 FROM credential_references r WHERE r.provider_id=c.provider_id AND r.credential_version=c.credential_version AND r.data_epoch=c.data_epoch AND r.released=0)`,now,op.DataEpoch);err!=nil{return ExecutionStorageError(err)}
 if _,err:=tx.ExecContext(ctx,`DELETE FROM credentials WHERE data_epoch=? AND EXISTS(SELECT 1 FROM credential_retirements r WHERE r.provider_id=credentials.provider_id AND r.credential_version=credentials.credential_version AND r.data_epoch=credentials.data_epoch)`,op.DataEpoch);err!=nil{return ExecutionStorageError(err)}
 if _,err:=tx.ExecContext(ctx,`UPDATE header_secrets AS h SET nonce=X'',ciphertext=X'',destroyed_at=? WHERE h.data_epoch=? AND h.destroyed_at IS NULL AND NOT EXISTS(SELECT 1 FROM route_header_refs r WHERE r.ref_id=h.ref_id AND r.released=0) AND NOT EXISTS(SELECT 1 FROM providers p,json_each(p.extra_header_secret_refs_json) j WHERE p.provider_id=h.provider_id AND p.data_epoch=h.data_epoch AND j.value=h.ref_id)`,now,op.DataEpoch);err!=nil{return ExecutionStorageError(err)}
 return nil
}
