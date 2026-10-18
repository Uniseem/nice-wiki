// Package proposal 验证人工审阅候选；验证不修改 active 记忆。
package proposal

import (
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/memory/internal/entry"
 "nice-wiki/modules/memory/internal/gen"
)

type OperationContext=platform.OperationContext
type ValidateProposalInput struct{Proposal gen.MemoryProposal `json:"proposal"`;SourceGeneration int `json:"sourceGeneration"`}
type ValidateProposalResult struct{Valid bool `json:"valid"`;ProposalHash string `json:"proposalHash"`}
func ValidateProposal(ctx context.Context,op OperationContext,input ValidateProposalInput)(ValidateProposalResult,error){
 p:=input.Proposal;bad:=false
 if p.State!="pending"||p.ApprovedBy!=nil||p.ApprovalRevision!=0{bad=true}
 entryFields:=p.EntryId!=nil||p.ExpectedEntryVersion!=nil||p.ProposedTitle!=nil||p.ProposedBody!=nil||p.ProposedLanguage!=nil
 glossaryFields:=p.SourceLanguage!=nil||p.SourceTerm!=nil||p.TargetTerm!=nil||p.TargetLanguage!=nil||p.ExpectedGlossaryGeneration!=nil
 switch p.Kind{
 case "entry_create":bad=bad||glossaryFields||p.EntryId!=nil||p.ExpectedEntryVersion!=nil||p.ProposedTitle==nil||p.ProposedBody==nil||p.ProposedLanguage==nil
 case "entry_update":bad=bad||glossaryFields||p.EntryId==nil||p.ExpectedEntryVersion==nil||p.ProposedTitle==nil||p.ProposedBody==nil||p.ProposedLanguage==nil
 case "entry_archive":bad=bad||glossaryFields||p.EntryId==nil||p.ExpectedEntryVersion==nil||p.ProposedTitle!=nil||p.ProposedBody!=nil||p.ProposedLanguage!=nil
 case "glossary":bad=bad||entryFields||p.SourceLanguage==nil||p.SourceTerm==nil||p.TargetTerm==nil||p.TargetLanguage==nil||p.ExpectedGlossaryGeneration==nil
 default:bad=true
 }
 if bad{return ValidateProposalResult{},platform.NewError(platform.CodeValidation,false,"提议类型与字段集合不匹配")}
 for _,value:=range []*string{p.ProposedTitle,p.ProposedBody,p.SourceTerm,p.TargetTerm}{if value!=nil&&entry.ContainsPrivateMaterial(*value){return ValidateProposalResult{},platform.NewError(platform.CodeValidation,false,"提议包含禁止公开的凭证或敏感标记")}}
 p.PayloadHash="";raw,err:=platform.CanonicalJSON(map[string]any{"proposal":p,"sourceGeneration":input.SourceGeneration});if err!=nil{return ValidateProposalResult{},platform.NewError(platform.CodeInternal,false,"提议摘要编码失败")}
 return ValidateProposalResult{Valid:true,ProposalHash:platform.HashBytes(raw)},nil
}
