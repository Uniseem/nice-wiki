package build

import (
 "context"
 "database/sql"
 "encoding/json"
 "math"
 "net/url"
 "sort"
 "strconv"
 "time"
 "unicode/utf8"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/admin"
 "nice-wiki/modules/knowledge/internal/chunk"
 "nice-wiki/modules/knowledge/internal/extract"
 "nice-wiki/modules/knowledge/internal/gen"
 indexer "nice-wiki/modules/knowledge/internal/index"
 "nice-wiki/modules/knowledge/internal/source"
)

type EstimateRebuildPlanInput = gen.EstimateRebuildPlanInput
type ApproveRebuildPlanInput = gen.ApproveRebuildPlanInput
type ApproveRebuildPlanResult = gen.ApproveRebuildPlanResult
type RebuildMoney struct { Currency string `json:"currency"`;Micros int64 `json:"micros"` }
type RebuildQuoteComponent struct { TaskID string `json:"taskId"`;ModelID string `json:"modelId"`;Quantity int64 `json:"quantity"`;InputTokens int64 `json:"inputTokens"`;MaximumOutputTokens int64 `json:"maximumOutputTokens"`;Quote json.RawMessage `json:"quote"` }
type RebuildQuote struct { PricingKnown bool `json:"pricingKnown"`;MaximumCost *RebuildMoney `json:"maximumCost,omitempty"`;Warning string `json:"warning,omitempty"`;Components []RebuildQuoteComponent `json:"components"` }
type EstimateRebuildPlanResult struct { PlanID string `json:"planId"`;DocumentCount int64 `json:"documentCount"`;ChunkCount int64 `json:"chunkCount"`;ChunkCountKnown bool `json:"chunkCountKnown"`;Quote RebuildQuote `json:"quote"`;QuoteHash string `json:"quoteHash"`;ExpiresAt string `json:"expiresAt"` }
type RebuildRoute struct { indexer.TaskRoute;Memory *platform.MemoryContext `json:"memory,omitempty"`;Prices []platform.ExpectedPriceSnapshot `json:"expectedPriceSnapshots,omitempty"`;PriceQuotes map[string]json.RawMessage `json:"priceQuotes,omitempty"` }
type RebuildResourcePlan struct { Source source.AssetSource `json:"source"`;Plan extract.ExtractionPlan `json:"plan"` }
type RebuildPolicy struct { Embedding RebuildRoute `json:"embedding"`;Context RebuildRoute `json:"context"`;Resources []RebuildResourcePlan `json:"resources"` }
type RebuildDocument struct { Document gen.PublishedRevisionRef `json:"document"`;Policy RebuildPolicy `json:"policy"` }
type RebuildSnapshot struct { Reason string `json:"reason"`;Config gen.IndexConfig `json:"config"`;TreeRevision int64 `json:"treeRevision"`;Documents []RebuildDocument `json:"documents"`;ChunkCount int64 `json:"chunkCount"`;ChunkCountKnown bool `json:"chunkCountKnown"`;Quote RebuildQuote `json:"quote"` }

// EstimateRebuildPlan 仅免费读取/分词和权威报价，不准备付费结果或发起模型。
func EstimateRebuildPlan(ctx context.Context,op OperationContext,input EstimateRebuildPlanInput)(EstimateRebuildPlanResult,error) {
 if err:=admin.AuthorizeAdmin(op);err!=nil {return EstimateRebuildPlanResult{},err}
 if op.CostClass!=platform.CostClassBackground {return EstimateRebuildPlanResult{},platform.NewError(platform.CodeForbidden,false,"全量估价只解析可信后台任务策略")}
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return EstimateRebuildPlanResult{},err}
 var previous []byte;var inputHash string
 inputRaw,_:=platform.CanonicalJSON(input)
 err=runtime.DB.QueryRowContext(ctx,`SELECT result_json,input_hash FROM rebuild_plans WHERE site_id=? AND data_epoch=? AND actor_id=? AND operation_id=?`,op.Actor.SiteID,op.DataEpoch,op.Actor.SubjectID,op.OperationID).Scan(&previous,&inputHash)
 if err==nil {var result EstimateRebuildPlanResult;if inputHash!=platform.HashBytes(inputRaw) {return result,platform.NewError(platform.CodeConflict,false,"同操作已保存其他重建估价")};if json.Unmarshal(previous,&result)!=nil {return result,platform.NewError(platform.CodeIntegrityError,false,"原重建估价回执损坏")};return result,nil}
 if err!=sql.ErrNoRows {return EstimateRebuildPlanResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"原重建估价不可读")}
 snapshot,err:=PrepareRebuildSnapshot(ctx,op,string(input.Reason));if err!=nil {return EstimateRebuildPlanResult{},err}
 raw,err:=platform.CanonicalJSON(snapshot);if err!=nil {return EstimateRebuildPlanResult{},err};if len(raw)>16<<20 {return EstimateRebuildPlanResult{},platform.NewError(platform.CodeBudgetExceeded,false,"完整重建策略超过16MiB持久预算")}
 result:=EstimateRebuildPlanResult{PlanID:platform.NewIdentifier(),DocumentCount:int64(len(snapshot.Documents)),ChunkCount:snapshot.ChunkCount,ChunkCountKnown:snapshot.ChunkCountKnown,Quote:snapshot.Quote,QuoteHash:platform.HashBytes(raw),ExpiresAt:time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339Nano)}
 resultRaw,_:=platform.CanonicalJSON(result)
 work,tx,cancel,err:=indexer.BeginIndexWrite(ctx,op);if err!=nil {return result,err};defer cancel();defer tx.Rollback()
 current,err:=indexer.ReadConfig(work,tx,op);if err!=nil {return result,err};before,_:=platform.CanonicalJSON(snapshot.Config);after,_:=platform.CanonicalJSON(current);if string(before)!=string(after) {return result,platform.NewError(platform.CodeVersionMismatch,false,"估价期间索引配置变化")}
 _,err=tx.ExecContext(work,`INSERT INTO rebuild_plans(plan_id,site_id,data_epoch,actor_id,operation_id,input_hash,snapshot_json,snapshot_hash,result_json,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,result.PlanID,op.Actor.SiteID,op.DataEpoch,op.Actor.SubjectID,op.OperationID,platform.HashBytes(inputRaw),raw,result.QuoteHash,resultRaw,result.ExpiresAt)
 if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeConflict,false,"原重建估价保存冲突")};if err=tx.Commit();err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"重建估价提交失败")};return result,nil
}

// ApproveRebuildPlan 一次事务批准固定全量集合并创建原任务；实际派发在提交之后。
func ApproveRebuildPlan(ctx context.Context,op OperationContext,input ApproveRebuildPlanInput)(ApproveRebuildPlanResult,error) {
 if err:=admin.AuthorizeAdmin(op);err!=nil {return ApproveRebuildPlanResult{},err}
 if op.CostClass!=platform.CostClassBackground {return ApproveRebuildPlanResult{},platform.NewError(platform.CodeForbidden,false,"重建批准只接受可信后台费用类别")}
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return ApproveRebuildPlanResult{},err}
 var raw,approved []byte;var hash,expires,site,epoch,owner string
 err=runtime.DB.QueryRowContext(ctx,`SELECT snapshot_json,snapshot_hash,expires_at,site_id,data_epoch,actor_id,approved_result_json FROM rebuild_plans WHERE plan_id=?`,input.PlanId).Scan(&raw,&hash,&expires,&site,&epoch,&owner,&approved)
 if err!=nil {return ApproveRebuildPlanResult{},platform.NormalizeIOError(err,platform.CodeNotFound,false,"重建计划不存在")}
 if site!=op.Actor.SiteID||epoch!=op.DataEpoch||owner!=op.Actor.SubjectID {return ApproveRebuildPlanResult{},platform.NewError(platform.CodeForbidden,false,"重建计划不属于当前管理员及本站同代")}
 if hash!=platform.HashBytes(raw)||hash!=input.QuoteHash {return ApproveRebuildPlanResult{},platform.NewError(platform.CodeConflict,false,"重建估价摘要不一致")}
 var snapshot RebuildSnapshot;if json.Unmarshal(raw,&snapshot)!=nil {return ApproveRebuildPlanResult{},platform.NewError(platform.CodeIntegrityError,false,"重建计划损坏")}
 if len(approved)>0 {var result ApproveRebuildPlanResult;if json.Unmarshal(approved,&result)!=nil {return result,platform.NewError(platform.CodeIntegrityError,false,"原批准回执损坏")};return result,nil}
 until,err:=platform.ParseTimestamp(expires);if err!=nil||!until.After(time.Now()) {return ApproveRebuildPlanResult{},platform.NewError(platform.CodeValidation,false,"重建估价已过期")}
 current,err:=PrepareRebuildSnapshot(ctx,op,snapshot.Reason);if err!=nil {return ApproveRebuildPlanResult{},err};currentRaw,_:=platform.CanonicalJSON(current)
 if platform.HashBytes(currentRaw)!=hash {return ApproveRebuildPlanResult{},platform.NewError(platform.CodeConflict,false,"源修订、模型、记忆、价格或可复用结果变化，请重新估价")}
 work,tx,cancel,err:=indexer.BeginIndexWrite(ctx,op);if err!=nil {return ApproveRebuildPlanResult{},err};defer cancel();defer tx.Rollback()
 var savedHash string;var savedResult []byte
 if err=tx.QueryRowContext(work,`SELECT snapshot_hash,approved_result_json FROM rebuild_plans WHERE plan_id=?`,input.PlanId).Scan(&savedHash,&savedResult);err!=nil {return ApproveRebuildPlanResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"批准CAS来源不可读")}
 if savedHash!=hash {return ApproveRebuildPlanResult{},platform.NewError(platform.CodeConflict,false,"批准前计划已变化")}
 if len(savedResult)>0 {var result ApproveRebuildPlanResult;if json.Unmarshal(savedResult,&result)!=nil {return result,platform.NewError(platform.CodeIntegrityError,false,"原批准回执损坏")};return result,nil}
 config,err:=indexer.ReadConfig(work,tx,op);if err!=nil {return ApproveRebuildPlanResult{},err};a,_:=platform.CanonicalJSON(config);b,_:=platform.CanonicalJSON(snapshot.Config);if string(a)!=string(b)||!until.After(time.Now()) {return ApproveRebuildPlanResult{},platform.NewError(platform.CodeConflict,false,"批准前配置变化或计划到期")}
 ids:=[]string{}
 for _,document:=range snapshot.Documents {
  value,err:=QueueBuild(work,tx,op,config,document.Document,true);if err!=nil {return ApproveRebuildPlanResult{},err};ids=append(ids,value.BuildId)
  policyRaw,_:=platform.CanonicalJSON(document.Policy)
  if _,err=tx.ExecContext(work,`INSERT INTO build_execution_plans(build_id,phase,plan_json,plan_hash) VALUES(?,'rebuild_approval',?,?)`,value.BuildId,policyRaw,platform.HashBytes(policyRaw));err!=nil {return ApproveRebuildPlanResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"原重建批准策略保存失败")}
 }
 result:=ApproveRebuildPlanResult{BuildJobs:len(ids)};resultRaw,_:=platform.CanonicalJSON(result);idRaw,_:=platform.CanonicalJSON(ids)
 changed,err:=tx.ExecContext(work,`UPDATE rebuild_plans SET approved_by=?,approved_at=?,approved_root=?,approved_result_json=?,build_ids_json=? WHERE plan_id=? AND snapshot_hash=? AND approved_result_json IS NULL`,op.Actor.SubjectID,platform.Now(),op.RootOperationID,resultRaw,idRaw,input.PlanId,hash)
 if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原重建批准保存失败")};count,err:=changed.RowsAffected();if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"重建批准CAS不可读")};if count!=1 {return result,platform.NewError(platform.CodeConflict,false,"重建计划已批准")}
 if err=tx.Commit();err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"重建批准提交失败")}
 for _,id:=range ids {_=DispatchBuild(ctx,op,id)};return result,nil
}

func ReadRebuildDocuments(ctx context.Context,op OperationContext,runtime indexer.Runtime)([]gen.PublishedRevisionRef,int64,error) {
 if runtime.Content==nil||runtime.Validators["ContentReadPublicTreeResult"]==nil {return nil,0,platform.NewError(platform.CodeDependencyUnavailable,false,"公开树权威未装配")}
 raw,err:=runtime.Content.ReadPublicTree(platform.WithOperationContext(ctx,op));if err!=nil {return nil,0,err};if err=runtime.Validators["ContentReadPublicTreeResult"](raw);err!=nil {return nil,0,err}
 var result struct{Nodes []struct{Reference gen.PublishedRevisionRef `json:"reference"`} `json:"nodes"`;TreeRevision int64 `json:"treeRevision"`}
 if json.Unmarshal(raw,&result)!=nil {return nil,0,platform.NewError(platform.CodeIntegrityError,false,"公开树固定修订不可读")};if len(result.Nodes)>256 {return nil,0,platform.NewError(platform.CodeBudgetExceeded,false,"全量估价超过256文档同步完整预算，不截断")}
 documents:=make([]gen.PublishedRevisionRef,len(result.Nodes));for i,node:=range result.Nodes {if node.Reference.DataEpoch!=op.DataEpoch {return nil,0,platform.NewError(platform.CodeStaleResult,false,"公开树不是当前代次")};documents[i]=node.Reference};sort.Slice(documents,func(i,j int)bool{return documents[i].DocumentId<documents[j].DocumentId});return documents,result.TreeRevision,nil
}

func PrepareRebuildPolicy(ctx context.Context,op OperationContext,runtime indexer.Runtime,config gen.IndexConfig,resources []source.AssetSource)(RebuildPolicy,error) {
 result:=RebuildPolicy{Resources:[]RebuildResourcePlan{}}
 embedding,err:=indexer.ReadTaskRoute(ctx,op,runtime,"knowledge.embed");if err!=nil {return result,err}
 if embedding.Profile.ProfileID!=config.EmbeddingProfileId {return result,platform.NewError(platform.CodeConflict,false,"嵌入任务路由与已配置档位不一致")}
 for _,candidate:=range embedding.Models {if candidate.EmbeddingSpace==nil||*candidate.EmbeddingSpace!=config.Space {return result,platform.NewError(platform.CodeConflict,false,"完整嵌入路由包含其他空间候选")}}
 result.Embedding.TaskRoute=embedding
 contextRoute,err:=indexer.ReadTaskRoute(ctx,op,runtime,"knowledge.context");if err!=nil {return result,err};result.Context.TaskRoute=contextRoute
 memory,err:=indexer.ResolveTaskMemory(ctx,op,"knowledge.context",config.Language);if err!=nil {return result,err};result.Context.Memory=&memory
 for _,origin:=range resources {if !origin.Supported {continue};version:="resource-description-v1";switch origin.Asset.MediaType {case "application/pdf":version=extract.PDFExtractorVersion;case "video/mp4","video/webm":version="video-published-metadata-v1"}
  plan,err:=extract.PrepareExtractionPlan(ctx,op,origin,version,config.Language);if err!=nil {return result,err};result.Resources=append(result.Resources,RebuildResourcePlan{origin,plan})
 };return result,nil
}

// CheckRebuildApproval 每次可能付费前核原明批策略，不因目录/记忆漂移自动新费用；正常单篇构建无该记录。
func CheckRebuildApproval(ctx context.Context,op OperationContext,buildID string,config gen.IndexConfig)error {
 var expected RebuildPolicy
 found,err:=ReadBuildPlan(ctx,op,buildID,"rebuild_approval",&expected);if err!=nil||!found {return err}
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return err}
 origins:=make([]source.AssetSource,len(expected.Resources));for i,value:=range expected.Resources {origins[i]=value.Source}
 current,err:=PrepareRebuildPolicy(ctx,op,runtime,config,origins);if err!=nil {return err}
 expected.Embedding.Prices=nil;expected.Context.Prices=nil;expected.Embedding.PriceQuotes=nil;expected.Context.PriceQuotes=nil;for i:=range expected.Resources {expected.Resources[i].Plan.ExpectedPriceSnapshots=nil;expected.Resources[i].Plan.PriceQuotes=nil}
 a,_:=platform.CanonicalJSON(expected);b,_:=platform.CanonicalJSON(current);if string(a)!=string(b) {return platform.NewError(platform.CodeConflict,false,"全量重建明批模型、任务策略或记忆变化，保留原结果不另付费")};return nil
}

func AddRebuildQuoteRoute(components map[string]*RebuildQuoteComponent,route RebuildRoute,quantity,output int64)error {
 if quantity==0 {return nil}
 for _,model:=range route.Models {key:=route.TaskID+"\x00"+model.ModelID;component:=components[key];if component==nil {if len(components)>=256 {return platform.NewError(platform.CodeBudgetExceeded,false,"全量估价分项超过256完整预算")};component=&RebuildQuoteComponent{TaskID:route.TaskID,ModelID:model.ModelID,InputTokens:int64(model.MaximumInputTokens),MaximumOutputTokens:output};components[key]=component}
  if component.Quantity>math.MaxInt64-quantity||component.MaximumOutputTokens!=output||component.InputTokens!=int64(model.MaximumInputTokens) {return platform.NewError(platform.CodeBudgetExceeded,false,"报价数量或固定模型窗口不一致")};if string(component.Quote)!=""&&string(component.Quote)!=string(route.PriceQuotes[model.ModelID]) {return platform.NewError(platform.CodeConflict,false,"同全量估价模型报价并发变化")};component.Quote=route.PriceQuotes[model.ModelID];component.Quantity+=quantity
 };return nil
}

func QuoteRebuildComponents(ctx context.Context,op OperationContext,runtime indexer.Runtime,components map[string]*RebuildQuoteComponent,countKnown bool)(RebuildQuote,error) {
 result:=RebuildQuote{PricingKnown:countKnown,Components:[]RebuildQuoteComponent{}}
 if runtime.Gateway==nil||runtime.Validators["GatewayQuoteCostResult"]==nil {return result,platform.NewError(platform.CodeDependencyUnavailable,false,"真实模型报价权威未装配")}
 keys:=make([]string,0,len(components));for key:=range components {keys=append(keys,key)};sort.Strings(keys)
 total:=int64(0)
 for _,key:=range keys {component:=components[key]
  raw:=component.Quote;if len(raw)==0 {return result,platform.NewError(platform.CodeIntegrityError,false,"完整分项缺原冻结真实报价")}
  if err:=runtime.Validators["GatewayQuoteCostResult"](raw);err!=nil {return result,err}
  var quote struct{PricingKnown bool `json:"pricingKnown"`;MaximumCost *RebuildMoney `json:"maximumCost"`}
  if json.Unmarshal(raw,&quote)!=nil {return result,platform.NewError(platform.CodeIntegrityError,false,"真实模型报价不可读")};component.Quote=raw;result.Components=append(result.Components,*component)
  if !quote.PricingKnown||quote.MaximumCost==nil {result.PricingKnown=false;continue}
  cost:=quote.MaximumCost;if cost.Currency!="USD"||cost.Micros<0 {return result,platform.NewError(platform.CodeIntegrityError,false,"真实模型报价金额不合法")}
  if cost.Micros!=0&&component.Quantity>(math.MaxInt64-total)/cost.Micros {return result,platform.NewError(platform.CodeBudgetExceeded,false,"完整重建最大费用超过整数预算")};total+=cost.Micros*component.Quantity
 }
 if result.PricingKnown {result.MaximumCost=&RebuildMoney{Currency:"USD",Micros:total}} else {result.Warning="报价包含真实已知分项；缺价或未生成的资源文字使完整分块数/总费用上限未知，不以0代替。"}
 return result,nil
}

func PrepareRebuildSnapshot(ctx context.Context,op OperationContext,reason string)(RebuildSnapshot,error) {
 result:=RebuildSnapshot{Reason:reason,Documents:[]RebuildDocument{},ChunkCountKnown:true}
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return result,err}
 read,err:=runtime.DB.BeginTx(ctx,&sql.TxOptions{ReadOnly:true});if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"估价配置不可读")};result.Config,err=indexer.ReadConfig(ctx,read,op);_=read.Rollback();if err!=nil {return result,err};if !result.Config.Enabled {return result,platform.NewError(platform.CodeValidation,false,"知识入库未启用")}
 documents,revision,err:=ReadRebuildDocuments(ctx,op,runtime);if err!=nil {return result,err};result.TreeRevision=revision
 components:=map[string]*RebuildQuoteComponent{};sourceBytes:=0;allChunks:=0;policyBytes:=0
 for _,document:=range documents {
  body,err:=ReadRebuildSource(ctx,op,runtime,document);if err!=nil {return result,err};sourceBytes+=len(body);if sourceBytes>16<<20 {return result,platform.NewError(platform.CodeBudgetExceeded,false,"全量估价原文超过16MiB同步读取预算")}
  assets,err:=source.ListExtractableAssets(ctx,op,source.ListExtractableAssetsInput{Document:document,Source:string(body)});if err!=nil {return result,err}
  policy,err:=PrepareRebuildPolicy(ctx,op,runtime,result.Config,assets.Sources);if err!=nil {return result,err};if err=FreezeRebuildRoutePrices(ctx,op,runtime,&policy.Embedding,0);err!=nil {return result,err};if err=FreezeRebuildRoutePrices(ctx,op,runtime,&policy.Context,500);err!=nil {return result,err};for i:=range policy.Resources {resource:=&policy.Resources[i];if resource.Plan.TaskID!="" {route:=RebuildRoute{TaskRoute:resource.Plan.Route};if err=FreezeRebuildRoutePrices(ctx,op,runtime,&route,800);err!=nil {return result,err};resource.Plan.ExpectedPriceSnapshots=route.Prices;resource.Plan.PriceQuotes=route.PriceQuotes}};policyRaw,_:=platform.CanonicalJSON(policy);policyBytes+=len(policyRaw);if policyBytes>16<<20 {return result,platform.NewError(platform.CodeBudgetExceeded,false,"完整重建模型及记忆策略超过16MiB预算")};result.Documents=append(result.Documents,RebuildDocument{document,policy})
  blocks:=[]chunk.SourceBlock{};var cursor *source.SourceCursor
  for {value,err:=source.BuildSourceBlocks(ctx,op,source.BuildSourceBlocksInput{Document:document,Source:string(body),ModelID:policy.Embedding.Models[0].ModelID,Space:result.Config.Space,Cursor:cursor});if err!=nil {return result,err};blocks=append(blocks,value.Blocks...);if value.Next==nil {break};cursor=value.Next}
  for _,resource:=range policy.Resources {
   switch resource.Source.Asset.MediaType {case "video/mp4","video/webm":continue}
   artifact,found,err:=ReadRebuildExtractionCache(ctx,op,runtime,resource);if err!=nil {return result,err}
   if !found {
    switch resource.Source.Asset.MediaType {case "video/mp4","video/webm","image/svg+xml":continue}
    result.ChunkCountKnown=false
    if resource.Plan.TaskID!="" {route:=RebuildRoute{TaskRoute:resource.Plan.Route,Prices:resource.Plan.ExpectedPriceSnapshots,PriceQuotes:resource.Plan.PriceQuotes};if err=AddRebuildQuoteRoute(components,route,1,800);err!=nil {return result,err}}
    continue
   }
   if artifact.Extraction.State!="ready" {continue}
   leaves:=[]source.SourceLeaf{}
   if len(artifact.Pages)>0 {for _,page:=range artifact.Pages {leaves=append(leaves,source.SourceLeaf{Block:resource.Source.Block,Path:"/extractions/"+artifact.Extraction.ExtractionID+"/pages/"+strconv.Itoa(page.Page),Text:page.Text})}} else if artifact.Extraction.Text!=nil {leaves=append(leaves,source.SourceLeaf{Block:resource.Source.Block,Path:"/extractions/"+artifact.Extraction.ExtractionID+"/text",Text:*artifact.Extraction.Text})}
   asset:=gen.AssetRef{AssetId:resource.Source.Asset.AssetID,Sha256:resource.Source.Asset.SHA256,MediaType:resource.Source.Asset.MediaType,ByteSize:int(resource.Source.Asset.ByteSize)}
   for _,leaf:=range leaves {sourceBytes+=len(leaf.Text);if sourceBytes>16<<20 {return result,platform.NewError(platform.CodeBudgetExceeded,false,"全量估价含资源原文超过16MiB完整预算")};values,err:=source.BuildMaterialBlocks(ctx,op,leaf,&asset,policy.Embedding.Models[0].ModelID,result.Config.Space);if err!=nil {return result,err};blocks=append(blocks,values...)}
  }
  chunks:=[]gen.Chunk{};var split *chunk.SplitCursor
  for {value,err:=chunk.SplitChunks(ctx,op,chunk.SplitChunksInput{BuildID:document.DocumentId,Document:document,Blocks:blocks,ChunkTokens:result.Config.ChunkTokens,OverlapTokens:result.Config.OverlapTokens,Cursor:split});if err!=nil {return result,err};chunks=append(chunks,value.Chunks...);if len(chunks)+allChunks>4096 {return result,platform.NewError(platform.CodeBudgetExceeded,false,"全量估价超过4096真实分块预算，不截断")};if value.NextCursor==nil {break};split=value.NextCursor}
  allChunks+=len(chunks)
  missingVectors:=int64(0)
  for start:=0;start<len(chunks);start+=16 {
   missingContext:=false
   for _,value:=range chunks[start:min(start+16,len(chunks))] {
    enriched,found,err:=ReadRebuildCachedContext(ctx,op,runtime,document,policy.Context,value);if err!=nil {return result,err}
    if !found {missingContext=true;missingVectors++;continue}
    reused,err:=HasRebuildVector(ctx,op,runtime,enriched,result.Config.Space);if err!=nil {return result,err};if !reused {missingVectors++}
   }
   if missingContext {if err=AddRebuildQuoteRoute(components,policy.Context,1,500);err!=nil {return result,err}}
  }
  if err=AddRebuildQuoteRoute(components,policy.Embedding,missingVectors,0);err!=nil {return result,err};result.ChunkCount+=missingVectors
 }
 result.Quote,err=QuoteRebuildComponents(ctx,op,runtime,components,result.ChunkCountKnown);return result,err
}

func ReadRebuildSource(ctx context.Context,op OperationContext,runtime indexer.Runtime,document gen.PublishedRevisionRef)([]byte,error) {
 if runtime.Content==nil||runtime.Validators["ContentReadPublishedRevisionResult"]==nil {return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"估价正文权威未装配")}
 raw,err:=runtime.Content.ReadPublishedRevision(platform.WithOperationContext(ctx,op),document.DocumentId,url.Values{"publishedRevisionId":{document.PublishedRevisionId}});if err!=nil {return nil,err};if err=runtime.Validators["ContentReadPublishedRevisionResult"](raw);err!=nil {return nil,err}
 var result struct{Reference gen.PublishedRevisionRef `json:"reference"`;Body json.RawMessage `json:"body"`};if json.Unmarshal(raw,&result)!=nil||result.Reference!=document {return nil,platform.NewError(platform.CodeIntegrityError,false,"估价正文权威引用不一致")};canonical,err:=platform.CanonicalJSON(result.Body);if err!=nil {return nil,err}
 if len(canonical)>16<<20||platform.HashBytes(canonical)!=document.ContentHash {return nil,platform.NewError(platform.CodeIntegrityError,false,"估价正文hash不一致")};if utf8.RuneCount(canonical)>65536 {return nil,platform.NewError(platform.CodeBudgetExceeded,false,"完整发布上下文超过固定模型消息预算，不截断估价")};return canonical,nil
}

// FreezeRebuildRoutePrices 逐权威eligible候选免费读取完整价格快照；不以缺价或未知未来材料补0。
func FreezeRebuildRoutePrices(ctx context.Context,op OperationContext,runtime indexer.Runtime,route *RebuildRoute,output int64)error {
 if runtime.Gateway==nil||runtime.Validators["GatewayQuoteCostResult"]==nil {return platform.NewError(platform.CodeDependencyUnavailable,false,"真实模型报价未装配")}
 route.Prices=[]platform.ExpectedPriceSnapshot{};route.PriceQuotes=map[string]json.RawMessage{}
 service:=op;service.CallerService="knowledge";service.Actor=platform.AuthContext{SubjectID:"knowledge",Kind:"service",SiteID:op.Actor.SiteID,ExpiresAt:op.Deadline,Grants:[]string{}}
 for _,model:=range route.Models {
  raw,err:=runtime.Gateway.QuoteCost(platform.WithOperationContext(platform.WithPortalProof(ctx,""),service),struct{ModelID string `json:"modelId"`;InputTokens int64 `json:"inputTokens"`;MaximumOutputTokens int64 `json:"maximumOutputTokens"`}{model.ModelID,int64(model.MaximumInputTokens),output})
  if err!=nil {return err};if err=runtime.Validators["GatewayQuoteCostResult"](raw);err!=nil {return err}
  var quote struct {PriceSnapshot struct {ID string `json:"priceSnapshotId"`;ModelID string `json:"modelId"`} `json:"priceSnapshot"`}
  if json.Unmarshal(raw,&quote)!=nil||quote.PriceSnapshot.ModelID!=model.ModelID||quote.PriceSnapshot.ID=="" {return platform.NewError(platform.CodeIntegrityError,false,"真实报价价格快照来源不一致")}
  route.Prices=append(route.Prices,platform.ExpectedPriceSnapshot{ModelID:model.ModelID,PriceSnapshotID:quote.PriceSnapshot.ID});canonical,err:=platform.CanonicalJSON(raw);if err!=nil {return err};route.PriceQuotes[model.ModelID]=canonical
 };return nil
}
