package source

import (
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/chunk"
 "nice-wiki/modules/knowledge/internal/gen"
 modelruntime "nice-wiki/modules/knowledge/internal/model"
)

// BuildMaterialBlocks 对已持久的完整机器原文视图按真实NFKC稳定边界续读，原页/描述路径不变。
func BuildMaterialBlocks(ctx context.Context,op OperationContext,leaf SourceLeaf,asset *gen.AssetRef,modelID string,space gen.EmbeddingSpace)([]chunk.SourceBlock,error) {
 result:=[]chunk.SourceBlock{};start:=int64(0)
 if leaf.Text=="" {return result,nil}
 for {
  normalized,err:=chunk.NormalizeSource(ctx,op,chunk.NormalizeSourceInput{Source:leaf.Text,PreprocessingVersion:space.PreprocessingVersion,StartByte:start});if err!=nil {return nil,err}
  tokenized,err:=modelruntime.TokenizeSource(ctx,op,modelruntime.TokenizeSourceInput{ModelID:modelID,Space:space,Text:normalized.Normalized});if err!=nil {return nil,err}
  offsets:=make([]int,len(normalized.SourceOffsets));for i,value:=range normalized.SourceOffsets {offsets[i]=leaf.BlockStart+int(value)}
  binding,_:=platform.CanonicalJSON([]any{leaf.Block,leaf.Path,space.PreprocessingVersion})
  result=append(result,chunk.SourceBlock{Block:leaf.Block,Normalized:normalized.Normalized,SourceOffsets:offsets,SourceByteStart:leaf.BlockStart+int(normalized.SourceStartByte),SourceByteEnd:leaf.BlockStart+int(normalized.SourceByteEnd),SourcePath:leaf.Path,TokenEnds:tokenized.TokenEnds,ContextVersionHash:platform.HashBytes(binding),Asset:asset})
  if normalized.NextByte==nil {return result,nil};start=*normalized.NextByte
 }
}
