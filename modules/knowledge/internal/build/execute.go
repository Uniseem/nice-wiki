package build

import (
 "context"
 "database/sql"
 "encoding/json"
 "time"
 "unicode/utf8"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/chunk"
 "nice-wiki/modules/knowledge/internal/extract"
 "nice-wiki/modules/knowledge/internal/gen"
 indexer "nice-wiki/modules/knowledge/internal/index"
 modelruntime "nice-wiki/modules/knowledge/internal/model"
 "nice-wiki/modules/knowledge/internal/source"
)

type JobExecutionInput = platform.JobExecutionInput
type JobExecutionResult = platform.JobExecutionResult
type EmbeddingPlan struct { Profile indexer.ProfileDeclaration `json:"profile"`;Model indexer.ModelDeclaration `json:"model"`;Batches [][]string `json:"batches"` }

// ExecuteBuild 仅执行持久原批准，免费分词/独立提取/背景/嵌入各阶段保留真实计划与付费结果。
func ExecuteBuild(ctx context.Context,op OperationContext,input JobExecutionInput)(result JobExecutionResult,failure error) {
 defer func(){if failure!=nil {result,failure=indexer.FinishExecutionFailure(ctx,op,input,failure)}}()
 execution,err:=indexer.ReadExecutionRecord(ctx,op,input);if err!=nil {return JobExecutionResult{},err}
 if execution.State=="completed" {return JobExecutionResult{State:platform.JobSucceeded,ResultRef:execution.ResultRef,PaidResultRefs:execution.PaidRefs},nil}
 var payload BuildJobInput
 if json.Unmarshal(execution.Input,&payload)!=nil||payload.BuildID!=execution.ObjectID||payload.Document.DataEpoch!=op.DataEpoch {return JobExecutionResult{},platform.NewError(platform.CodeIntegrityError,false,"原构建完整任务输入损坏")}
 if input.CheckLease==nil {return JobExecutionResult{},platform.NewError(platform.CodeDependencyUnavailable,false,"原构建缺权威租约检查")}
 ctx,err=modelruntime.WithModelLease(ctx,input.CheckLease);if err!=nil {return JobExecutionResult{},err}
 leaseID:=indexer.ExecutionLeaseID(input.JobID,input.FencingToken)
 if err=input.CheckLease(ctx);err!=nil {return JobExecutionResult{},err}
 if err=SetBuildStage(ctx,op,payload.BuildID,leaseID,"fetching");err!=nil {return JobExecutionResult{},err}
 content,err:=source.FetchPublishedSource(ctx,op,source.FetchPublishedSourceInput{BuildID:payload.BuildID,LeaseID:leaseID});if err!=nil {return JobExecutionResult{},err}
 if utf8.RuneCountInString(content.Source)>65536 {return JobExecutionResult{},platform.NewError(platform.CodeBudgetExceeded,false,"完整发布上下文超过固定消息预算，保留旧索引及原结果")}
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return JobExecutionResult{},err}
 rawChunks:=[]gen.Chunk{}
 found,err:=ReadBuildPlan(ctx,op,payload.BuildID,"raw_chunks",&rawChunks);if err!=nil {return JobExecutionResult{},err}
 var model indexer.ModelDeclaration
 if !found {
  catalog,err:=indexer.ReadModelCatalog(ctx,op,runtime);if err!=nil {return JobExecutionResult{},err}
  _,model,err=indexer.EmbeddingProfile(catalog,payload.Config.EmbeddingProfileId,op.CostClass,payload.Config.Space);if err!=nil {return JobExecutionResult{},err}
  if err=SetBuildStage(ctx,op,payload.BuildID,leaseID,"extracting");err!=nil {return JobExecutionResult{},err}
  rawChunks,err=PrepareBuildChunks(ctx,op,payload,content,leaseID,model.ModelID);if err!=nil {return JobExecutionResult{},err}
  if err=FreezeBuildPlan(ctx,op,payload.BuildID,leaseID,"raw_chunks",rawChunks);err!=nil {return JobExecutionResult{},err}
 }
 enriched:=[]gen.Chunk{}
 found,err=ReadBuildPlan(ctx,op,payload.BuildID,"enriched_chunks",&enriched);if err!=nil {return JobExecutionResult{},err}
 if !found {
  if err=SetBuildStage(ctx,op,payload.BuildID,leaseID,"chunking");err!=nil {return JobExecutionResult{},err}
  var memory platform.MemoryContext
  exists,err:=ReadBuildPlan(ctx,op,payload.BuildID,"context_memory",&memory);if err!=nil {return JobExecutionResult{},err}
  if !exists {memory,err=indexer.ResolveTaskMemory(ctx,op,"knowledge.context",payload.Config.Language);if err!=nil {return JobExecutionResult{},err};if err=FreezeBuildPlan(ctx,op,payload.BuildID,leaseID,"context_memory",memory);err!=nil {return JobExecutionResult{},err}}
  // 固定真实embedding模型只承担免费编码，不以目录主模型冒充context实际winning来源。
  catalog,err:=indexer.ReadModelCatalog(ctx,op,runtime);if err!=nil {return JobExecutionResult{},err}
  _,model,err=indexer.EmbeddingProfile(catalog,payload.Config.EmbeddingProfileId,op.CostClass,payload.Config.Space);if err!=nil {return JobExecutionResult{},err}
  for start:=0;start<len(rawChunks);start+=16 {
   if err=input.CheckLease(ctx);err!=nil {return JobExecutionResult{},err}
   group:=rawChunks[start:min(start+16,len(rawChunks))]
   contextResult,err:=chunk.EnrichChunkContext(ctx,op,chunk.EnrichChunkContextInput{BuildID:payload.BuildID,Chunks:group,DocumentText:content.Source,Memory:memory,LeaseID:leaseID});if err!=nil {return JobExecutionResult{},err}
   values,err:=chunk.ApplyChunkContext(ctx,op,group,contextResult,model.ModelID,payload.Config.Space);if err!=nil {return JobExecutionResult{},err};enriched=append(enriched,values...)
  }
  if err=FreezeBuildPlan(ctx,op,payload.BuildID,leaseID,"enriched_chunks",enriched);err!=nil {return JobExecutionResult{},err}
 }
 for start:=0;start<len(enriched);start+=256 {if _,err=indexer.WriteStagingChunks(ctx,op,indexer.WriteStagingChunksInput{BuildID:payload.BuildID,Chunks:enriched[start:min(start+256,len(enriched))],LeaseID:leaseID});err!=nil {return JobExecutionResult{},err}}
 if _,err=indexer.BuildLexicalIndex(ctx,op,indexer.BuildLexicalIndexInput{BuildID:payload.BuildID,Language:payload.Config.Language,LeaseID:leaseID});err!=nil {return JobExecutionResult{},err}
 if err=SetBuildStage(ctx,op,payload.BuildID,leaseID,"embedding");err!=nil {return JobExecutionResult{},err}
 plan:=EmbeddingPlan{}
 found,err=ReadBuildPlan(ctx,op,payload.BuildID,"embedding_batches",&plan);if err!=nil {return JobExecutionResult{},err}
 if !found {plan,err=PrepareEmbeddingPlan(ctx,op,payload.Config,enriched,runtime);if err!=nil {return JobExecutionResult{},err};if err=FreezeBuildPlan(ctx,op,payload.BuildID,leaseID,"embedding_batches",plan);err!=nil {return JobExecutionResult{},err}}
 for _,ids:=range plan.Batches {
  complete,err:=ReuseEmbeddingBatch(ctx,op,payload.BuildID,leaseID,ids,payload.Config.Space);if err!=nil {return JobExecutionResult{},err};if complete {continue}
  if err=input.CheckLease(ctx);err!=nil {return JobExecutionResult{},err}
  if err=CheckEmbeddingPlan(ctx,op,payload.BuildID,ids,payload.Config.Space,plan);err!=nil {return JobExecutionResult{},err}
  value,err:=indexer.RequestEmbeddings(ctx,op,indexer.RequestEmbeddingsInput{BuildID:payload.BuildID,ChunkIDs:ids,Space:payload.Config.Space,LeaseID:leaseID});if err!=nil {return JobExecutionResult{},err}
  if _,err=indexer.StoreEmbeddings(ctx,op,indexer.StoreEmbeddingsInput{BuildID:payload.BuildID,ChunkIDs:ids,Vectors:value.Vectors,Space:value.Space,LeaseID:leaseID});err!=nil {return JobExecutionResult{},err}
 }
 if err=input.CheckLease(ctx);err!=nil {return JobExecutionResult{},err}
 if _,err=ValidateBuild(ctx,op,ValidateBuildInput{BuildID:payload.BuildID,LeaseID:leaseID});err!=nil {return JobExecutionResult{},err}
 var previous int
 err=runtime.DB.QueryRowContext(ctx,`SELECT generation FROM active_indexes WHERE site_id=? AND data_epoch=? AND document_id=? AND space_id=?`,op.Actor.SiteID,op.DataEpoch,payload.Document.DocumentId,payload.Config.Space.SpaceId).Scan(&previous)
 if err!=nil&&err!=sql.ErrNoRows {return JobExecutionResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"原活动代数不可读")}
 if _,err=ActivateBuild(ctx,op,ActivateBuildInput{BuildID:payload.BuildID,ExpectedActiveGeneration:previous,LeaseID:leaseID});err!=nil {return JobExecutionResult{},err}
 completed,err:=indexer.ReadExecutionRecord(ctx,op,input);if err!=nil {return JobExecutionResult{},err}
 return JobExecutionResult{State:platform.JobSucceeded,ResultRef:completed.ResultRef,PaidResultRefs:completed.PaidRefs},nil
}

// PrepareBuildChunks 固定正文与真实资源文字分块；不合并跨块权限，不遗漏续读游标。
func PrepareBuildChunks(ctx context.Context,op OperationContext,payload BuildJobInput,content source.FetchPublishedSourceResult,leaseID,modelID string)([]gen.Chunk,error) {
 blocks:=[]chunk.SourceBlock{};var cursor *source.SourceCursor
 for {value,err:=source.BuildSourceBlocks(ctx,op,source.BuildSourceBlocksInput{Document:payload.Document,Source:content.Source,ModelID:modelID,Space:payload.Config.Space,Cursor:cursor});if err!=nil {return nil,err};blocks=append(blocks,value.Blocks...);if value.Next==nil {break};cursor=value.Next}
 assets,err:=source.ListExtractableAssets(ctx,op,source.ListExtractableAssetsInput{Document:payload.Document,Source:content.Source});if err!=nil {return nil,err}
 bytes:=len(content.Source)
 for _,origin:=range assets.Sources {
  asset:=platform.AssetRef{AssetID:origin.Asset.AssetID,SHA256:origin.Asset.SHA256,MediaType:origin.Asset.MediaType,ByteSize:origin.Asset.ByteSize}
  version:="resource-description-v1";if asset.MediaType=="application/pdf" {version=extract.PDFExtractorVersion};if asset.MediaType=="video/mp4"||asset.MediaType=="video/webm" {version="video-published-metadata-v1"}
  accepted,err:=extract.CreateExtraction(ctx,op,extract.CreateExtractionInput{BuildID:payload.BuildID,LeaseID:leaseID,Asset:asset,Source:origin,ExtractorVersion:version,IDempotencyKey:"source."+platform.HashBytes([]byte(origin.Block.BlockId+asset.SHA256))});if err!=nil {return nil,err}
  // 此等待不领取孩子，也不建立本地续租；专用extract平台worker负责独立执行。
  var artifact extract.ExtractionArtifact
  for {
   artifact,err=extract.ReadExtraction(ctx,op,accepted.Extraction.ExtractionID,origin);if err!=nil {return nil,err}
   if artifact.Extraction.State=="ready"||artifact.Extraction.State=="unsupported" {break}
   if artifact.Extraction.State=="failed" {
    state,err:=extract.ReadExtractionJobState(ctx,op,accepted.Extraction.ExtractionID);if err!=nil {return nil,err}
    switch state {
    case "dead-letter":return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"原资源任务已终止，保留原模型引用与结果")
    case "cancelled":return nil,platform.NewError(platform.CodeCancelled,false,"原资源任务已权威取消")
    case "succeeded":
     artifact,err=extract.ReadExtraction(ctx,op,accepted.Extraction.ExtractionID,origin);if err!=nil {return nil,err}
     if artifact.Extraction.State!="ready"&&artifact.Extraction.State!="unsupported" {return nil,platform.NewError(platform.CodeIntegrityError,false,"原任务成功但提取最终结果不存在")}
    }
    if state=="succeeded" {break}
    // queued/leased/running/retry-wait/cancel-requested/orphan都只等原任务，不另建owner或耗尽父重试。
   }
   timer:=time.NewTimer(time.Second);select {case <-ctx.Done():timer.Stop();return nil,ctx.Err();case <-timer.C:}
  }
  if asset.MediaType=="video/mp4"||asset.MediaType=="video/webm" {continue} // 题名/简介已有真实正文叶，不重复生成另一份检索文字。
  leaves,err:=extract.BindExtractionViews(ctx,op,payload.BuildID,leaseID,origin,artifact);if err!=nil {return nil,err}
  for _,leaf:=range leaves {if len(leaf.Text)>16<<20-bytes {return nil,platform.NewError(platform.CodeBudgetExceeded,false,"完整资源原文超过16MiB构建预算")};bytes+=len(leaf.Text);ref:=&gen.AssetRef{AssetId:asset.AssetID,Sha256:asset.SHA256,MediaType:asset.MediaType,ByteSize:int(asset.ByteSize)};values,err:=source.BuildMaterialBlocks(ctx,op,leaf,ref,modelID,payload.Config.Space);if err!=nil {return nil,err};blocks=append(blocks,values...)}
 }
 result:=[]gen.Chunk{};var split *chunk.SplitCursor
 for {value,err:=chunk.SplitChunks(ctx,op,chunk.SplitChunksInput{BuildID:payload.BuildID,Document:payload.Document,Blocks:blocks,ChunkTokens:payload.Config.ChunkTokens,OverlapTokens:payload.Config.OverlapTokens,Cursor:split});if err!=nil {return nil,err};result=append(result,value.Chunks...);if value.NextCursor==nil {break};split=value.NextCursor}
 return result,nil
}
