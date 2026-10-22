package source

import (
	"bytes"
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/chunk"
	"nice-wiki/modules/knowledge/internal/gen"
	modelruntime "nice-wiki/modules/knowledge/internal/model"
	"strconv"
	"unicode/utf8"
)

type SourceCursor struct {
	LeafIndex int   `json:"leafIndex"`
	StartByte int64 `json:"startByte"`
}
type BuildSourceBlocksInput struct {
	Document gen.PublishedRevisionRef `json:"document"`
	Source   string                   `json:"source"`
	ModelID  string                   `json:"modelId"`
	Space    gen.EmbeddingSpace       `json:"space"`
	Cursor   *SourceCursor            `json:"cursor,omitempty"`
}
type BuildSourceBlocksResult struct {
	Blocks []chunk.SourceBlock `json:"blocks"`
	Next   *SourceCursor       `json:"next,omitempty"`
}
type SourceLeaf struct {
	Block      gen.BlockRef
	Path       string
	Text       string
	BlockStart int
}

// BuildSourceBlocks 每次只生成一个真实叶片段，避免NFKC膨胀映射积压整个正文。
// 调用方必须续读Next；相同原块多个叶/范围不生成替代块ID。
func BuildSourceBlocks(ctx context.Context, op OperationContext, input BuildSourceBlocksInput) (BuildSourceBlocksResult, error) {
	canonical, err := platform.CanonicalJSON(json.RawMessage(input.Source))
	if err != nil {
		return BuildSourceBlocksResult{}, err
	}
	if len(canonical) > 16*1024*1024 || input.Document.DataEpoch != op.DataEpoch || platform.HashBytes(canonical) != input.Document.ContentHash {
		return BuildSourceBlocksResult{}, platform.NewError(platform.CodeIntegrityError, false, "分词源不属于固定发布修订")
	}
	leaves, err := CollectSourceLeaves(canonical, input.Document)
	if err != nil {
		return BuildSourceBlocksResult{}, err
	}
	cursor := SourceCursor{}
	if input.Cursor != nil {
		cursor = *input.Cursor
	}
	if cursor.LeafIndex < 0 || cursor.LeafIndex > len(leaves) || cursor.StartByte < 0 {
		return BuildSourceBlocksResult{}, platform.NewError(platform.CodeValidation, false, "原叶游标不合法")
	}
	result := BuildSourceBlocksResult{Blocks: []chunk.SourceBlock{}}
	if cursor.LeafIndex == len(leaves) {
		if cursor.StartByte != 0 {
			return BuildSourceBlocksResult{}, platform.NewError(platform.CodeValidation, false, "结束游标不合法")
		}
		return result, nil
	}
	leaf := leaves[cursor.LeafIndex]
	normalized, err := chunk.NormalizeSource(ctx, op, chunk.NormalizeSourceInput{Source: leaf.Text, PreprocessingVersion: input.Space.PreprocessingVersion, StartByte: cursor.StartByte})
	if err != nil {
		return BuildSourceBlocksResult{}, err
	}
	tokenized, err := modelruntime.TokenizeSource(ctx, op, modelruntime.TokenizeSourceInput{ModelID: input.ModelID, Space: input.Space, Text: normalized.Normalized})
	if err != nil {
		return BuildSourceBlocksResult{}, err
	}
	offsets := make([]int, len(normalized.SourceOffsets))
	for i, offset := range normalized.SourceOffsets {
		offsets[i] = leaf.BlockStart + int(offset)
	}
	contextRaw, _ := platform.CanonicalJSON([]any{input.Document, leaf.Block, input.Space.PreprocessingVersion})
	result.Blocks = append(result.Blocks, chunk.SourceBlock{Block: leaf.Block, Normalized: normalized.Normalized, SourceOffsets: offsets, SourceByteStart: leaf.BlockStart + int(normalized.SourceStartByte), SourceByteEnd: leaf.BlockStart + int(normalized.SourceByteEnd), SourcePath: leaf.Path, TokenEnds: tokenized.TokenEnds, ContextVersionHash: platform.HashBytes(contextRaw)})
	next := SourceCursor{LeafIndex: cursor.LeafIndex + 1}
	if normalized.NextByte != nil {
		next = SourceCursor{LeafIndex: cursor.LeafIndex, StartByte: *normalized.NextByte}
	}
	if next.LeafIndex < len(leaves) {
		result.Next = &next
	}
	return result, nil
}

// 合并叶的上限：同一节相邻短块合并到大约这个字节数为止（约一个 512 token 分块），更长的由分块按 token 再切。
const mergedLeafBytes = 1200

// mergedLeafPrefix 标出合并叶的路径，与旧索引按文本节点记下的路径不会撞。
const mergedLeafPrefix = "group:"

// CollectSourceLeaves 给出建索引用的检索原文叶，按文档顺序：
// 同一稳定块里的文字（被加粗、链接等拆开的多个文本节点，以及图片替代文字、视频题名简介）合成一个叶，句子不再被格式拆碎；
// 同一节（到下一个标题为止）里相邻的短块再合并，直到约 mergedLeafBytes，避免短段落、列表项、表格单元格各自成块。
// 合并叶以第一个块为来源块（引用锚点指向它），文字用换行连接，偏移是合并文字内的字节位置。
func CollectSourceLeaves(body []byte, document gen.PublishedRevisionRef) ([]SourceLeaf, error) {
	pieces, headings, err := collectTextNodes(body, document)
	if err != nil {
		return nil, err
	}
	type blockText struct {
		block      gen.BlockRef
		first      string
		last       string
		text       string
		newSection bool
	}
	blocks := []blockText{}
	for _, piece := range pieces {
		if n := len(blocks); n > 0 && blocks[n-1].block.BlockId == piece.Block.BlockId {
			blocks[n-1].text += piece.Text
			blocks[n-1].last = piece.Path
			continue
		}
		blocks = append(blocks, blockText{block: piece.Block, first: piece.Path, last: piece.Path, text: piece.Text, newSection: headings[piece.Block.BlockId]})
	}
	leaves := []SourceLeaf{}
	var group *SourceLeaf
	var groupLast string
	flush := func() {
		if group != nil {
			group.Path = mergedLeafPrefix + group.Path + ".." + groupLast
			leaves = append(leaves, *group)
			group = nil
		}
	}
	for _, block := range blocks {
		if group != nil && (block.newSection || len(group.Text)+1+len(block.text) > mergedLeafBytes) {
			flush()
		}
		if group == nil {
			group = &SourceLeaf{Block: block.block, Path: block.first, Text: block.text}
		} else {
			group.Text += "\n" + block.text
		}
		groupLast = block.last
	}
	flush()
	return leaves, nil
}

// CollectLegacySourceLeaves 是旧索引按文本节点逐个切出的叶；检索核验旧索引时按原路径查找，新索引不再使用。
func CollectLegacySourceLeaves(body []byte, document gen.PublishedRevisionRef) ([]SourceLeaf, error) {
	pieces, _, err := collectTextNodes(body, document)
	return pieces, err
}

// collectTextNodes 原块文本视图为最近稳定块的text、alt及视频题名/简介按固定顺序直接串联，逐个文本节点给出；
// 同时记下哪些块是标题。嵌套稳定块单独起算；sourcePath指向实际JSON字符串，offset不混入JSON转义字节。
func collectTextNodes(body []byte, document gen.PublishedRevisionRef) ([]SourceLeaf, map[string]bool, error) {
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&root) != nil {
		return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "源正文树不可读")
	}
	leaves := []SourceLeaf{}
	headings := map[string]bool{}
	positions := map[string]int{}
	var walk func(map[string]any, string, *gen.BlockRef) error
	appendLeaf := func(value any, path string, current *gen.BlockRef) error {
		text, ok := value.(string)
		if !ok || text == "" {
			return nil
		}
		if current == nil || len(text) > 4*1024*1024 || utf8.RuneCountInString(text) > 1<<20 {
			return platform.NewError(platform.CodeIntegrityError, false, "原叶缺稳定块或超过真实分词片段预算")
		}
		leaves = append(leaves, SourceLeaf{Block: *current, Path: path, Text: text, BlockStart: positions[current.BlockId]})
		positions[current.BlockId] += len(text)
		return nil
	}
	walk = func(node map[string]any, path string, current *gen.BlockRef) error {
		attrs, _ := node["attrs"].(map[string]any)
		if id, ok := attrs["blockId"].(string); ok {
			raw, _ := platform.CanonicalJSON(node)
			ref := gen.BlockRef{Document: document, BlockId: id, BlockHash: platform.HashBytes(raw)}
			current = &ref
			if node["type"] == "heading" {
				headings[id] = true
			}
		}
		if err := appendLeaf(node["text"], path+"/text", current); err != nil {
			return err
		}
		if content, ok := node["content"].([]any); ok {
			for i, child := range content {
				n, ok := child.(map[string]any)
				if !ok {
					return platform.NewError(platform.CodeIntegrityError, false, "原正文节点不可读")
				}
				if err := walk(n, path+"/content/"+strconv.Itoa(i), current); err != nil {
					return err
				}
			}
		}
		if err := appendLeaf(attrs["alt"], path+"/attrs/alt", current); err != nil {
			return err
		}
		if node["type"] == "video" {
			if err := appendLeaf(attrs["title"], path+"/attrs/title", current); err != nil {
				return err
			}
			if err := appendLeaf(attrs["description"], path+"/attrs/description", current); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, "", nil); err != nil {
		return nil, nil, err
	}
	return leaves, headings, nil
}
