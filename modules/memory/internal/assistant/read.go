package assistant

import (
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/memory/internal/gen"
 memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type ReadAssistantRunInput struct{RunId string `json:"runId"`}
type ReadAssistantRunResult=gen.ReadAssistantRunResult
func ReadAssistantRun(ctx context.Context,op OperationContext,input ReadAssistantRunInput)(ReadAssistantRunResult,error){
 if err:=memoryruntime.RequireManager(op);err!=nil{return ReadAssistantRunResult{},err};result:=ReadAssistantRunResult{Proposals:[]gen.MemoryProposal{}}
 err:=memoryruntime.ReadSnapshot(ctx,func(_ *memoryruntime.Store,tx *sql.Tx)error{
  var ids string;var invocation sql.NullString;result.Run.RunId=input.RunId
  err:=tx.QueryRowContext(ctx,`SELECT source_generation,state,invocation_id,actor_id,proposal_ids_json FROM assistant_runs WHERE site_id=? AND run_id=?`,op.Actor.SiteID,input.RunId).Scan(&result.Run.SourceGeneration,&result.Run.State,&invocation,&result.Run.ActorId,&ids)
  if errors.Is(err,sql.ErrNoRows){return platform.NewError(platform.CodeNotFound,false,"记忆助手任务不存在")};if err!=nil{return memoryruntime.DBError(err)};if json.Unmarshal([]byte(ids),&result.Run.ProposalIds)!=nil{return platform.NewError(platform.CodeIntegrityError,false,"助手提议索引损坏")};if invocation.Valid{result.Run.InvocationId=&invocation.String}
  for _,id:=range result.Run.ProposalIds{var raw []byte;if err=tx.QueryRowContext(ctx,`SELECT payload_json FROM proposals WHERE site_id=? AND proposal_id=? AND run_id=?`,op.Actor.SiteID,id,input.RunId).Scan(&raw);err!=nil{return memoryruntime.DBError(err)};var p gen.MemoryProposal;if json.Unmarshal(raw,&p)!=nil{return platform.NewError(platform.CodeIntegrityError,false,"助手提议正文损坏")};result.Proposals=append(result.Proposals,p)};return nil
 });return result,err
}
