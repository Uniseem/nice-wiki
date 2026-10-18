package extract

import (
 "context"
 "strings"
 "unicode/utf8"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/source"
)

type DescribeAttachmentInput struct { ExtractionID string `json:"extractionId"`;Asset platform.AssetRef `json:"asset"`;Source source.AssetSource `json:"source"`;SourceText string `json:"sourceText,omitempty"`;Memory platform.MemoryContext `json:"memory"`;LeaseID string `json:"leaseId"` }
type DescribeAttachmentResult struct { Extraction Extraction `json:"extraction"` }

// DescribeAttachment PDF原生文件分支只用于已批准的附件任务，不作为无文字PDF的自动OCR回退。
func DescribeAttachment(ctx context.Context,op OperationContext,input DescribeAttachmentInput)(DescribeAttachmentResult,error) {
 if !SameAsset(input.Asset,input.Source) {return DescribeAttachmentResult{},platform.NewError(platform.CodeIntegrityError,false,"附件原资源与授权版本不一致")}
 raw,err:=DownloadExtractionAsset(ctx,op,input.Source);if err!=nil {return DescribeAttachmentResult{},err}
 task,tier,capability:="knowledge.describe","fast",platform.CapabilityChat
 var parts []platform.GatewayPart
 text:=input.SourceText
 switch input.Asset.MediaType {
 case "text/plain","text/markdown","text/csv","application/json":
  if !utf8.Valid(raw)||strings.ContainsRune(string(raw),0) {return DescribeAttachmentResult{},platform.NewError(platform.CodeUnsupportedSchema,false,"文本附件编码不支持")}
  original:=string(raw)
  if text!=""&&text!=original {return DescribeAttachmentResult{},platform.NewError(platform.CodeIntegrityError,false,"文本附件来源不完整或被替换")};text=original
 case "application/pdf":
  if text=="" {task="knowledge.extract-attachment";tier="vision";capability=platform.CapabilityVision;parts=[]platform.GatewayPart{{Kind:"file",File:&input.Asset}}}
 default:return DescribeAttachmentResult{},platform.NewError(platform.CodeUnsupportedCapability,false,"模型未声明此附件格式的原生受控输入")
 }
 if text!=""&&utf8.RuneCountInString(text)>65536 {return DescribeAttachmentResult{},platform.NewError(platform.CodeValidation,false,"完整附件文字超过模型消息预算")}
 limit:=int64(800);if task=="knowledge.extract-attachment" {limit=1500}
 extraction,err:=DescribeResource(ctx,op,input.ExtractionID,input.Source,input.Memory,task,tier,capability,text,parts,limit)
 if err!=nil {return DescribeAttachmentResult{},err};return DescribeAttachmentResult{Extraction:extraction},nil
}
