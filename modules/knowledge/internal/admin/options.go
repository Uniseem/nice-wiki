package admin

import (
 "context"
 "encoding/json"
 "errors"
 "slices"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/gen"
 indexer "nice-wiki/modules/knowledge/internal/index"
)

type ReadIndexOptionsInput struct{}
type IndexProfileOption struct { ProfileID string `json:"profileId"`;Tier string `json:"tier"`;CostClass string `json:"costClass"`;Label string `json:"label"`;ModelID string `json:"modelId,omitempty"`;Space *gen.EmbeddingSpace `json:"space,omitempty"`;Usable bool `json:"usable"`;ReasonCode string `json:"reasonCode,omitempty"` }
type IndexDocumentOption struct { DocumentID string `json:"documentId"`;Title string `json:"title"`;Reference gen.PublishedRevisionRef `json:"reference"`;VisibilityRevision int `json:"visibilityRevision"` }
type ReadIndexOptionsResult struct { BaseLanguage string `json:"baseLanguage"`;ModelsState string `json:"modelsState"`;DocumentsState string `json:"documentsState"`;ModelErrorCode string `json:"modelErrorCode,omitempty"`;DocumentErrorCode string `json:"documentErrorCode,omitempty"`;Profiles []IndexProfileOption `json:"profiles"`;Documents []IndexDocumentOption `json:"documents"`;TreeRevision *int `json:"treeRevision,omitempty"` }

// ReadIndexOptions 为现有管理选择框提供真实安全声明，不把跨域故障伪装成空目录。
func ReadIndexOptions(ctx context.Context,op OperationContext,input ReadIndexOptionsInput)(ReadIndexOptionsResult,error) {
 if err:=AuthorizeAdmin(op);err!=nil { return ReadIndexOptionsResult{},err }
 runtime,err:=indexer.RuntimeFromContext(ctx)
 if err!=nil { return ReadIndexOptionsResult{},err }
 if runtime.BaseLanguage=="" { return ReadIndexOptionsResult{},platform.NewError(platform.CodeDependencyUnavailable,false,"站点基准语言未装配") }
 result:=ReadIndexOptionsResult{BaseLanguage:runtime.BaseLanguage,ModelsState:"unavailable",DocumentsState:"unavailable",Profiles:[]IndexProfileOption{},Documents:[]IndexDocumentOption{}}
 catalog,err:=indexer.ReadModelCatalog(ctx,op,runtime)
 if err!=nil { result.ModelErrorCode=optionErrorCode(err) } else {
  result.ModelsState="ready"
  for _,profile:=range catalog.Profiles {
   if profile.Tier!="embedding"&&profile.Tier!="rerank"&&profile.Tier!="fast" {continue}
   option:=IndexProfileOption{ProfileID:profile.ProfileID,Tier:profile.Tier,CostClass:profile.CostClass,Label:profile.Tier,ReasonCode:platform.CodeNotFound}
   if len(profile.Models)>0 {
    for _,model:=range catalog.Models {
     if model.ModelID!=profile.Models[0] {continue}
     option.ModelID=model.ModelID;option.Label=model.ModelVersion;option.Space=model.EmbeddingSpace
     capability:="chat";if profile.Tier=="embedding" {capability="embedding"};if profile.Tier=="rerank" {capability="rerank"}
     option.Usable=model.Enabled&&slices.Contains(model.Capabilities,capability)
     if profile.Tier=="embedding" { option.Usable=option.Usable&&model.EmbeddingSpace!=nil }
     if option.Usable { option.ReasonCode="" } else { option.ReasonCode=platform.CodeUnsupportedCapability }
     if profile.Tier=="embedding"&&option.Space!=nil { switch option.Space.PreprocessingVersion {case "none;tiktoken-go@v0.7.0;cl100k_base","none;tiktoken-go@v0.7.0;o200k_base":default:option.Usable=false;option.ReasonCode=platform.CodeUnsupportedSchema} }
     break
    }
   }
   result.Profiles=append(result.Profiles,option)
  }
 }
 if runtime.Content==nil||runtime.Validators["ContentReadPublicTreeResult"]==nil { result.DocumentErrorCode=platform.CodeDependencyUnavailable;return result,nil }
 service:=op;service.CallerService="knowledge";service.Actor=platform.AuthContext{SubjectID:"knowledge",Kind:"service",SiteID:op.Actor.SiteID,ExpiresAt:op.Deadline,Grants:[]string{}}
 raw,err:=runtime.Content.ReadPublicTree(platform.WithOperationContext(platform.WithPortalProof(ctx,""),service))
 if err!=nil { result.DocumentErrorCode=optionErrorCode(err);return result,nil }
 if err=runtime.Validators["ContentReadPublicTreeResult"](raw);err!=nil { result.DocumentErrorCode=optionErrorCode(err);return result,nil }
 var tree struct { Nodes []struct{DocumentID string `json:"documentId"`;Title string `json:"title"`;Reference gen.PublishedRevisionRef `json:"reference"`;VisibilityRevision int `json:"visibilityRevision"`} `json:"nodes"`;TreeRevision int `json:"treeRevision"` }
 if json.Unmarshal(raw,&tree)!=nil { result.DocumentErrorCode=platform.CodeIntegrityError;return result,nil }
 for _,node:=range tree.Nodes {
  if node.Reference.DataEpoch!=op.DataEpoch||node.Reference.DocumentId!=node.DocumentID { result.DocumentErrorCode=platform.CodeStaleResult;return result,nil }
 }
 for _,node:=range tree.Nodes { result.Documents=append(result.Documents,IndexDocumentOption{node.DocumentID,node.Title,node.Reference,node.VisibilityRevision}) }
 result.TreeRevision=&tree.TreeRevision;result.DocumentsState="ready"
 return result,nil
}
func optionErrorCode(err error)string { var failure *platform.Error;if errors.As(err,&failure) { return failure.Code };return platform.CodeDependencyUnavailable }
