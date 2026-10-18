package source

import (
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/gen"
 indexer "nice-wiki/modules/knowledge/internal/index"
)

// ReadExtractionSourceView 只读同发布/原块/资源hash的不可变机器提取原文；不冒充人工正文。
func ReadExtractionSourceView(ctx context.Context,op OperationContext,value gen.Chunk)(SourceLeaf,error) {
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return SourceLeaf{},err}
 document,_:=platform.CanonicalJSON(value.Document);block,_:=platform.CanonicalJSON(value.Block);asset,_:=platform.CanonicalJSON(value.Asset)
 var text,hash string;var start int
 err=runtime.DB.QueryRowContext(ctx,`SELECT source_text,source_hash,source_start FROM extraction_source_views WHERE site_id=? AND data_epoch=? AND publication_json=? AND block_json=? AND asset_json=? AND source_path=?`,op.Actor.SiteID,op.DataEpoch,document,block,asset,*value.SourcePath).Scan(&text,&hash,&start)
 if err!=nil {return SourceLeaf{},platform.NormalizeIOError(err,platform.CodeNotFound,false,"精确资源提取原文视图不存在")}
 if len(text)>8<<20||start<0||platform.HashBytes([]byte(text))!=hash {return SourceLeaf{},platform.NewError(platform.CodeIntegrityError,false,"资源提取原文视图hash或范围损坏")}
 return SourceLeaf{Block:value.Block,Path:*value.SourcePath,Text:text,BlockStart:start},nil
}
