package search

import (
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/gen"
 indexer "nice-wiki/modules/knowledge/internal/index"
 "nice-wiki/modules/knowledge/internal/source"
)

// ValidateSectionSources 回读本域保存的精确发布原文视图，不取latest，也不把机器背景当引文。
func ValidateSectionSources(ctx context.Context,op OperationContext,chunks []gen.Chunk)error {
 views:=map[string]map[string]source.SourceLeaf{}
 for _,value:=range chunks {
  if value.SourcePath==nil {continue} // 旧无path索引沿原证据映射兼容，不猜新叶路径。
  if value.Asset!=nil {
   leaf,err:=source.ReadExtractionSourceView(ctx,op,value);if err!=nil {return err}
   if err=ValidateSourceRange(value,leaf);err!=nil {return err};continue
  }
  key:=publicationKey(value.Document)
  leaves,found:=views[key]
  if !found {
   runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return err}
   documentRaw,_:=platform.CanonicalJSON(value.Document)
   var body []byte
   err=runtime.DB.QueryRowContext(ctx,`SELECT s.body_json FROM build_sources s JOIN builds b ON b.build_id=s.build_id WHERE b.site_id=? AND b.data_epoch=? AND b.document_json=? ORDER BY b.generation DESC LIMIT 1`,op.Actor.SiteID,op.DataEpoch,documentRaw).Scan(&body)
   if err!=nil {return platform.NormalizeIOError(err,platform.CodeIntegrityError,false,"精确证据源快照不可读")}
   if len(body)>16<<20||platform.HashBytes(body)!=value.Document.ContentHash {return platform.NewError(platform.CodeIntegrityError,false,"证据原文hash不一致")}
   original,err:=source.CollectSourceLeaves(body,value.Document);if err!=nil {return err}
   leaves=map[string]source.SourceLeaf{};for _,leaf:=range original {leaves[leaf.Path]=leaf};views[key]=leaves
  }
  leaf,ok:=leaves[*value.SourcePath]
  if !ok {return platform.NewError(platform.CodeIntegrityError,false,"证据原叶路径不存在")}
  if err:=ValidateSourceRange(value,leaf);err!=nil {return err}
 }
 return nil
}

// ValidateSourceRange 对人工原叶和资源机器原文使用同一真实UTF8区间规则，来源身份仍独立核对。
func ValidateSourceRange(value gen.Chunk,leaf source.SourceLeaf)error {
 if leaf.Block!=value.Block||value.ByteStart<leaf.BlockStart||value.ByteEnd>leaf.BlockStart+len(leaf.Text) {return platform.NewError(platform.CodeIntegrityError,false,"证据原块或UTF8区间与原文不一致")}
 for _,offset:=range value.SourceOffsets {if offset<value.ByteStart||offset>=value.ByteEnd {return platform.NewError(platform.CodeIntegrityError,false,"证据偏移超出真实原叶范围")}}
 return nil
}
