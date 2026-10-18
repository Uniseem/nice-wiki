package publicsite

import (
 "context"
 "encoding/json"
 "fmt"
 "nice-wiki/libs/platform"
)

type HandleNavigationInput struct { Locale string; DataEpoch string }
type HandleNavigationResult struct { Status int `json:"status"`; Tree []NavigationNode `json:"tree"`; ETag string `json:"etag"` }

// HandleNavigation 每次都读取权威；HTTP 绑定可在此完成之后处理 If-None-Match。
func HandleNavigation(ctx context.Context,op platform.OperationContext,in HandleNavigationInput)(HandleNavigationResult,error){
 navigation,err:=BuildNavigation(ctx,op,BuildNavigationInput{in.Locale,in.DataEpoch,10000});if err!=nil{return HandleNavigationResult{},err}
 encoded,err:=json.Marshal(navigation);if err!=nil{return HandleNavigationResult{},platform.NewError(platform.CodeInternal,false,"NAVIGATION_ENCODING_FAILED")}
 return HandleNavigationResult{200,navigation.Tree,fmt.Sprintf("\"%s\"",platform.HashBytes(encoded))},nil
}
