package proposal

import (
 "context"
 "database/sql"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/memory/internal/gen"
 memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type EnqueueGlossaryCommandInput struct{ProposalId string `json:"proposalId"`;IdempotencyKey string `json:"idempotencyKey"`}
type EnqueueGlossaryCommandResult struct{CommandId string `json:"commandId"`;Proposal gen.MemoryProposal `json:"proposal"`}
func EnqueueGlossaryCommand(ctx context.Context,tx *sql.Tx,op OperationContext,p gen.MemoryProposal)(EnqueueGlossaryCommandResult,error){
 if p.Kind!="glossary"||p.State!="approved"||p.ApprovedBy==nil||p.SourceLanguage==nil||p.SourceTerm==nil||p.TargetTerm==nil||p.TargetLanguage==nil||p.ExpectedGlossaryGeneration==nil{return EnqueueGlossaryCommandResult{},platform.NewError(platform.CodeForbidden,false,"术语提议未经完整人工批准")}
 termID:=platform.NewIdentifier();commandID:=platform.NewIdentifier()
 approved:=map[string]any{"proposalId":p.ProposalId,"approvedBy":*p.ApprovedBy,"approvalRevision":p.ApprovalRevision,"proposalHash":p.PayloadHash,"expectedGlossaryGeneration":*p.ExpectedGlossaryGeneration,"term":map[string]any{"termId":termID,"version":0,"sourceLanguage":*p.SourceLanguage,"targetLanguage":*p.TargetLanguage,"sourceTerm":*p.SourceTerm,"targetTerm":*p.TargetTerm,"scope":"global","generation":*p.ExpectedGlossaryGeneration,"source":"manual"}}
 payload:=map[string]any{"proposal":approved,"idempotencyKey":p.ProposalId};raw,err:=platform.CanonicalJSON(payload);if err!=nil{return EnqueueGlossaryCommandResult{},platform.NewError(platform.CodeInternal,false,"术语批准命令编码失败")}
 _,err=tx.ExecContext(ctx,`INSERT INTO proposal_commands(site_id,command_id,proposal_id,payload_hash,state,command_json) VALUES(?,?,?,?,'pending',?)`,op.Actor.SiteID,commandID,p.ProposalId,p.PayloadHash,raw);if err!=nil{return EnqueueGlossaryCommandResult{},memoryruntime.DBError(err)}
 p.State="applying";if err=SaveProposal(ctx,tx,op,p);err!=nil{return EnqueueGlossaryCommandResult{},err};return EnqueueGlossaryCommandResult{CommandId:commandID,Proposal:p},nil
}
