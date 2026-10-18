package provider

import (
 "context"
 "bytes"
 "encoding/json"
 "io"
 "mime"
 "net/http"
 "net/url"
 "strconv"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/model-gateway/internal/gen"
 gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

// ServiceProviderContext只为media服务读取使用MG身份，不改本域原付费归属。
func ServiceProviderContext(ctx context.Context,op OperationContext)context.Context{op.CallerService="model-gateway";op.Actor=platform.AuthContext{Kind:"service",SubjectID:"model-gateway",SiteID:op.Actor.SiteID,Grants:[]string{},ExpiresAt:op.Deadline};return platform.WithOperationContext(platform.WithPortalProof(ctx,""),op)}
func AuthorizeImages(ctx context.Context,op OperationContext,messages []gen.GatewayMessage)(map[string]int,error){
 versions:=make(map[string]int);options,err:=gatewayruntime.ExecutionDependencies();if err!=nil{return nil,err}
 for _,message:=range messages{if message.Parts==nil{continue};for _,part:=range *message.Parts{if part.Kind=="text"{continue};asset,err:=MediaPartAsset(part);if err!=nil{return nil,err};key:=ImageKey(asset);if _,exists:=versions[key];exists{continue};if options.Media==nil||options.Validators["MediaReadAssetResult"]==nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"视觉资源权威未装配")};raw,err:=options.Media.ReadAsset(ServiceProviderContext(ctx,op),asset.AssetId);if err!=nil{return nil,err}
  var response struct{Asset struct{Ref struct{AssetID string `json:"assetId"`;Version int `json:"version"`;Hash string `json:"sha256"`;MediaType string `json:"mediaType"`;ByteSize int `json:"byteSize"`} `json:"ref"`;SiteID string `json:"siteId"`;State string `json:"state"`} `json:"asset"`}
  if options.Validators["MediaReadAssetResult"](raw)!=nil||json.Unmarshal(raw,&response)!=nil{return nil,platform.NewError(platform.CodeIntegrityError,false,"视觉资源元数据不符合权威合同")};ref:=response.Asset.Ref
  if response.Asset.SiteID!=op.Actor.SiteID||response.Asset.State!="ready"||ref.AssetID!=asset.AssetId||ref.Hash!=asset.Sha256||ref.MediaType!=asset.MediaType||ref.ByteSize!=asset.ByteSize||ref.Version<1{return nil,platform.NewError(platform.CodeStaleResult,false,"视觉资源内容、身份或授权已变化")};versions[key]=ref.Version
 }}
 return versions,nil
}
func LoadImages(ctx context.Context,op OperationContext,messages []gen.GatewayMessage,versions map[string]int)(map[string][]byte,error){
 result:=make(map[string][]byte);options,err:=gatewayruntime.ExecutionDependencies();if err!=nil{return nil,err};total:=0
 for _,message:=range messages{if message.Parts==nil{continue};for _,part:=range *message.Parts{if part.Kind=="text"{continue};asset,err:=MediaPartAsset(part);if err!=nil{return nil,err};key:=ImageKey(asset);if _,exists:=result[key];exists{continue};version,exists:=versions[key];if !exists{return nil,platform.NewError(platform.CodeIntegrityError,false,"视觉资源没有accepted时固定的授权版本")};if asset.ByteSize<1||asset.ByteSize>24<<20||total+asset.ByteSize>24<<20{return nil,platform.NewError(platform.CodeBudgetExceeded,false,"内联视觉资源超过32MiB请求编码预算")};if options.MediaBytes==nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"视觉资源字节权威未装配")}
  response,err:=options.MediaBytes.ReadAssetBytes(ServiceProviderContext(ctx,op),asset.AssetId,url.Values{"version":{strconv.Itoa(version)}});if err!=nil{return nil,err};data,err:=ReadImageBytes(response,asset);if err!=nil{return nil,err};result[key]=data;total+=len(data)
 }}
 return result,nil
}
func ReadImageBytes(response *http.Response,asset gen.AssetRef)([]byte,error){
 if response==nil||response.Body==nil{return nil,platform.NewError(platform.CodeIntegrityError,false,"视觉资源响应缺少正文")};defer response.Body.Close()
 if response.StatusCode!=http.StatusOK{return nil,platform.NewError(platform.CodeStaleResult,false,"视觉资源未返回完整授权字节")};mediaType,_,err:=mime.ParseMediaType(response.Header.Get("Content-Type"));if err!=nil||mediaType!=asset.MediaType{return nil,platform.NewError(platform.CodeIntegrityError,false,"视觉资源实际MIME不一致")}
 data,err:=io.ReadAll(io.LimitReader(response.Body,int64(asset.ByteSize)+1));if err!=nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"视觉资源字节读取未完成")};if len(data)!=asset.ByteSize||platform.HashBytes(data)!=asset.Sha256{return nil,platform.NewError(platform.CodeIntegrityError,false,"视觉资源实际字节/hash不一致")};if asset.MediaType=="application/pdf"&&!bytes.HasPrefix(data,[]byte("%PDF-")){return nil,platform.NewError(platform.CodeIntegrityError,false,"原生PDF字节头不符合MIME，不能伪装其他附件")};return data,nil
}

// MediaPartAsset仅接受真实图像或原生PDF引用，不将附件伪装成图像。
func MediaPartAsset(part gen.GatewayPart)(gen.AssetRef,error){
 if part.Kind=="image"&&part.Image!=nil{return *part.Image,nil}
 if part.Kind=="file"&&part.File!=nil{if part.File.MediaType!="application/pdf"{return gen.AssetRef{},platform.NewError(platform.CodeUnsupportedCapability,false,"当前原生文件仅支持application/pdf")};return *part.File,nil}
 return gen.AssetRef{},platform.NewError(platform.CodeIntegrityError,false,"媒体片段缺少对应的唯一固定资源引用")
}
