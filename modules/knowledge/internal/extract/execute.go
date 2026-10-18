package extract

import (
 "context"
 "encoding/json"
 "strings"
 "time"
 "nice-wiki/libs/platform"
 indexer "nice-wiki/modules/knowledge/internal/index"
 modelruntime "nice-wiki/modules/knowledge/internal/model"
)

type JobExecutionInput = platform.JobExecutionInput
type JobExecutionResult = platform.JobExecutionResult
type extractionPlanKey struct{}

// ExecuteExtraction 独立jobs租约只执行持久原输入，最终结果与拥有者回执同事务。
func ExecuteExtraction(ctx context.Context,op OperationContext,input JobExecutionInput)(result JobExecutionResult,failure error) {
 defer func(){if failure!=nil {result,failure=indexer.FinishExecutionFailure(ctx,op,input,failure)}}()
 record,err:=indexer.ReadExecutionRecord(ctx,op,input);if err!=nil {return JobExecutionResult{},err}
 if record.State=="completed" {return JobExecutionResult{State:platform.JobSucceeded,ResultRef:record.ResultRef,PaidResultRefs:record.PaidRefs},nil}
 var payload ExtractionJobInput
 if json.Unmarshal(record.Input,&payload)!=nil||payload.ExtractionID!=record.ObjectID||payload.Source.Block.Document.DataEpoch!=op.DataEpoch {return JobExecutionResult{},platform.NewError(platform.CodeIntegrityError,false,"原提取完整输入损坏")}
 if input.CheckLease==nil {return JobExecutionResult{},platform.NewError(platform.CodeDependencyUnavailable,false,"原提取缺权威租约检查")}
 if err=input.CheckLease(ctx);err!=nil {return JobExecutionResult{},err}
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return JobExecutionResult{},err}
 if runtime.Content==nil||runtime.ValidatePublicationResult==nil {return JobExecutionResult{},platform.NewError(platform.CodeDependencyUnavailable,false,"提取发布权威未装配")}
 if _,_,err=indexer.ValidatePublication(ctx,op,runtime.Content,runtime.ValidatePublicationResult,payload.Source.Block.Document);err!=nil {return JobExecutionResult{},err}
 if err=CheckExtractionPlan(ctx,op,payload);err!=nil {return JobExecutionResult{},err}
 ctx,err=modelruntime.WithModelLease(ctx,input.CheckLease);if err!=nil {return JobExecutionResult{},err}
 ctx=context.WithValue(ctx,extractionPlanKey{},payload.Plan)
 asset:=platform.AssetRef{AssetID:payload.Source.Asset.AssetID,SHA256:payload.Source.Asset.SHA256,MediaType:payload.Source.Asset.MediaType,ByteSize:payload.Source.Asset.ByteSize}
 leaseID:=indexer.ExecutionLeaseID(input.JobID,input.FencingToken)
 artifact:=ExtractionArtifact{Extraction:Extraction{ExtractionID:payload.ExtractionID,Asset:&asset,SourceBlock:&payload.Source.Block,ExtractorVersion:payload.Plan.ExtractorVersion,State:"unsupported"},Pages:[]PDFPage{}}
 switch asset.MediaType {
 case "application/pdf":value,err:=ExtractPdfText(ctx,op,ExtractPdfTextInput{ExtractionID:payload.ExtractionID,Asset:asset,Source:payload.Source,LeaseID:leaseID});if err!=nil {return JobExecutionResult{},err};artifact.Extraction=value.Extraction;artifact.Pages=value.Pages
 case "image/png","image/jpeg","image/webp","image/gif":value,err:=ExtractImageDescription(ctx,op,ExtractImageDescriptionInput{ExtractionID:payload.ExtractionID,Asset:asset,Source:payload.Source,Memory:payload.Plan.Memory,LeaseID:leaseID});if err!=nil {return JobExecutionResult{},err};artifact.Extraction=value.Extraction
 case "text/plain","text/markdown","text/csv","application/json":value,err:=DescribeAttachment(ctx,op,DescribeAttachmentInput{ExtractionID:payload.ExtractionID,Asset:asset,Source:payload.Source,Memory:payload.Plan.Memory,LeaseID:leaseID});if err!=nil {return JobExecutionResult{},err};artifact.Extraction=value.Extraction
 case "video/mp4","video/webm":
  var body []byte
  err=runtime.DB.QueryRowContext(ctx,`SELECT b.body_json FROM build_sources b JOIN extractions e ON e.owner_build_id=b.build_id WHERE e.extraction_id=? AND e.site_id=? AND e.data_epoch=?`,payload.ExtractionID,op.Actor.SiteID,op.DataEpoch).Scan(&body)
  if err!=nil {return JobExecutionResult{},platform.NormalizeIOError(err,platform.CodeNotFound,false,"原视频发布快照不存在")}
  value,err:=ExtractVideoMetadata(ctx,op,ExtractVideoMetadataInput{ExtractionID:payload.ExtractionID,Document:payload.Source.Block.Document,Source:string(body),BlockID:payload.Source.Block.BlockId});if err!=nil {return JobExecutionResult{},err};artifact.Extraction=value.Extraction

 // 已确认无字幕的视频与未声明格式不自动转写、OCR或调用未批准模型。
 }
 if artifact.Extraction.Text!=nil&&strings.TrimSpace(*artifact.Extraction.Text)=="" {return JobExecutionResult{},platform.NewError(platform.CodeIntegrityError,false,"提取成功没有真实文字")}
 if err=input.CheckLease(ctx);err!=nil {return JobExecutionResult{},err}
 work,tx,cancel,err:=indexer.BeginIndexWrite(ctx,op);if err!=nil {return JobExecutionResult{},err};defer cancel();defer tx.Rollback()
 var ownLease,until string;var cancelled int;var planRaw []byte
 err=tx.QueryRowContext(work,`SELECT lease_id,lease_until,cancelled,plan_json FROM extractions WHERE extraction_id=? AND site_id=? AND data_epoch=?`,payload.ExtractionID,op.Actor.SiteID,op.DataEpoch).Scan(&ownLease,&until,&cancelled,&planRaw)
 if err!=nil {return JobExecutionResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"提取写资格不可读")}
 expires,err:=platform.ParseTimestamp(until);if err!=nil||!expires.After(time.Now())||ownLease!=leaseID||cancelled!=0 {return JobExecutionResult{},platform.NewError(platform.CodeStaleResult,false,"提取原租约写资格已失效")}
 expectedPlan,_:=platform.CanonicalJSON(payload.Plan);if string(expectedPlan)!=string(planRaw) {return JobExecutionResult{},platform.NewError(platform.CodeIntegrityError,false,"提取持久计划与原任务不一致")}
 resultRaw,err:=platform.CanonicalJSON(artifact);if err!=nil {return JobExecutionResult{},err}
 _,err=tx.ExecContext(work,`UPDATE extractions SET state=?,result_json=?,result_hash=? WHERE extraction_id=? AND lease_id=? AND cancelled=0`,artifact.Extraction.State,resultRaw,platform.HashBytes(resultRaw),payload.ExtractionID,leaseID)
 if err!=nil {return JobExecutionResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"真实提取结果保存失败")}
 result,err=indexer.CommitExecutionResult(work,tx,op,"knowledge.extract",payload.ExtractionID,leaseID,artifact);if err!=nil {return JobExecutionResult{},err}
 if err=tx.Commit();err!=nil {return JobExecutionResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"提取最终结果提交失败")};return result,nil
}

// CheckExtractionPlan 首次付费前核仍为原声明；已有步骤只观察原key，不重选新模型/任务记忆。
func CheckExtractionPlan(ctx context.Context,op OperationContext,payload ExtractionJobInput)error {
 if payload.Plan.TaskID=="" {return nil}
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return err}
 var steps int
 if err=runtime.DB.QueryRowContext(ctx,`SELECT count(*) FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND dispatch_started=1`,op.Actor.SiteID,op.DataEpoch,payload.ExtractionID).Scan(&steps);err!=nil {return platform.NormalizeIOError(err,platform.CodeInternal,false,"原提取模型阶段不可读")}
 if steps>0 {return nil}
 current,err:=PrepareExtractionPlan(ctx,op,payload.Source,payload.Plan.ExtractorVersion,payload.Plan.Memory.Language);if err!=nil {return err}
 oldRaw,_:=platform.CanonicalJSON(payload.Plan);newRaw,_:=platform.CanonicalJSON(current)
 if string(oldRaw)!=string(newRaw) {return platform.NewError(platform.CodeConflict,false,"原提取批准计划已变化，不能以新模型绕过原owner")};return nil
}
