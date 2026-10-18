package provider

import (
 "context"
 "encoding/base64"
 "encoding/json"
 "strings"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/model-gateway/internal/gen"
)
type MapAnthropicRequestInput struct{Messages []gen.GatewayMessage `json:"messages"`;ModelID string `json:"modelId"`;MaximumOutputTokens int `json:"maximumOutputTokens"`;Images map[string][]byte `json:"images,omitempty"`;Stream bool `json:"stream"`}
type MapAnthropicRequestResult struct{Body string `json:"body"`;System string `json:"system"`}

// ImageKey 以已核AssetRef完整身份作内存索引，不接受任意外部URL。
func ImageKey(asset gen.AssetRef)string{raw,_:=platform.CanonicalJSON(asset);return platform.HashBytes(raw)}
func ProviderContent(message gen.GatewayMessage,images map[string][]byte,protocol string)([]map[string]any,error){
 parts:=[]map[string]any{{"type":"text","text":message.Content}}
 if protocol=="gemini"{parts=[]map[string]any{{"text":message.Content}}}
 if message.Parts==nil{return parts,nil}
 for _,part:=range *message.Parts{
  if part.Kind=="text"{if protocol=="gemini"{parts=append(parts,map[string]any{"text":*part.Text})}else{parts=append(parts,map[string]any{"type":"text","text":*part.Text})};continue}
  if part.Image==nil{return nil,platform.NewError(platform.CodeIntegrityError,false,"视觉片段缺少固定资源身份")};data,exists:=images[ImageKey(*part.Image)];if !exists||len(data)!=part.Image.ByteSize||platform.HashBytes(data)!=part.Image.Sha256{return nil,platform.NewError(platform.CodeIntegrityError,false,"视觉资源未经过受控读取或hash核对")}
  encoded:=base64.StdEncoding.EncodeToString(data)
  switch protocol{case "anthropic":parts=append(parts,map[string]any{"type":"image","source":map[string]any{"type":"base64","media_type":part.Image.MediaType,"data":encoded}});case "gemini":parts=append(parts,map[string]any{"inlineData":map[string]any{"mimeType":part.Image.MediaType,"data":encoded}});default:parts=append(parts,map[string]any{"type":"image_url","image_url":map[string]any{"url":"data:"+part.Image.MediaType+";base64,"+encoded}})}
 }
 return parts,nil
}
func MapAnthropicRequest(ctx context.Context,op OperationContext,input MapAnthropicRequestInput)(MapAnthropicRequestResult,error){
 systems:=make([]string,0);messages:=make([]map[string]any,0)
 for _,message:=range input.Messages{if message.Role=="system"{system:=message.Content;if message.Parts!=nil{for _,part:=range *message.Parts{if part.Kind!="text"||part.Text==nil{return MapAnthropicRequestResult{},platform.NewError(platform.CodeUnsupportedSchema,false,"Messages系统段只允许文本")};system+="\n"+*part.Text}};systems=append(systems,system);continue};content,err:=ProviderContent(message,input.Images,"anthropic");if err!=nil{return MapAnthropicRequestResult{},err};messages=append(messages,map[string]any{"role":message.Role,"content":content})}
 system:=strings.Join(systems,"\n\n");body:=map[string]any{"model":input.ModelID,"messages":messages,"max_tokens":input.MaximumOutputTokens,"stream":input.Stream};if system!=""{body["system"]=system};raw,err:=json.Marshal(body);if err!=nil{return MapAnthropicRequestResult{},platform.NewError(platform.CodeIntegrityError,false,"Messages请求无法编码")};return MapAnthropicRequestResult{Body:string(raw),System:system},nil
}
