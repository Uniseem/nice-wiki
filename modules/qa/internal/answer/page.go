package answer

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

// ReadCurrentPage 把访客所在页面全文交给回答模型：已建索引时按分块给（每段可引用）；
// 还没索引或索引读不到时直接读公开正文转成纯文本（不带引用标识），保证模型总能看到这一页。
func ReadCurrentPage(ctx context.Context, op OperationContext, input ReadDocumentsInput) (RetrieveEvidenceResult, string, error) {
	if indexed, err := ReadDocuments(ctx, op, input); err == nil && len(indexed.Citations) > 0 {
		return indexed, indexed.Citations[0].Title, nil
	}
	document, err := qaruntime.ReadCurrentDocument(ctx, op, input.DocumentIDs[0])
	if err != nil {
		return RetrieveEvidenceResult{}, "", err
	}
	var body pageNode
	if err := json.Unmarshal(document.Body, &body); err != nil {
		return RetrieveEvidenceResult{}, "", platform.NewError(platform.CodeIntegrityError, false, "公开正文不可读")
	}
	var text strings.Builder
	writePageText(&text, body, "")
	content, truncated := clipTokens(strings.TrimSpace(text.String()), input.MaximumTokens)
	type article struct {
		Title     string `json:"title"`
		Truncated bool   `json:"truncated,omitempty"`
		Text      string `json:"text"`
	}
	encoded, _ := json.Marshal([]article{{Title: document.Metadata.Title, Truncated: truncated, Text: content}})
	return RetrieveEvidenceResult{Citations: []gen.Citation{}, EvidenceText: string(encoded)}, document.Metadata.Title, nil
}

type pageNode struct {
	Type    string         `json:"type"`
	Text    string         `json:"text"`
	Attrs   map[string]any `json:"attrs"`
	Content []pageNode     `json:"content"`
}

// writePageText 把正文树写成近似 Markdown 的纯文本：标题带 #，列表带 -，代码块加围栏，块之间空行。
func writePageText(out *strings.Builder, node pageNode, indent string) {
	switch node.Type {
	case "text":
		out.WriteString(node.Text)
		return
	case "hardBreak":
		out.WriteString("\n" + indent)
		return
	case "heading":
		level, _ := node.Attrs["level"].(float64)
		out.WriteString(strings.Repeat("#", max(int(level), 1)) + " ")
	case "codeBlock":
		out.WriteString("```\n")
		writeChildren(out, node, indent)
		out.WriteString("\n```\n\n")
		return
	case "bulletList", "orderedList", "taskList":
		for _, item := range node.Content {
			out.WriteString(indent + "- ")
			for i, child := range item.Content {
				if i > 0 {
					out.WriteString(indent + "  ")
				}
				writePageText(out, child, indent+"  ")
			}
		}
		out.WriteString("\n")
		return
	case "tableRow":
		for _, cell := range node.Content {
			out.WriteString("| ")
			var inner strings.Builder
			writeChildren(&inner, cell, "")
			out.WriteString(strings.Join(strings.Fields(inner.String()), " ") + " ")
		}
		out.WriteString("|\n")
		return
	case "image":
		if alt, _ := node.Attrs["alt"].(string); alt != "" {
			out.WriteString("[图片：" + alt + "]\n\n")
		}
		return
	}
	writeChildren(out, node, indent)
	switch node.Type {
	case "paragraph", "heading":
		out.WriteString("\n\n")
	case "table":
		out.WriteString("\n")
	}
}

func writeChildren(out *strings.Builder, node pageNode, indent string) {
	for _, child := range node.Content {
		writePageText(out, child, indent)
	}
}

// clipTokens 按网关估算口径把文本截到预算以内，截断时在字符边界处切。
func clipTokens(text string, budget int) (string, bool) {
	estimate := platform.EstimateInputTokens(len(text), 0)
	if estimate <= budget {
		return text, false
	}
	cut := len(text) * budget / estimate
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut], true
}
