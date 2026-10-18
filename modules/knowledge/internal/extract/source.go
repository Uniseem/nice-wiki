// Package extract 生成机器派生检索材料，不覆盖人工正文或资源说明。
package extract

import (
 "context"
 "io"
 "net/http"
 "net/url"
 "strconv"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/gen"
 indexer "nice-wiki/modules/knowledge/internal/index"
 "nice-wiki/modules/knowledge/internal/source"
)

type OperationContext = platform.OperationContext
type Extraction struct { ModelSnapshotHash *string `json:"modelSnapshotHash,omitempty"`; ExtractionID string `json:"extractionId"`;Asset *platform.AssetRef `json:"asset,omitempty"`;SourceBlock *gen.BlockRef `json:"sourceBlock,omitempty"`;ExtractorVersion string `json:"extractorVersion"`;State string `json:"state"`;Text *string `json:"text,omitempty"`;ContentHash *string `json:"contentHash,omitempty"`;PageCount int `json:"pageCount"` }

// DownloadExtractionAsset 只读原发布引用明确授权的精确version；有界读/核hash并关闭流。
func DownloadExtractionAsset(ctx context.Context,op OperationContext,origin source.AssetSource)([]byte,error) {
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return nil,err}
 if runtime.Media==nil {return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"资源权威未装配")}
 ref:=origin.Asset
 if ref.ByteSize<0||ref.ByteSize>50<<20||origin.Block.Document.DataEpoch!=op.DataEpoch {return nil,platform.NewError(platform.CodeValidation,false,"资源版本或50MiB提取预算不合法")}
 service:=op;service.CallerService="knowledge";service.Actor=platform.AuthContext{SubjectID:"knowledge",Kind:"service",SiteID:op.Actor.SiteID,ExpiresAt:op.Deadline,Grants:[]string{}}
 response,err:=runtime.Media.ReadAssetBytes(platform.WithOperationContext(platform.WithPortalProof(ctx,""),service),ref.AssetID,url.Values{"version":{strconv.FormatInt(ref.Version,10)},"publicationDocumentId":{origin.Block.Document.DocumentId},"publishedRevisionId":{origin.Block.Document.PublishedRevisionId}})
 if err!=nil {return nil,err};defer response.Body.Close()
 if response.StatusCode!=http.StatusOK {return nil,platform.NewError(platform.CodeIntegrityError,false,"精确资源读取返回非完整内容")}
 raw,err:=io.ReadAll(io.LimitReader(response.Body,ref.ByteSize+1))
 if err!=nil {return nil,platform.NormalizeIOError(err,platform.CodeDependencyUnavailable,false,"资源授权流读取失败")}
 if int64(len(raw))!=ref.ByteSize||platform.HashBytes(raw)!=ref.SHA256 {return nil,platform.NewError(platform.CodeIntegrityError,false,"资源完整长度或原hash不一致")}
 return raw,nil
}
