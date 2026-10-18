package extract

import (
 "context"
 "encoding/json"
 "strings"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/gen"
 "nice-wiki/modules/knowledge/internal/source"
)

type ExtractVideoMetadataInput struct { ExtractionID string `json:"extractionId"`;Document gen.PublishedRevisionRef `json:"document"`;Source string `json:"source"`;BlockID string `json:"blockId"` }
type VideoMetadata struct { Title string `json:"title"`;Description string `json:"description"`;SourceKind string `json:"sourceKind"` }
type ExtractVideoMetadataResult struct { Extraction Extraction `json:"extraction"`;Metadata VideoMetadata `json:"metadata"`;TranscriptState string `json:"transcriptState"` }

// ExtractVideoMetadata 只读已核固定发布正文，metadata就绪不表示存在字幕，不重抓平台URL。
func ExtractVideoMetadata(ctx context.Context,op OperationContext,input ExtractVideoMetadataInput)(ExtractVideoMetadataResult,error) {
 canonical,err:=platform.CanonicalJSON(json.RawMessage(input.Source));if err!=nil {return ExtractVideoMetadataResult{},err}
 if len(canonical)>16<<20||input.Document.DataEpoch!=op.DataEpoch||platform.HashBytes(canonical)!=input.Document.ContentHash {return ExtractVideoMetadataResult{},platform.NewError(platform.CodeIntegrityError,false,"视频元数据不属于固定发布正文")}
 raw,found,err:=FindVideoBlock(canonical,input.BlockID);if err!=nil {return ExtractVideoMetadataResult{},err};if !found {return ExtractVideoMetadataResult{},platform.NewError(platform.CodeNotFound,false,"固定正文不存在原视频块")}
 var node struct { Attrs struct { Title string `json:"title"`;Description string `json:"description"`;Source struct {Kind string `json:"kind"`;Asset source.AssetVersionRef `json:"asset"`} `json:"source"` } `json:"attrs"` }
 if json.Unmarshal(raw,&node)!=nil {return ExtractVideoMetadataResult{},platform.NewError(platform.CodeIntegrityError,false,"视频源节点不可读")}
 if node.Attrs.Source.Kind!="uploaded"&&node.Attrs.Source.Kind!="platform" {return ExtractVideoMetadataResult{},platform.NewError(platform.CodeUnsupportedSchema,false,"视频来源种类不支持")}
 block:=gen.BlockRef{Document:input.Document,BlockId:input.BlockID,BlockHash:platform.HashBytes(raw)}
 extraction:=Extraction{ExtractionID:input.ExtractionID,SourceBlock:&block,ExtractorVersion:"video-published-metadata-v1",State:"unsupported",PageCount:0}
 if node.Attrs.Source.Kind=="uploaded" {ref:=node.Attrs.Source.Asset;extraction.Asset=&platform.AssetRef{AssetID:ref.AssetID,SHA256:ref.SHA256,MediaType:ref.MediaType,ByteSize:ref.ByteSize}}
 value:=strings.TrimSpace(node.Attrs.Title+"\n"+node.Attrs.Description)
 if value!="" {hash:=platform.HashBytes([]byte(value));extraction.State="ready";extraction.Text=&value;extraction.ContentHash=&hash}
 return ExtractVideoMetadataResult{Extraction:extraction,Metadata:VideoMetadata{node.Attrs.Title,node.Attrs.Description,node.Attrs.Source.Kind},TranscriptState:"unsupported"},nil
}

func FindVideoBlock(raw json.RawMessage,blockID string)(json.RawMessage,bool,error) {
 var node struct { Type string `json:"type"`;Attrs struct {BlockID string `json:"blockId"`} `json:"attrs"`;Content []json.RawMessage `json:"content"` }
 if json.Unmarshal(raw,&node)!=nil {return nil,false,platform.NewError(platform.CodeIntegrityError,false,"视频正文节点不可读")}
 if node.Type=="video"&&node.Attrs.BlockID==blockID {canonical,err:=platform.CanonicalJSON(raw);return canonical,true,err}
 for _,child:=range node.Content {found,ok,err:=FindVideoBlock(child,blockID);if err!=nil||ok {return found,ok,err}}
 return nil,false,nil
}
