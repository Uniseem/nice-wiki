package source

import (
 "bytes"
 "context"
 "encoding/json"
 "strconv"
 "unicode/utf8"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/chunk"
 "nice-wiki/modules/knowledge/internal/gen"
 modelruntime "nice-wiki/modules/knowledge/internal/model"
)

type SourceCursor struct { LeafIndex int `json:"leafIndex"`;StartByte int64 `json:"startByte"` }
type BuildSourceBlocksInput struct { Document gen.PublishedRevisionRef `json:"document"`;Source string `json:"source"`;ModelID string `json:"modelId"`;Space gen.EmbeddingSpace `json:"space"`;Cursor *SourceCursor `json:"cursor,omitempty"` }
type BuildSourceBlocksResult struct { Blocks []chunk.SourceBlock `json:"blocks"`;Next *SourceCursor `json:"next,omitempty"` }
type SourceLeaf struct { Block gen.BlockRef;Path string;Text string;BlockStart int }

// BuildSourceBlocks 每次只生成一个真实叶片段，避免NFKC膨胀映射积压整个正文。
// 调用方必须续读Next；相同原块多个叶/范围不生成替代块ID。
func BuildSourceBlocks(ctx context.Context,op OperationContext,input BuildSourceBlocksInput)(BuildSourceBlocksResult,error) {
 canonical,err:=platform.CanonicalJSON(json.RawMessage(input.Source))
 if err!=nil {return BuildSourceBlocksResult{},err}
 if len(canonical)>16*1024*1024||input.Document.DataEpoch!=op.DataEpoch||platform.HashBytes(canonical)!=input.Document.ContentHash {return BuildSourceBlocksResult{},platform.NewError(platform.CodeIntegrityError,false,"分词源不属于固定发布修订")}
 leaves,err:=CollectSourceLeaves(canonical,input.Document)
 if err!=nil {return BuildSourceBlocksResult{},err}
 cursor:=SourceCursor{};if input.Cursor!=nil {cursor=*input.Cursor}
 if cursor.LeafIndex<0||cursor.LeafIndex>len(leaves)||cursor.StartByte<0 {return BuildSourceBlocksResult{},platform.NewError(platform.CodeValidation,false,"原叶游标不合法")}
 result:=BuildSourceBlocksResult{Blocks:[]chunk.SourceBlock{}}
 if cursor.LeafIndex==len(leaves) {if cursor.StartByte!=0 {return BuildSourceBlocksResult{},platform.NewError(platform.CodeValidation,false,"结束游标不合法")};return result,nil}
 leaf:=leaves[cursor.LeafIndex]
 normalized,err:=chunk.NormalizeSource(ctx,op,chunk.NormalizeSourceInput{Source:leaf.Text,PreprocessingVersion:input.Space.PreprocessingVersion,StartByte:cursor.StartByte})
 if err!=nil {return BuildSourceBlocksResult{},err}
 tokenized,err:=modelruntime.TokenizeSource(ctx,op,modelruntime.TokenizeSourceInput{ModelID:input.ModelID,Space:input.Space,Text:normalized.Normalized})
 if err!=nil {return BuildSourceBlocksResult{},err}
 offsets:=make([]int,len(normalized.SourceOffsets));for i,offset:=range normalized.SourceOffsets {offsets[i]=leaf.BlockStart+int(offset)}
 contextRaw,_:=platform.CanonicalJSON([]any{input.Document,leaf.Block,input.Space.PreprocessingVersion})
 result.Blocks=append(result.Blocks,chunk.SourceBlock{Block:leaf.Block,Normalized:normalized.Normalized,SourceOffsets:offsets,SourceByteStart:leaf.BlockStart+int(normalized.SourceStartByte),SourceByteEnd:leaf.BlockStart+int(normalized.SourceByteEnd),SourcePath:leaf.Path,TokenEnds:tokenized.TokenEnds,ContextVersionHash:platform.HashBytes(contextRaw)})
 next:=SourceCursor{LeafIndex:cursor.LeafIndex+1}
 if normalized.NextByte!=nil {next=SourceCursor{LeafIndex:cursor.LeafIndex,StartByte:*normalized.NextByte}}
 if next.LeafIndex<len(leaves) {result.Next=&next}
 return result,nil
}

// CollectSourceLeaves 原块文本视图为最近稳定块的text、alt及视频题名/简介按固定顺序直接串联。
// 嵌套稳定块单独起算；sourcePath仍指向实际JSON字符串，offset不混入JSON转义字节。
func CollectSourceLeaves(body []byte,document gen.PublishedRevisionRef)([]SourceLeaf,error) {
 var root map[string]any;decoder:=json.NewDecoder(bytes.NewReader(body));decoder.UseNumber()
 if decoder.Decode(&root)!=nil {return nil,platform.NewError(platform.CodeIntegrityError,false,"源正文树不可读")}
 leaves:=[]SourceLeaf{};positions:=map[string]int{}
 var walk func(map[string]any,string,*gen.BlockRef)error
 appendLeaf:=func(value any,path string,current *gen.BlockRef)error {
  text,ok:=value.(string);if !ok||text=="" {return nil}
  if current==nil||len(text)>4*1024*1024||utf8.RuneCountInString(text)>1<<20 {return platform.NewError(platform.CodeIntegrityError,false,"原叶缺稳定块或超过真实分词片段预算")}
  leaves=append(leaves,SourceLeaf{Block:*current,Path:path,Text:text,BlockStart:positions[current.BlockId]});positions[current.BlockId]+=len(text);return nil
 }
 walk=func(node map[string]any,path string,current *gen.BlockRef)error {
  attrs,_:=node["attrs"].(map[string]any)
  if id,ok:=attrs["blockId"].(string);ok {raw,_:=platform.CanonicalJSON(node);ref:=gen.BlockRef{Document:document,BlockId:id,BlockHash:platform.HashBytes(raw)};current=&ref}
  if err:=appendLeaf(node["text"],path+"/text",current);err!=nil {return err}
  if content,ok:=node["content"].([]any);ok {for i,child:=range content {n,ok:=child.(map[string]any);if !ok {return platform.NewError(platform.CodeIntegrityError,false,"原正文节点不可读")};if err:=walk(n,path+"/content/"+strconv.Itoa(i),current);err!=nil {return err}}}
  if err:=appendLeaf(attrs["alt"],path+"/attrs/alt",current);err!=nil {return err}
  if node["type"]=="video" {
   if err:=appendLeaf(attrs["title"],path+"/attrs/title",current);err!=nil {return err}
   if err:=appendLeaf(attrs["description"],path+"/attrs/description",current);err!=nil {return err}
  };return nil
 }
 if err:=walk(root,"",nil);err!=nil {return nil,err};return leaves,nil
}
