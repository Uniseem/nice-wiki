package glossary

import (
 "context"
 "database/sql"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/language"
)

// HandleGlossaryChange 与术语提交同事务失效机翻；保护文本仅置review，不重新计费。
func HandleGlossaryChange(ctx context.Context,tx *sql.Tx,op OperationContext,source,target string,generation int)(int,error){
 result,err:=tx.ExecContext(ctx,`UPDATE translation_manifests SET state='stale',updated_at=? WHERE target_language=? AND data_epoch=? AND glossary_generation<? AND state IN('ready','review') AND EXISTS(SELECT 1 FROM translation_units u WHERE u.translation_id=translation_manifests.translation_id AND u.origin='machine' AND u.protected=0)`,platform.Now(),target,op.DataEpoch,generation);if err!=nil{return 0,language.StorageError(err)}
 count,err:=result.RowsAffected();if err!=nil{return 0,language.StorageError(err)}
 _,err=tx.ExecContext(ctx,`UPDATE translation_units SET status='review',updated_at=? WHERE target_language=? AND data_epoch=? AND (protected=1 OR origin='human')`,platform.Now(),target,op.DataEpoch);return int(count),language.StorageError(err)
}
