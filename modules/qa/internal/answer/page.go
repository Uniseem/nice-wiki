package answer

import (
	"context"
	"strings"
	"unicode/utf8"

	"nice-wiki/libs/platform"
)

// ReadCurrentPage 从当前发布正文预读访客所在页面，不依赖索引；分页后仍可按nextOffset继续读。
func ReadCurrentPage(ctx context.Context, op OperationContext, input ReadDocumentsInput) (RetrieveEvidenceResult, string, error) {
	if len(input.DocumentIDs) == 0 {
		return RetrieveEvidenceResult{}, "", platform.NewError(platform.CodeValidation, false, "当前页缺少文档ID")
	}
	read, err := readAgentDocument(ctx, op, agentDocumentArguments{DocumentID: input.DocumentIDs[0]}, AgentToolInput{TurnID: input.TurnID, Language: input.Language, MaximumBytes: min(56000, max(4096, input.MaximumTokens*5/3))})
	if err != nil {
		return RetrieveEvidenceResult{}, "", err
	}
	return RetrieveEvidenceResult{Citations: read.Citations, EvidenceText: read.Content}, read.Title, nil
}

type pageNode struct {
	Type    string         `json:"type"`
	Text    string         `json:"text"`
	Attrs   map[string]any `json:"attrs"`
	Content []pageNode     `json:"content"`
	Marks   []struct {
		Type  string         `json:"type"`
		Attrs map[string]any `json:"attrs"`
	} `json:"marks"`
}

// writePageText 把正文树写成近似 Markdown 的纯文本：标题带 #，列表带 -，代码块加围栏，块之间空行。
func writePageText(out *strings.Builder, node pageNode, indent string) {
	switch node.Type {
	case "text":
		text := node.Text
		for _, mark := range node.Marks {
			switch mark.Type {
			case "link":
				if href, ok := mark.Attrs["href"].(string); ok && href != "" {
					text = "[" + text + "](" + href + ")"
				}
			case "strike":
				text = "~~" + text + "~~"
			case "code":
				text = "`" + text + "`"
			}
		}
		out.WriteString(text)
		return
	case "hardBreak":
		out.WriteString("\n" + indent)
		return
	case "heading":
		level, _ := node.Attrs["level"].(float64)
		out.WriteString(strings.Repeat("#", min(max(int(level), 1), 6)) + " ")
	case "codeBlock":
		language, _ := node.Attrs["language"].(string)
		out.WriteString("```" + language + "\n")
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
		} else {
			out.WriteString("[图片，未提供替代文字]\n\n")
		}
		if caption, _ := node.Attrs["caption"].(string); caption != "" {
			out.WriteString(caption + "\n\n")
		}
		return
	case "inlineMath", "blockMath", "mathInline", "mathBlock":
		formula, _ := node.Attrs["latex"].(string)
		if formula == "" {
			formula, _ = node.Attrs["formula"].(string)
		}
		out.WriteString("$" + formula + "$")
		if node.Type == "blockMath" || node.Type == "mathBlock" {
			out.WriteString("\n\n")
		}
		return
	case "video", "attachment", "file":
		out.WriteString("[媒体/附件")
		for _, key := range []string{"title", "fileName", "name", "caption", "url", "src"} {
			if value, _ := node.Attrs[key].(string); value != "" {
				out.WriteString(" " + key + "=" + value)
			}
		}
		out.WriteString("]\n\n")
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
