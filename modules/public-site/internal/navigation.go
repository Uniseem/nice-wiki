package publicsite

import (
 "context"
 "encoding/json"
 "sort"

 "nice-wiki/libs/platform"
 "nice-wiki/modules/public-site/internal/gen"
)

type NavigationNode struct {
 DocumentID string `json:"documentId"`
 ParentID *string `json:"parentId"`
 Title string `json:"title"`
 CanonicalPath string `json:"canonicalPath"`
 SortKey string `json:"sortKey"`
 Publication gen.PublishedRevisionRef `json:"publication"`
}
type BuildNavigationInput struct { Locale string; DataEpoch string; MaxNodes int64 }
type BuildNavigationResult struct { Tree []NavigationNode `json:"tree"`; Revision int `json:"revision"` }

// BuildNavigation 不使用异步投影授予公开权限，返回前重新核对树水位和每个精确发布引用。
func BuildNavigation(ctx context.Context, op platform.OperationContext, in BuildNavigationInput) (BuildNavigationResult,error) {
 directory,err:=directoryFrom(ctx);if err!=nil{return BuildNavigationResult{},err}
 site,err:=readSiteSnapshot(ctx,directory.SiteConfig);if err!=nil{return BuildNavigationResult{},err}
 languages,err:=readLanguageSnapshot(ctx,directory.Localization);if err!=nil{return BuildNavigationResult{},err}
 if _,ok:=languages.versions[in.Locale];!ok{return BuildNavigationResult{},platform.NewError(platform.CodeValidation,false,"LOCALE_UNSUPPORTED")}
 raw,err:=directory.Content.ReadPublicTree(ctx);if err!=nil{return BuildNavigationResult{},err}
 var tree struct { Nodes []struct {
 DocumentID string `json:"documentId"`; ParentID *string `json:"parentId"`; Title string `json:"title"`; CanonicalPath string `json:"canonicalPath"`; OrderKey string `json:"orderKey"`; Reference gen.PublishedRevisionRef `json:"reference"`; VisibilityRevision int `json:"visibilityRevision"`
 }; TreeRevision int `json:"treeRevision"` }
 if err=json.Unmarshal(raw,&tree);err!=nil||tree.Nodes==nil{return BuildNavigationResult{},dependency("公开树权威不完整")}
 if int64(len(tree.Nodes))>in.MaxNodes{return BuildNavigationResult{},platform.NewError(platform.CodeValidation,false,"NAVIGATION_LIMIT")}
 nodes:=make([]NavigationNode,0,len(tree.Nodes)); refs:=make([]gen.PublishedRevisionRef,0,len(tree.Nodes))
 for _,node:=range tree.Nodes {
 if string(node.Reference.DataEpoch)!=in.DataEpoch||in.DataEpoch!=string(op.DataEpoch){return BuildNavigationResult{},platform.NewError(platform.CodeVersionMismatch,false,"EPOCH_MISMATCH")}
 href,err:=relativeLink(ctx,op,site.publicBaseURL,in.Locale,node.CanonicalPath,"");if err!=nil{return BuildNavigationResult{},err}
 nodes=append(nodes,NavigationNode{node.DocumentID,node.ParentID,node.Title,href,node.OrderKey,node.Reference});refs=append(refs,node.Reference)
 }
 ordered,err:=navigationOrder(nodes);if err!=nil{return BuildNavigationResult{},err}
 for start:=0;start<len(refs);start+=100 {
 end:=start+100;if end>len(refs){end=len(refs)}
 raw,err=directory.Content.ValidatePublications(ctx,map[string]any{"refs":refs[start:end]});if err!=nil{return BuildNavigationResult{},err}
 var result struct { Validations []struct{Reference gen.PublishedRevisionRef `json:"reference"`;Visible bool `json:"visible"`;MatchesCurrent bool `json:"matchesCurrent"`;VisibilityRevision int `json:"visibilityRevision"`} `json:"validations"` }
 if err=json.Unmarshal(raw,&result);err!=nil||len(result.Validations)!=end-start{return BuildNavigationResult{},dependency("公开引用权威不完整")}
 for i,item:=range result.Validations {if !item.Visible||!item.MatchesCurrent||!samePublication(item.Reference,refs[start+i])||item.VisibilityRevision!=tree.Nodes[start+i].VisibilityRevision{return BuildNavigationResult{},stale()}}
 }
 if err=confirmWatermarks(ctx,directory,site,languages);err!=nil{return BuildNavigationResult{},err}
 again,err:=readTree(ctx,directory.Content);if err!=nil{return BuildNavigationResult{},err};if again.revision!=tree.TreeRevision{return BuildNavigationResult{},stale()}
 return BuildNavigationResult{ordered,tree.TreeRevision},nil
}

// navigationOrder 只排序已由 content 过滤的公开节点，不拼接可能缺失祖先的路径。
func navigationOrder(nodes []NavigationNode) ([]NavigationNode,error) {
 byID:=make(map[string]NavigationNode,len(nodes));children:=make(map[string][]NavigationNode)
 for _,node:=range nodes {if _,exists:=byID[node.DocumentID];exists{return nil,platform.NewError(platform.CodeIntegrityError,false,"DUPLICATE_DOCUMENT")};byID[node.DocumentID]=node}
 for _,node:=range nodes {parent:="";if node.ParentID!=nil{parent=*node.ParentID;if _,ok:=byID[parent];!ok{return nil,platform.NewError(platform.CodeIntegrityError,false,"MISSING_PARENT")}};children[parent]=append(children[parent],node)}
 for key:=range children{sort.SliceStable(children[key],func(i,j int)bool{return children[key][i].SortKey<children[key][j].SortKey})}
 result:=make([]NavigationNode,0,len(nodes));seen:=make(map[string]bool)
 var visit func(string)error
 visit=func(parent string)error {for _,node:=range children[parent]{if seen[node.DocumentID]{return platform.NewError(platform.CodeIntegrityError,false,"TREE_CYCLE")};seen[node.DocumentID]=true;result=append(result,node);if err:=visit(node.DocumentID);err!=nil{return err}};return nil}
 if err:=visit("");err!=nil{return nil,err};if len(result)!=len(nodes){return nil,platform.NewError(platform.CodeIntegrityError,false,"TREE_CYCLE")};return result,nil
}
