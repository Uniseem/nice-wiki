package extract

import (
 "context"
 "database/sql"
 "encoding/json"
 "nice-wiki/libs/platform"
 indexer "nice-wiki/modules/knowledge/internal/index"
 modelruntime "nice-wiki/modules/knowledge/internal/model"
 "nice-wiki/modules/knowledge/internal/source"
)

type FailedExtractionResolution struct { PreviousID string; Replace bool; Blocked bool; Artifact *ExtractionArtifact; ModelStepKey string; ModelResultHash string }

// InspectFailedExtraction 在新明确批准构建关联原意图之前锁外免费观察；同构建永远保持原owner。
func InspectFailedExtraction(ctx context.Context,op OperationContext,buildID,intentKey string,origin source.AssetSource,plan ExtractionPlan)(FailedExtractionResolution,error) {
 result:=FailedExtractionResolution{}
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return result,err}
 var state,owner string;var sourceRaw,planRaw []byte
 err=runtime.DB.QueryRowContext(ctx,`SELECT e.extraction_id,e.state,e.owner_build_id,e.source_json,e.plan_json FROM extraction_cache_intents c JOIN extractions e ON e.extraction_id=c.extraction_id WHERE c.site_id=? AND c.data_epoch=? AND c.cache_key=?`,op.Actor.SiteID,op.DataEpoch,intentKey).Scan(&result.PreviousID,&state,&owner,&sourceRaw,&planRaw)
 if err==sql.ErrNoRows {
  // 旧计划hash意图迁移为稳定资源语义时保留最早真实owner，不能以记忆/模型变化绕过旧unknown。
  err=runtime.DB.QueryRowContext(ctx,`SELECT extraction_id,state,owner_build_id,source_json,plan_json FROM extractions WHERE site_id=? AND data_epoch=? AND json_extract(source_json,'$.asset.sha256')=? AND json_extract(source_json,'$.asset.mediaType')=? AND json_extract(source_json,'$.asset.byteSize')=? AND json_extract(plan_json,'$.taskId')=? AND json_extract(plan_json,'$.extractorVersion')=? ORDER BY created_at,extraction_id LIMIT 1`,op.Actor.SiteID,op.DataEpoch,origin.Asset.SHA256,origin.Asset.MediaType,origin.Asset.ByteSize,plan.TaskID,plan.ExtractorVersion).Scan(&result.PreviousID,&state,&owner,&sourceRaw,&planRaw)
 }
 if err==sql.ErrNoRows {return result,nil};if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原失败提取意图不可读")}
 if owner==buildID {return result,nil}
 var payload ExtractionJobInput;payload.ExtractionID=result.PreviousID
 if json.Unmarshal(sourceRaw,&payload.Source)!=nil||json.Unmarshal(planRaw,&payload.Plan)!=nil {return result,platform.NewError(platform.CodeIntegrityError,false,"原失败提取来源或计划损坏")}
 originalRead,err:=runtime.DB.BeginTx(ctx,&sql.TxOptions{ReadOnly:true});if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原提取批准输入不可读")}
 frozen,_,_,_,_,err:=indexer.ApprovedJobInput(ctx,originalRead,"knowledge.extract",result.PreviousID);_=originalRead.Rollback();if err!=nil {return result,err}
 var approvedPayload ExtractionJobInput
 if json.Unmarshal(frozen.Input,&approvedPayload)!=nil {return result,platform.NewError(platform.CodeIntegrityError,false,"原提取批准完整输入损坏")};expectedPayload,_:=platform.CanonicalJSON(payload);actualPayload,_:=platform.CanonicalJSON(approvedPayload)
 if string(expectedPayload)!=string(actualPayload) {return result,platform.NewError(platform.CodeIntegrityError,false,"原提取计划不匹配批准输入hash")}

 expected,_:=platform.CanonicalJSON(plan);samePlan:=string(expected)==string(planRaw);result.Blocked=!samePlan
 if state!="failed"&&((state!="ready"&&state!="unsupported")||samePlan) {return result,nil}
 if plan.TaskID!="" {
  if err=modelruntime.ObserveOwnerModelSteps(ctx,op,result.PreviousID);err!=nil {return result,err}
  var raw []byte;var hash,key string
  err=runtime.DB.QueryRowContext(ctx,`SELECT result_json,result_hash,step_key FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND semantic_key=?`,op.Actor.SiteID,op.DataEpoch,result.PreviousID,"resource-description."+payload.Source.Asset.SHA256).Scan(&raw,&hash,&key)
  if err!=nil&&err!=sql.ErrNoRows {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原paid提取结果不可读")}
  if err==nil&&len(raw)>0 {
   var outcome modelruntime.InvokeModelResult
   if platform.HashBytes(raw)!=hash||json.Unmarshal(raw,&outcome)!=nil {return result,platform.NewError(platform.CodeIntegrityError,false,"原paid提取完整结果损坏")}
   if modelruntime.HasCompleteResult(outcome) {
    snapshot,sourceErr:=modelruntime.WinningModelSnapshot(outcome);text,textErr:=DescriptionText(outcome)
    eligible:=false;for _,current:=range plan.ModelSnapshots {if current==snapshot {eligible=true;break}}
    if sourceErr==nil&&textErr==nil&&eligible&&payload.Plan.TaskID==plan.TaskID&&payload.Plan.Memory==plan.Memory&&payload.Plan.ExtractorVersion==plan.ExtractorVersion {
     asset:=platform.AssetRef{AssetID:payload.Source.Asset.AssetID,SHA256:payload.Source.Asset.SHA256,MediaType:payload.Source.Asset.MediaType,ByteSize:payload.Source.Asset.ByteSize};contentHash:=platform.HashBytes([]byte(text))
     artifact:=ExtractionArtifact{Extraction:Extraction{ExtractionID:result.PreviousID,Asset:&asset,SourceBlock:&payload.Source.Block,ExtractorVersion:payload.Plan.ExtractorVersion,State:"ready",Text:&text,ContentHash:&contentHash,ModelSnapshotHash:&snapshot},Pages:[]PDFPage{}}
     result.Artifact=&artifact;result.ModelStepKey=key;result.ModelResultHash=hash;return result,nil
    }
   }
  }
 }
 // 本地failed只是一轮失败。只有jobs真正终局且无可用完整paid结果，才核新owner的额外费用证明。
 jobState,err:=ReadExtractionJobState(ctx,op,result.PreviousID);if err!=nil {return result,err}
 if jobState!="dead-letter"&&jobState!="cancelled"&&jobState!="succeeded" {return result,nil}
 read,err:=runtime.DB.BeginTx(ctx,&sql.TxOptions{ReadOnly:true});if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原提取费用来源不可读")}
 original,_,_,_,_,err:=indexer.ApprovedJobInput(ctx,read,"knowledge.extract",result.PreviousID)
 if err!=nil {read.Rollback();return result,err}
 var missing int
 err=read.QueryRowContext(ctx,`SELECT count(*) FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND dispatch_started=1 AND (invocation_id IS NULL OR invocation_id='')`,op.Actor.SiteID,op.DataEpoch,result.PreviousID).Scan(&missing);_=read.Rollback()
 if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原提取外发接受证明不可读")};if missing!=0 {return result,nil} // 专用时点未接受也不允许新owner绕过可能晚接受的原RPC。
 if plan.TaskID!="" {final,err:=modelruntime.RootFeesFinal(ctx,op,original.Operation.RootOperationID);if err!=nil {return result,err};if !final {return result,nil}}
 result.Replace=true;result.Blocked=false;return result,nil
}

// SaveRecoveredExtraction 只保存真实原paid输出的免费派生结果，保留失败任务/原费用/原执行证明，不伪造jobs完成回执。
func SaveRecoveredExtraction(ctx context.Context,tx *sql.Tx,op OperationContext,resolution FailedExtractionResolution)error {
 if resolution.Artifact==nil {return nil}
 raw,err:=platform.CanonicalJSON(resolution.Artifact);if err!=nil {return err}
 var original []byte;var hash string
 err=tx.QueryRowContext(ctx,`SELECT result_json,result_hash FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND step_key=?`,op.Actor.SiteID,op.DataEpoch,resolution.PreviousID,resolution.ModelStepKey).Scan(&original,&hash)
 if err!=nil {return platform.NormalizeIOError(err,platform.CodeInternal,false,"原paid免费恢复来源不可读")}
 if hash!=resolution.ModelResultHash||platform.HashBytes(original)!=hash {return platform.NewError(platform.CodeStaleResult,false,"免费恢复期间原paid输出变化")}
 _,err=tx.ExecContext(ctx,`INSERT INTO extraction_recovered_results(extraction_id,artifact_json,artifact_hash,model_step_key,model_result_hash,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(extraction_id) DO NOTHING`,resolution.PreviousID,raw,platform.HashBytes(raw),resolution.ModelStepKey,hash,platform.Now())
 if err!=nil {return platform.NormalizeIOError(err,platform.CodeInternal,false,"原paid免费恢复结果保存失败")}
 var saved []byte;var savedHash string
 if err=tx.QueryRowContext(ctx,`SELECT artifact_json,artifact_hash FROM extraction_recovered_results WHERE extraction_id=?`,resolution.PreviousID).Scan(&saved,&savedHash);err!=nil {return platform.NormalizeIOError(err,platform.CodeInternal,false,"原paid免费恢复回执不可读")}
 if savedHash!=platform.HashBytes(raw)||savedHash!=platform.HashBytes(saved) {return platform.NewError(platform.CodeConflict,false,"原paid免费恢复已有不同结果")};return nil
}
