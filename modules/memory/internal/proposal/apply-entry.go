package proposal

import (
 "context"
 "database/sql"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/memory/internal/entry"
 "nice-wiki/modules/memory/internal/gen"
 memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type ApplyEntryProposalInput struct{ProposalId string `json:"proposalId"`;ExpectedEntryVersion int `json:"expectedEntryVersion"`;IdempotencyKey string `json:"idempotencyKey"`}
type ApplyEntryProposalResult struct{Proposal gen.MemoryProposal `json:"proposal"`;Entry *gen.MemoryEntry `json:"entry,omitempty"`}
func ApplyEntryProposal(ctx context.Context,s *memoryruntime.Store,tx *sql.Tx,op OperationContext,p gen.MemoryProposal,expected int)(ApplyEntryProposalResult,error){
 result:=ApplyEntryProposalResult{Proposal:p}
 if p.State!="approved"||p.ApprovedBy==nil||p.ApprovalRevision<1||p.Kind=="glossary"{return result,platform.NewError(platform.CodeForbidden,false,"记忆提议尚未由人工批准")}
 var e gen.MemoryEntry;var previous *int
 if p.Kind=="entry_create"{
  if p.ProposedTitle==nil||p.ProposedBody==nil||p.ProposedLanguage==nil{return result,platform.NewError(platform.CodeIntegrityError,false,"新建记忆提议字段缺失")}
  e=gen.MemoryEntry{EntryId:platform.NewIdentifier(),Version:0,Title:*p.ProposedTitle,Description:*p.ProposedTitle,Body:*p.ProposedBody,Language:*p.ProposedLanguage,MaximumBytes:1536,Priority:100,Tags:[]string{},State:"active",Source:"assistant",Readonly:false,AutoUpdateEnabled:false}
 }else{
  if p.EntryId==nil||p.ExpectedEntryVersion==nil{return result,platform.NewError(platform.CodeIntegrityError,false,"记忆提议目标版本缺失")}
  current,err:=s.ReadEntryObject(ctx,tx,op.Actor.SiteID,*p.EntryId,nil);if err!=nil{return result,err}
  if current.Version!=*p.ExpectedEntryVersion||current.Version!=expected{p.State="conflicted";result.Proposal=p;if err=SaveProposal(ctx,tx,op,p);err!=nil{return result,err};return result,memoryruntime.Emit(tx,op,"memory.proposal.conflicted",p.ProposalId,p.ApprovalRevision,map[string]any{"proposalId":p.ProposalId,"proposalHash":p.PayloadHash,"approvalRevision":p.ApprovalRevision,"code":platform.CodeVersionMismatch,"dataEpoch":op.DataEpoch})}
  if current.Readonly{return result,platform.NewError(platform.CodeForbidden,false,"只读记忆只能由管理员直接编辑")}
  if current.State!="active"{return result,platform.NewError(platform.CodeConflict,false,"助手不能修改归档记忆")}
  v:=current.Version;previous=&v;e=current;e.Version++;e.Source="assistant";e.AutoUpdateEnabled=false
  if p.Kind=="entry_archive"{e.State="archived"}else{if p.ProposedTitle==nil||p.ProposedBody==nil||p.ProposedLanguage==nil{return result,platform.NewError(platform.CodeIntegrityError,false,"更新记忆提议字段缺失")};e.Title=*p.ProposedTitle;e.Body=*p.ProposedBody;e.Language=*p.ProposedLanguage}
 }
 saved,err:=entry.PersistVersion(ctx,s,tx,op,e,previous);if err!=nil{return result,err}
 p.State="applied";if p.EntryId==nil{p.EntryId=&saved.EntryId};if err=SaveProposal(ctx,tx,op,p);err!=nil{return result,err}
 if err=memoryruntime.Emit(tx,op,"memory.proposal.applied",p.ProposalId,p.ApprovalRevision,map[string]any{"proposalId":p.ProposalId,"proposalHash":p.PayloadHash,"approvalRevision":p.ApprovalRevision,"entryId":saved.EntryId,"dataEpoch":op.DataEpoch});err!=nil{return result,err}
 return ApplyEntryProposalResult{Proposal:p,Entry:&saved},nil
}
