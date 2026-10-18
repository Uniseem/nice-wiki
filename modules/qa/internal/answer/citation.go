package answer

import(
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/qa/internal/gen"
 qaruntime "nice-wiki/modules/qa/internal/runtime"
)
type ValidateCitationInput struct{CitationID string `json:"citationId"`;Allowed []gen.Citation `json:"allowed"`;Language string `json:"language"`}
type ValidateCitationResult struct{Citation gen.Citation `json:"citation"`}

func ValidateCitation(ctx context.Context,op OperationContext,input ValidateCitationInput)(ValidateCitationResult,error){
 var selected *gen.Citation
 for _,citation:=range input.Allowed{if citation.CitationId==input.CitationID{if selected!=nil{return ValidateCitationResult{},platform.NewError(platform.CodeIntegrityError,false,"证据白名单含重复引用标识")};copy:=citation;selected=&copy}}
 if selected==nil{return ValidateCitationResult{},platform.NewError(platform.CodeStaleResult,false,"引用不属于本轮证据白名单")}
 result,err:=qaruntime.ValidateCitations(ctx,op,[]gen.Citation{*selected},input.Language);if err!=nil{return ValidateCitationResult{},err}
 return ValidateCitationResult{Citation:result[0]},nil
}
