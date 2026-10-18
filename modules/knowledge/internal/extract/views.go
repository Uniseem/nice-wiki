package extract

import (
 "context"
 "strconv"
 "nice-wiki/libs/platform"
 indexer "nice-wiki/modules/knowledge/internal/index"
 "nice-wiki/modules/knowledge/internal/source"
)

// BindExtractionViews 将真实已付费/免费提取文字绑定当前原发布授权，原结果不覆盖人工正文。
func BindExtractionViews(ctx context.Context,op OperationContext,buildID,leaseID string,origin source.AssetSource,artifact ExtractionArtifact)([]source.SourceLeaf,error) {
 leaves:=[]source.SourceLeaf{}
 if artifact.Extraction.State!="ready" {return leaves,nil}
 if len(artifact.Pages)>0 {
  for _,page:=range artifact.Pages {if page.Text!="" {leaves=append(leaves,source.SourceLeaf{Block:origin.Block,Path:"/extractions/"+artifact.Extraction.ExtractionID+"/pages/"+strconv.Itoa(page.Page),Text:page.Text})}}
 } else if artifact.Extraction.Text!=nil {leaves=append(leaves,source.SourceLeaf{Block:origin.Block,Path:"/extractions/"+artifact.Extraction.ExtractionID+"/text",Text:*artifact.Extraction.Text})}
 work,tx,_,cancel,err:=indexer.BeginBuildWrite(ctx,op,buildID,leaseID);if err!=nil {return nil,err};defer cancel();defer tx.Rollback()
 documentRaw,_:=platform.CanonicalJSON(origin.Block.Document);blockRaw,_:=platform.CanonicalJSON(origin.Block);assetRaw,_:=platform.CanonicalJSON(artifact.Extraction.Asset)
 for _,leaf:=range leaves {
  hash:=platform.HashBytes([]byte(leaf.Text))
  _,err=tx.ExecContext(work,`INSERT INTO extraction_source_views(site_id,data_epoch,publication_json,block_json,asset_json,source_path,source_text,source_hash,source_start) VALUES(?,?,?,?,?,?,?,?,0) ON CONFLICT DO NOTHING`,op.Actor.SiteID,op.DataEpoch,documentRaw,blockRaw,assetRaw,leaf.Path,leaf.Text,hash)
  if err!=nil {return nil,platform.NormalizeIOError(err,platform.CodeInternal,false,"资源原文视图保存失败")}
  var oldHash,oldText string;var start int
  err=tx.QueryRowContext(work,`SELECT source_hash,source_text,source_start FROM extraction_source_views WHERE site_id=? AND data_epoch=? AND publication_json=? AND block_json=? AND asset_json=? AND source_path=?`,op.Actor.SiteID,op.DataEpoch,documentRaw,blockRaw,assetRaw,leaf.Path).Scan(&oldHash,&oldText,&start)
  if err!=nil||oldHash!=hash||oldText!=leaf.Text||start!=0 {return nil,platform.NewError(platform.CodeIntegrityError,false,"原资源证据视图冲突")}
 }
 if err=tx.Commit();err!=nil {return nil,platform.NormalizeIOError(err,platform.CodeInternal,false,"资源证据视图提交失败")};return leaves,nil
}
