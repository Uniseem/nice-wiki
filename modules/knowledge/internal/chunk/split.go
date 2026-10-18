package chunk

import (
	"context"
	"sort"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
)

// SourceBlock 已按固定预处理版本规范化。TokenEnds 由所选嵌入模型的
// tokenizer 提供每个 token 排他 UTF-8 字节终点，不推测模型分词。
type SourceBlock struct {
	Block              gen.BlockRef  `json:"block"`
	Normalized         string        `json:"normalized"`
	SourceOffsets      []int         `json:"sourceOffsets"`
	SourceByteEnd      int           `json:"sourceByteEnd"`
 SourceByteStart int `json:"sourceByteStart"`
 SourcePath string `json:"sourcePath,omitempty"`
	TokenEnds          []int         `json:"tokenEnds"`
	ContextVersionHash string        `json:"contextVersionHash"`
	Asset              *gen.AssetRef `json:"asset,omitempty"`
}
type SplitChunksInput struct {
	BuildID       string                   `json:"buildId"`
	Document      gen.PublishedRevisionRef `json:"document"`
	Blocks        []SourceBlock            `json:"blocks"`
	ChunkTokens   int                      `json:"chunkTokens"`
	OverlapTokens int                      `json:"overlapTokens"`
 Cursor *SplitCursor `json:"cursor,omitempty"`
}
type SplitCursor struct { BlockIndex int `json:"blockIndex"`;TokenIndex int `json:"tokenIndex"`;Ordinal int `json:"ordinal"` }
type SplitChunksResult struct {
	Chunks []gen.Chunk `json:"chunks"`
 NextCursor *SplitCursor `json:"nextCursor,omitempty"`
}

// SplitChunks 不跨源块或权限合并；长块按模型 token 边界切分并完整保留来源映射。
func SplitChunks(ctx context.Context, op OperationContext, input SplitChunksInput) (SplitChunksResult, error) {
	if input.ChunkTokens < 128 || input.ChunkTokens > 1024 || input.OverlapTokens < 0 || input.OverlapTokens > 128 || input.OverlapTokens >= input.ChunkTokens {
		return SplitChunksResult{}, platform.NewError(platform.CodeValidation, false, "分块或重叠预算不合法")
	}
	result := SplitChunksResult{Chunks: make([]gen.Chunk, 0)}
 seen := make(map[string]bool)
 cursor:=SplitCursor{}
 if input.Cursor!=nil { cursor=*input.Cursor }
 if cursor.BlockIndex<0||cursor.BlockIndex>len(input.Blocks)||cursor.TokenIndex<0||cursor.Ordinal<0 { return SplitChunksResult{},platform.NewError(platform.CodeValidation,false,"分块游标不合法") }
 for blockIndex,source:=range input.Blocks {
  identity,_:=platform.CanonicalJSON([]any{source.Block,source.SourcePath,source.SourceByteStart,source.SourceByteEnd})
  key:=platform.HashBytes(identity)
  if source.Block.Document!=input.Document||seen[key]||len(source.SourceOffsets)!=len(source.Normalized)||!utf8.ValidString(source.Normalized)||source.SourceByteStart<0||source.SourceByteStart>source.SourceByteEnd {
   return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"源片段身份或偏移映射不一致")
  };seen[key]=true
  if source.Normalized=="" { if len(source.TokenEnds)!=0 { return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"空源片段含分词边界") };continue }
  end:=0
  for _,next:=range source.TokenEnds {
   // BPE token可切开UTF8码点；只要求真实字节覆盖，不能伪造rune=token。
   if next<=end||next>len(source.Normalized) { return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"模型分词字节边界不合法") };end=next
  }
  if end!=len(source.Normalized) { return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"模型分词未覆盖完整片段") }
  for j,offset:=range source.SourceOffsets {
   if offset<source.SourceByteStart||offset>=source.SourceByteEnd||(j>0&&offset<source.SourceOffsets[j-1]) { return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"原片段偏移不是有效单调映射") }
  }
  if blockIndex<cursor.BlockIndex { continue }
  first:=0
  if blockIndex==cursor.BlockIndex { first=cursor.TokenIndex }
  if first>len(source.TokenEnds) { return SplitChunksResult{},platform.NewError(platform.CodeValidation,false,"分块游标超出原分词范围") }
  for first<len(source.TokenEnds) {
   startByte:=0;if first>0 { startByte=source.TokenEnds[first-1] }
   if startByte<len(source.Normalized)&&!utf8.RuneStart(source.Normalized[startByte]) { return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"分块游标不在完整UTF8边界") }
   last:=min(first+input.ChunkTokens,len(source.TokenEnds))
   for last>first { at:=source.TokenEnds[last-1];if at-startByte<=65536&&(at==len(source.Normalized)||utf8.RuneStart(source.Normalized[at])) {break};last-- }
   if last==first { return SplitChunksResult{},platform.NewError(platform.CodeValidation,false,"模型分词无法在分块字节预算内保持完整UTF8") }
   endByte:=source.TokenEnds[last-1]
   text:=source.Normalized[startByte:endByte]
   endOffset:=source.SourceByteEnd
   next:=sort.Search(len(source.SourceOffsets),func(j int)bool{return source.SourceOffsets[j]>source.SourceOffsets[endByte-1]})
   if next<len(source.SourceOffsets) { endOffset=source.SourceOffsets[next] }
   legacy:=struct{Block gen.BlockRef;Asset *gen.AssetRef;Context string;Start,End int;TextHash string}{source.Block,source.Asset,source.ContextVersionHash,startByte,endByte,platform.HashBytes([]byte(text))}
   encoded,_:=platform.CanonicalJSON(legacy)
   var sourcePath *string
   if source.SourcePath!="" {
    path:=source.SourcePath;sourcePath=&path
    encoded,_=platform.CanonicalJSON(struct{Identity any;SourcePath string;SourceStart,SourceEnd int}{legacy,path,source.SourceByteStart,source.SourceByteEnd})
   }
   result.Chunks=append(result.Chunks,gen.Chunk{ChunkId:"chunk."+platform.HashBytes(encoded),Document:input.Document,Block:source.Block,Ordinal:cursor.Ordinal+len(result.Chunks),ChunkHash:platform.HashBytes([]byte(text)),Text:text,TokenCount:last-first,ByteStart:source.SourceOffsets[startByte],ByteEnd:endOffset,SourceOffsets:append([]int{},source.SourceOffsets[startByte:endByte]...),SourcePath:sourcePath,Asset:source.Asset})
   nextToken:=max(first+1,last-input.OverlapTokens)
   for nextToken<last { at:=source.TokenEnds[nextToken-1];if at==len(source.Normalized)||utf8.RuneStart(source.Normalized[at]) {break};nextToken++ }
   if len(result.Chunks)==256 {
    nextBlock:=blockIndex
    if last==len(source.TokenEnds) { nextBlock++;nextToken=0 }
    if nextBlock<len(input.Blocks) { result.NextCursor=&SplitCursor{BlockIndex:nextBlock,TokenIndex:nextToken,Ordinal:cursor.Ordinal+len(result.Chunks)} }
    return result,nil
   }
   if last==len(source.TokenEnds) {break};first=nextToken
  }
 }
 return result,nil
}
