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
	Block gen.BlockRef `json:"block"`
	Normalized string `json:"normalized"`
	SourceOffsets []int `json:"sourceOffsets"`
	SourceByteEnd int `json:"sourceByteEnd"`
	TokenEnds []int `json:"tokenEnds"`
	ContextVersionHash string `json:"contextVersionHash"`
	Asset *gen.AssetRef `json:"asset,omitempty"`
}
type SplitChunksInput struct {
	BuildID string `json:"buildId"`
	Document gen.PublishedRevisionRef `json:"document"`
	Blocks []SourceBlock `json:"blocks"`
	ChunkTokens int `json:"chunkTokens"`
	OverlapTokens int `json:"overlapTokens"`
}
type SplitChunksResult struct { Chunks []gen.Chunk `json:"chunks"` }

// SplitChunks 不跨源块或权限合并；长块按模型 token 边界切分并完整保留来源映射。
func SplitChunks(ctx context.Context, op OperationContext, input SplitChunksInput) (SplitChunksResult, error) {
	if input.ChunkTokens < 128 || input.ChunkTokens > 1024 || input.OverlapTokens < 0 || input.OverlapTokens > 128 || input.OverlapTokens >= input.ChunkTokens { return SplitChunksResult{},platform.NewError(platform.CodeValidation,false,"分块或重叠预算不合法") }
	result:=SplitChunksResult{Chunks:make([]gen.Chunk,0)}
	seen:=make(map[string]bool)
	for _,source:=range input.Blocks {
		if source.Block.Document!=input.Document || seen[source.Block.BlockId] || len(source.SourceOffsets)!=len(source.Normalized) || !utf8.ValidString(source.Normalized) { return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"源块身份或偏移映射不一致") }
		seen[source.Block.BlockId]=true
		if source.Normalized=="" { if len(source.TokenEnds)!=0 { return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"空源块含分词边界") }; continue }
		last:=0
		for _,end:=range source.TokenEnds {
			if end<=last || end>len(source.Normalized) || (end<len(source.Normalized) && !utf8.RuneStart(source.Normalized[end])) { return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"模型分词边界不合法") }; last=end
		}
		if last!=len(source.Normalized) { return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"模型分词未覆盖全部源文本") }
		for i,offset:=range source.SourceOffsets {
			if offset<0 || offset>=source.SourceByteEnd || (i>0 && offset<source.SourceOffsets[i-1]) { return SplitChunksResult{},platform.NewError(platform.CodeIntegrityError,false,"原文偏移不是有效单调映射") }
		}
		for first:=0;first<len(source.TokenEnds); {
			last:=min(first+input.ChunkTokens,len(source.TokenEnds))
			startByte:=0
			if first>0 { startByte=source.TokenEnds[first-1] }
			endByte:=source.TokenEnds[last-1]
			text:=source.Normalized[startByte:endByte]
			if len(text)>65536 || len(result.Chunks)==256 { return SplitChunksResult{},platform.NewError(platform.CodeValidation,false,"分块数量或文本超过上限") }
			endOffset:=source.SourceByteEnd
			next:=sort.Search(len(source.SourceOffsets),func(i int) bool { return source.SourceOffsets[i]>source.SourceOffsets[endByte-1] })
			if next<len(source.SourceOffsets) { endOffset=source.SourceOffsets[next] }
			identity,_:=platform.CanonicalJSON(struct { Block gen.BlockRef; Asset *gen.AssetRef; Context string; Start,End int; TextHash string }{source.Block,source.Asset,source.ContextVersionHash,startByte,endByte,platform.HashBytes([]byte(text))})
			result.Chunks=append(result.Chunks,gen.Chunk{ChunkId:"chunk."+platform.HashBytes(identity),Document:input.Document,Block:source.Block,Ordinal:len(result.Chunks),ChunkHash:platform.HashBytes([]byte(text)),Text:text,TokenCount:last-first,ByteStart:source.SourceOffsets[startByte],ByteEnd:endOffset,SourceOffsets:append([]int{},source.SourceOffsets[startByte:endByte]...),Asset:source.Asset})
			if last==len(source.TokenEnds) { break }
			first=last-input.OverlapTokens
		}
	}
	return result,nil
}
