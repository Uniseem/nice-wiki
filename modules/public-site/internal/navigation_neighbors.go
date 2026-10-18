package publicsite

import (
 "context"
 "nice-wiki/libs/platform"
)

type BuildPreviousNextInput struct { DocumentID string; Tree []NavigationNode }
type BuildPreviousNextResult struct { Previous []Breadcrumb; Next []Breadcrumb }

// BuildPreviousNext 使用同一公开树的稳定深度优先顺序。
func BuildPreviousNext(ctx context.Context, op platform.OperationContext, in BuildPreviousNextInput) (BuildPreviousNextResult,error) {
 nodes,err:=navigationOrder(in.Tree);if err!=nil{return BuildPreviousNextResult{},err}
 result:=BuildPreviousNextResult{Previous:[]Breadcrumb{},Next:[]Breadcrumb{}}
 for i,node:=range nodes {if node.DocumentID!=in.DocumentID{continue};if i>0{previous:=nodes[i-1];result.Previous=append(result.Previous,Breadcrumb{previous.Title,previous.CanonicalPath})};if i+1<len(nodes){next:=nodes[i+1];result.Next=append(result.Next,Breadcrumb{next.Title,next.CanonicalPath})};return result,nil}
 return BuildPreviousNextResult{},platform.NewError(platform.CodeNotFound,false,"NODE_NOT_FOUND")
}
