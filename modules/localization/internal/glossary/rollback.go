package glossary

import (
 "context"
 "database/sql"
 "sort"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/language"
)

type RollbackAutomaticGlossaryBatchInput struct{BatchId string `json:"batchId"`;ExpectedGeneration int `json:"expectedGeneration"`;IdempotencyKey string `json:"idempotencyKey"`}
type RollbackAutomaticGlossaryBatchResult struct{Removed int `json:"removed"`;Generation int `json:"generation"`}
func RollbackAutomaticGlossaryBatch(ctx context.Context,op OperationContext,input RollbackAutomaticGlossaryBatchInput)(RollbackAutomaticGlossaryBatchResult,error){
 if err:=language.RequireAdministrator(op);err!=nil{return RollbackAutomaticGlossaryBatchResult{},err};var result RollbackAutomaticGlossaryBatchResult
 err:=language.Mutate(ctx,op,"localization.RollbackAutomaticGlossaryBatch",input.IdempotencyKey,input,&result,func(tx *sql.Tx)error{
  g,err:=Generation(ctx,tx);if err!=nil{return err};if g!=input.ExpectedGeneration{return platform.NewError(platform.CodeVersionMismatch,false,"术语代数已变化")}
  rows,err:=tx.QueryContext(ctx,`SELECT term_id,source_language,target_language FROM glossary WHERE extraction_batch_id=? AND source='auto' AND deleted=0 AND data_epoch=? ORDER BY term_id`,input.BatchId,op.DataEpoch);if err!=nil{return language.StorageError(err)}
  type pair struct{source,target string};groups:=map[pair][]string{};ids:=[]string{}
  for rows.Next(){var id,a,b string;if err=rows.Scan(&id,&a,&b);err!=nil{rows.Close();return language.StorageError(err)};ids=append(ids,id);groups[pair{a,b}]=append(groups[pair{a,b}],id)};err=rows.Err();rows.Close();if err!=nil{return language.StorageError(err)}
  if len(ids)>256{return platform.NewError(platform.CodeIntegrityError,false,"自动术语批次超过已登记上限")};if len(ids)==0{result.Generation=g;return nil}
  result.Generation,err=AdvanceGeneration(ctx,tx);if err!=nil{return err}
  res,err:=tx.ExecContext(ctx,`UPDATE glossary SET deleted=1,version=version+1,generation=?,updated_at=? WHERE extraction_batch_id=? AND source='auto' AND deleted=0 AND data_epoch=?`,result.Generation,platform.Now(),input.BatchId,op.DataEpoch);if err!=nil{return language.StorageError(err)};removed,_:=res.RowsAffected();result.Removed=int(removed)
  pairs:=[]pair{};for p:=range groups{pairs=append(pairs,p)};sort.Slice(pairs,func(i,j int)bool{if pairs[i].source!=pairs[j].source{return pairs[i].source<pairs[j].source};return pairs[i].target<pairs[j].target})
  for _,p:=range pairs{if err=PublishChange(ctx,tx,op,p.source,p.target,result.Generation,groups[p],"auto");err!=nil{return err}};return nil
 });return result,err
}
