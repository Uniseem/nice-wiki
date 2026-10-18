package extract

import (
 "bytes"
 "context"
 "encoding/json"
 "io"
 "strings"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/lifecycle"
 "nice-wiki/modules/knowledge/internal/source"
)

type ExtractPdfTextInput struct { ExtractionID string `json:"extractionId"`;Asset platform.AssetRef `json:"asset"`;Source source.AssetSource `json:"source"`;LeaseID string `json:"leaseId"` }
type ExtractPdfTextResult struct { Extraction Extraction `json:"extraction"`;Pages []PDFPage `json:"pages"` }

// ExtractPdfText 的独立提取租约由执行器拥有；此函数无业务写入或模型调用。
func ExtractPdfText(ctx context.Context,op OperationContext,input ExtractPdfTextInput)(ExtractPdfTextResult,error) {
 origin:=input.Source.Asset
 if origin.AssetID!=input.Asset.AssetID||origin.SHA256!=input.Asset.SHA256||origin.ByteSize!=input.Asset.ByteSize||origin.MediaType!=input.Asset.MediaType||origin.MediaType!="application/pdf" {return ExtractPdfTextResult{},platform.NewError(platform.CodeIntegrityError,false,"PDF提取来源与原资源不一致")}
 raw,err:=DownloadExtractionAsset(ctx,op,input.Source);if err!=nil {return ExtractPdfTextResult{},err}
 parsed,err:=lifecycle.RunBoundedParser(ctx,"pdf-text-v1",raw);if err!=nil {return ExtractPdfTextResult{},err}
 var result PDFText;decoder:=json.NewDecoder(bytes.NewReader(parsed));decoder.DisallowUnknownFields()
 if decoder.Decode(&result)!=nil||decoder.Decode(&struct{}{})!=io.EOF||result.PageCount!=len(result.Pages)||result.PageCount>500 {return ExtractPdfTextResult{},platform.NewError(platform.CodeIntegrityError,false,"隔离PDF提取结果结构不一致")}
 var text strings.Builder
 for i,page:=range result.Pages {if page.Page!=i+1 {return ExtractPdfTextResult{},platform.NewError(platform.CodeIntegrityError,false,"隔离PDF原页号不一致")};if i>0 {text.WriteString("\n\n")};text.WriteString(page.Text)}
 value:=text.String()
 if len(value)>maximumPDFTextBytes {return ExtractPdfTextResult{},platform.NewError(platform.CodeValidation,false,"PDF正文超过8MiB提取预算")}
 extraction:=Extraction{ExtractionID:input.ExtractionID,Asset:&input.Asset,SourceBlock:&input.Source.Block,ExtractorVersion:PDFExtractorVersion,State:"unsupported",PageCount:result.PageCount}
 hasText:=strings.TrimSpace(value)!=""
 if hasText==result.Unsupported {return ExtractPdfTextResult{},platform.NewError(platform.CodeIntegrityError,false,"隔离PDF正文与unsupported声明不一致")}
 if hasText {hash:=platform.HashBytes([]byte(value));extraction.State="ready";extraction.Text=&value;extraction.ContentHash=&hash}
 return ExtractPdfTextResult{Extraction:extraction,Pages:result.Pages},nil
}

// RunPDFParserChild 必须在任何服务配置、数据库或密钥加载之前由main调用。
func RunPDFParserChild()(bool,error) {
 return lifecycle.RunParserChild("pdf-text-v1",func(raw []byte)([]byte,error){parsed,err:=ParsePDFPages(raw);if err!=nil {return nil,err};return json.Marshal(parsed)})
}
