package extract

import (
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/source"
)

type ExtractImageDescriptionInput struct { ExtractionID string `json:"extractionId"`;Asset platform.AssetRef `json:"asset"`;Source source.AssetSource `json:"source"`;Memory platform.MemoryContext `json:"memory"`;LeaseID string `json:"leaseId"` }
type ExtractImageDescriptionResult struct { Extraction Extraction `json:"extraction"` }

// ExtractImageDescription 只接已发布任务批准的真实视觉资源，机器描述不覆盖caption。
func ExtractImageDescription(ctx context.Context,op OperationContext,input ExtractImageDescriptionInput)(ExtractImageDescriptionResult,error) {
 if !SameAsset(input.Asset,input.Source) {return ExtractImageDescriptionResult{},platform.NewError(platform.CodeIntegrityError,false,"视觉资源与授权版本不一致")}
 switch input.Asset.MediaType {case "image/png","image/jpeg","image/webp","image/gif":default:return ExtractImageDescriptionResult{},platform.NewError(platform.CodeUnsupportedCapability,false,"该资源没有声明的原生视觉输入格式")}
 if _,err:=DownloadExtractionAsset(ctx,op,input.Source);err!=nil {return ExtractImageDescriptionResult{},err}
 extraction,err:=DescribeResource(ctx,op,input.ExtractionID,input.Source,input.Memory,"knowledge.describe-visual","vision",platform.CapabilityVision,"",[]platform.GatewayPart{{Kind:"image",Image:&input.Asset}},800)
 if err!=nil {return ExtractImageDescriptionResult{},err};return ExtractImageDescriptionResult{Extraction:extraction},nil
}
