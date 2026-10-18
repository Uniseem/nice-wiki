package proposal

import (
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/memory/internal/gen"
 memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type RejectProposalInput=gen.RejectProposalInput
type RejectProposalResult=gen.RejectProposalResult
func RejectProposal(ctx context.Context,op OperationContext,input RejectProposalInput)(RejectProposalResult,error){
 if err:=memoryruntime.RequireManager(op);err!=nil{return RejectProposalResult{},err};var result RejectProposalResult
 err:=memoryruntime.Write(ctx,op,"memory.RejectProposal",input.IdempotencyKey,input,&result,func(_ *memoryruntime.Store,tx *sql.Tx)error{
  var raw []byte;var state string;var revision int
  err:=tx.QueryRowContext(ctx,`SELECT payload_json,state,approval_revision FROM proposals WHERE site_id=? AND proposal_id=?`,op.Actor.SiteID,input.ProposalId).Scan(&raw,&state,&revision);if errors.Is(err,sql.ErrNoRows){return platform.NewError(platform.CodeNotFound,false,"记忆提议不存在")};if err!=nil{return memoryruntime.DBError(err)}
  if state!="pending"{return platform.NewError(platform.CodeConflict,false,"只有待审阅提议可以拒绝")};if revision!=input.ExpectedApprovalRevision{return platform.NewError(platform.CodeVersionMismatch,false,"提议修订已变化")}
  if json.Unmarshal(raw,&result.Proposal)!=nil{return platform.NewError(platform.CodeIntegrityError,false,"记忆提议正文损坏")};result.Proposal.State="rejected";result.Proposal.ApprovalRevision++;result.Proposal.ApprovedBy=&op.Actor.SubjectID
  updated,_:=platform.CanonicalJSON(result.Proposal)
  _,err=tx.ExecContext(ctx,`UPDATE proposals SET state='rejected',approved_by=?,approval_revision=?,payload_json=?,rejection_reason=? WHERE site_id=? AND proposal_id=? AND state='pending' AND approval_revision=?`,op.Actor.SubjectID,result.Proposal.ApprovalRevision,updated,input.Reason,op.Actor.SiteID,input.ProposalId,revision);return memoryruntime.DBError(err)
 });return result,err
}

type ApproveProposalInput=gen.ApproveProposalInput
type ApproveProposalResult=gen.ApproveProposalResult
func ApproveProposal(ctx context.Context,op OperationContext,input ApproveProposalInput)(ApproveProposalResult,error){
 if err:=memoryruntime.RequireManager(op);err!=nil{return ApproveProposalResult{},err};var result ApproveProposalResult
 err:=memoryruntime.Write(ctx,op,"memory.ApproveProposal",input.IdempotencyKey,input,&result,func(s *memoryruntime.Store,tx *sql.Tx)error{
  p,err:=ReadProposal(ctx,tx,op.Actor.SiteID,input.ProposalId);if err!=nil{return err}
  if p.State!="pending"{return platform.NewError(platform.CodeConflict,false,"只有待审阅提议可以批准")};if p.ApprovalRevision!=input.ExpectedApprovalRevision||p.PayloadHash!=input.ExpectedPayloadHash{return platform.NewError(platform.CodeVersionMismatch,false,"提议版本或摘要已变化")}
  var generation int;var runState string;if err=tx.QueryRowContext(ctx,`SELECT source_generation,state FROM assistant_runs WHERE site_id=? AND run_id=?`,op.Actor.SiteID,p.RunId).Scan(&generation,&runState);err!=nil{return memoryruntime.DBError(err)};if runState!="review"{return platform.NewError(platform.CodeVersionMismatch,false,"助手候选已失效或尚未完成")}
  checked,err:=ValidateProposal(ctx,op,ValidateProposalInput{Proposal:p,SourceGeneration:generation});if err!=nil{return err};if checked.ProposalHash!=p.PayloadHash{return platform.NewError(platform.CodeIntegrityError,false,"提议持久摘要不一致")}
  p.State="approved";p.ApprovedBy=&op.Actor.SubjectID;p.ApprovalRevision++
  if err=SaveProposal(ctx,tx,op,p);err!=nil{return err}
  if err=memoryruntime.Emit(tx,op,"memory.proposal.approved",p.ProposalId,p.ApprovalRevision,map[string]any{"proposalId":p.ProposalId,"proposalHash":p.PayloadHash,"approvalRevision":p.ApprovalRevision,"approvedBy":op.Actor.SubjectID,"kind":p.Kind,"dataEpoch":op.DataEpoch});err!=nil{return err}
  if p.Kind=="glossary"{enqueued,err:=EnqueueGlossaryCommand(ctx,tx,op,p);result.Proposal=enqueued.Proposal;return err}
  expected:=0;if p.ExpectedEntryVersion!=nil{expected=*p.ExpectedEntryVersion};applied,err:=ApplyEntryProposal(ctx,s,tx,op,p,expected);result.Proposal=applied.Proposal;return err
 });return result,err
}
func ReadProposal(ctx context.Context,q memoryruntime.Queryer,site,id string)(gen.MemoryProposal,error){
 var raw []byte;err:=q.QueryRowContext(ctx,`SELECT payload_json FROM proposals WHERE site_id=? AND proposal_id=?`,site,id).Scan(&raw);if errors.Is(err,sql.ErrNoRows){return gen.MemoryProposal{},platform.NewError(platform.CodeNotFound,false,"记忆提议不存在")};if err!=nil{return gen.MemoryProposal{},memoryruntime.DBError(err)};var p gen.MemoryProposal;if json.Unmarshal(raw,&p)!=nil{return p,platform.NewError(platform.CodeIntegrityError,false,"记忆提议正文损坏")};return p,nil
}
func SaveProposal(ctx context.Context,tx *sql.Tx,op OperationContext,p gen.MemoryProposal)error{
 raw,err:=platform.CanonicalJSON(p);if err!=nil{return platform.NewError(platform.CodeInternal,false,"记忆提议不可编码")}
 _,err=tx.ExecContext(ctx,`UPDATE proposals SET state=?,approved_by=?,approval_revision=?,payload_json=? WHERE site_id=? AND proposal_id=?`,p.State,p.ApprovedBy,p.ApprovalRevision,raw,op.Actor.SiteID,p.ProposalId);return memoryruntime.DBError(err)
}
