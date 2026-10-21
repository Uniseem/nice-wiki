package chunk

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"nice-wiki/modules/knowledge/internal/gen"
)

// ContextBatch 是一次背景短句调用：一组连续片段，以及给模型参考的“全文目录 + 所在章节原文”。
type ContextBatch struct {
	Chunks  []gen.Chunk
	Context string
}

// 一批最多的片段数（与 EnrichChunkContext 的上限一致），目录最多占的字节数。
const (
	contextBatchChunks  = 16
	contextOutlineBytes = 4000
)

// section 是正文里一个标题到下一个标题之间的内容；文档开头没有标题的部分也算一节。
type section struct {
	heading string
	chunks  []int // 属于这一节的片段下标（按文内顺序）
}

// PlanContextBatches 把片段按章节分批：连续片段凑成一批（最多 16 段），只要“目录 + 涉及章节的原文”放得进 room 给出的字节数；
// 一节原文本身放不下时，只取这批片段前后相邻的片段原文。room 由调用方按背景模型窗口、提示词和这批片段本身的大小算出。
// source 是发布正文树（JSON），用来认出标题和每个块属于哪一节；读不出来时整篇当作一节。
func PlanContextBatches(source string, chunks []gen.Chunk, room func(batch []gen.Chunk) int) []ContextBatch {
	sections, owner := splitSections(source, chunks)
	headings := []string{}
	for _, s := range sections {
		if s.heading != "" {
			headings = append(headings, s.heading)
		}
	}
	outline := ""
	if len(headings) > 0 {
		outline = clipBytes("全文目录：\n"+strings.Join(headings, "\n"), contextOutlineBytes) + "\n\n"
	}
	sectionText := func(index int, from, to int) string {
		var text strings.Builder
		if heading := sections[index].heading; heading != "" {
			text.WriteString("## " + heading + "\n")
		}
		for _, i := range sections[index].chunks[from:to] {
			text.WriteString(chunks[i].Text)
			text.WriteString("\n")
		}
		return text.String()
	}
	// contextFor 给一批片段组出参考原文：先试整节，放不下就以这批片段为中心向两边扩展同节片段。
	contextFor := func(batch []int) string {
		limit := room(pick(chunks, batch))
		var full strings.Builder
		full.WriteString(outline)
		seen := map[int]bool{}
		for _, i := range batch {
			if !seen[owner[i]] {
				seen[owner[i]] = true
				full.WriteString(sectionText(owner[i], 0, len(sections[owner[i]].chunks)))
			}
		}
		if full.Len() <= limit {
			return full.String()
		}
		home := sections[owner[batch[0]]].chunks
		first, last := position(home, batch[0]), position(home, batch[len(batch)-1])
		from, to := first, last+1
		for {
			grown := false
			if from > 0 && len(outline)+len(sectionText(owner[batch[0]], from-1, to)) <= limit {
				from--
				grown = true
			}
			if to < len(home) && len(outline)+len(sectionText(owner[batch[0]], from, to+1)) <= limit {
				to++
				grown = true
			}
			if !grown {
				break
			}
		}
		return clipBytes(outline+sectionText(owner[batch[0]], from, to), max(limit, 0))
	}
	// fits 判断一批片段的整节原文能不能放下；放不下也允许单独成批（由 contextFor 截取）。
	fits := func(batch []int) bool {
		size := len(outline)
		seen := map[int]bool{}
		for _, i := range batch {
			if !seen[owner[i]] {
				seen[owner[i]] = true
				size += len(sectionText(owner[i], 0, len(sections[owner[i]].chunks)))
			}
		}
		return size <= room(pick(chunks, batch))
	}
	batches := []ContextBatch{}
	current := []int{}
	flush := func() {
		if len(current) > 0 {
			batches = append(batches, ContextBatch{Chunks: pick(chunks, current), Context: contextFor(current)})
			current = []int{}
		}
	}
	for i := range chunks {
		next := append(append([]int{}, current...), i)
		// 换到新的一节且整节放不下，或已满 16 段，就先把这一批发出去；同一节内放不下的由 contextFor 截取。
		if len(current) > 0 && (len(current) == contextBatchChunks || (owner[i] != owner[current[len(current)-1]] && !fits(next)) || room(pick(chunks, next)) <= len(outline)) {
			flush()
			next = []int{i}
		}
		current = next
	}
	flush()
	return batches
}

// splitSections 按正文树顶层的标题切节，返回各节和每个片段所属的节；块不在树里的片段跟随前一个片段。
func splitSections(source string, chunks []gen.Chunk) ([]section, []int) {
	type node struct {
		Type    string         `json:"type"`
		Attrs   map[string]any `json:"attrs"`
		Text    string         `json:"text"`
		Content []node         `json:"content"`
	}
	var root node
	sections := []section{{}}
	blockSection := map[string]int{}
	if json.Unmarshal([]byte(source), &root) == nil {
		var text func(n node) string
		text = func(n node) string {
			parts := []string{n.Text}
			for _, child := range n.Content {
				parts = append(parts, text(child))
			}
			return strings.Join(parts, "")
		}
		var mark func(n node, index int)
		mark = func(n node, index int) {
			if id, ok := n.Attrs["blockId"].(string); ok {
				blockSection[id] = index
			}
			for _, child := range n.Content {
				mark(child, index)
			}
		}
		for _, top := range root.Content {
			if top.Type == "heading" {
				if heading := strings.TrimSpace(text(top)); heading != "" {
					sections = append(sections, section{heading: clipBytes(heading, 200)})
				}
			}
			mark(top, len(sections)-1)
		}
	}
	owner := make([]int, len(chunks))
	for i, value := range chunks {
		index, ok := blockSection[value.Block.BlockId]
		if !ok && i > 0 {
			index = owner[i-1]
		}
		owner[i] = index
		sections[index].chunks = append(sections[index].chunks, i)
	}
	return sections, owner
}

func pick(chunks []gen.Chunk, indexes []int) []gen.Chunk {
	result := make([]gen.Chunk, len(indexes))
	for i, index := range indexes {
		result[i] = chunks[index]
	}
	return result
}

func position(values []int, value int) int {
	for i, v := range values {
		if v == value {
			return i
		}
	}
	return 0
}

// clipBytes 按 UTF-8 字节截断，不切开多字节字符。
func clipBytes(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}

// ContextRoom 算出一批片段之外还能放多少字节参考原文：按网关的估算口径（libs/platform 的 EstimateInputTokens）
// 让整次请求留在背景模型窗口的九成以内。
func ContextRoom(window int, fixed []string, batch []gen.Chunk) int {
	if window <= 0 {
		return 0
	}
	used := 0
	for _, text := range fixed {
		used += len(text)
	}
	used += len(ContextRequestBatch(batch))
	// 四条消息（任务说明、记忆、参考原文、片段）：EstimateInputTokens(bytes, 4) ≤ window*0.9 ⇔ bytes ≤ (window*0.9/1.2 - 4*32)*2。
	return (window*3/4-4*32)*2 - used
}

// ContextRequestBatch 是一批片段发给模型的 JSON（与 EnrichChunkContext 发出的一致）。
func ContextRequestBatch(batch []gen.Chunk) string {
	items := make([]struct {
		ID   string `json:"chunkId"`
		Text string `json:"text"`
	}, len(batch))
	for i, value := range batch {
		items[i].ID = value.ChunkId
		items[i].Text = value.Text
	}
	raw, _ := json.Marshal(items)
	return string(raw)
}
