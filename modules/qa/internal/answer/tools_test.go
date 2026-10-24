package answer

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAgentToolsRejectWritesAndUnknownArguments(t *testing.T) {
	for _, call := range []platform.GatewayToolCall{{Name: "write_document", Arguments: `{}`}, {Name: "read_document", Arguments: `{"documentId":"doc","path":"../private"}`}, {Name: "read_document", Arguments: `{"documentId":"doc","offset":-1}`}} {
		if _, err := ExecuteAgentTool(context.Background(), OperationContext{}, call, AgentToolInput{}); err == nil {
			t.Fatal("只读工具接受了写入或越界参数")
		}
	}
}

func TestToolSchemaAndRichTextKeepReadingInformation(t *testing.T) {
	for _, tool := range AgentTools() {
		raw, _ := json.Marshal(tool.Parameters)
		if strings.Contains(string(raw), `"required":null`) {
			t.Fatal("工具声明包含非法的空required字段")
		}
	}
	var node pageNode
	if err := json.Unmarshal([]byte(`{"type":"paragraph","content":[{"type":"text","text":"官网","marks":[{"type":"link","attrs":{"href":"https://example.com/docs"}}]},{"type":"inlineMath","attrs":{"latex":"x^2"}}]}`), &node); err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	writePageText(&text, node, "")
	if !strings.Contains(text.String(), "https://example.com/docs") || !strings.Contains(text.String(), "$x^2$") {
		t.Fatal("正文阅读丢失链接目标或公式")
	}
}

func TestCurrentPageIsPairedToolMaterial(t *testing.T) {
	prompt, err := BuildAnswerPrompt(context.Background(), OperationContext{OperationID: "operation"}, BuildAnswerPromptInput{Question: "这一页怎么配置", Memory: qaruntime.MemoryContext{TaskID: "qa.answer", Language: "zh-CN"}, Materials: Materials{Agent: true, Page: `{"documentId":"doc","title":"配置","publishedRevisionId":"published","sections":[{"citationId":"citation.page","text":"公开正文"}]}`, Evidence: "[]"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.ValidateToolConversation(prompt.Messages, AgentTools(), "auto"); err != nil {
		t.Fatal(err)
	}
	if len(prompt.Messages) != 5 || prompt.Messages[2].ToolCalls[0].Name != "read_document" || prompt.Messages[3].Role != "tool" || !strings.Contains(prompt.Messages[4].Content, "bodyPreloaded") {
		t.Fatal("当前页没有作为可移出和续读的原生工具资料装配")
	}
	turn := prompt.Messages[prompt.TurnMessageStart:]
	if len(turn) != 4 || turn[0].Role != "user" || turn[2].Role != "tool" {
		t.Fatal("当前页预读没有纳入本轮可重放的工具过程")
	}
	if err := platform.ValidateToolConversation(turn, AgentTools(), "auto"); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentContinuationPreservesUnicodeAndReferences(t *testing.T) {
	paragraph := strings.Repeat("中文与emoji🙂配置。", 1200)
	body := map[string]any{"type": "doc", "content": []any{map[string]any{"type": "heading", "attrs": map[string]any{"blockId": "heading", "level": 1}, "content": []any{map[string]any{"type": "text", "text": "配置"}}}, map[string]any{"type": "paragraph", "attrs": map[string]any{"blockId": "body"}, "content": []any{map[string]any{"type": "text", "text": paragraph}}}}}
	raw, _ := json.Marshal(body)
	document := qaruntime.PublishedDocument{Reference: gen.PublishedRevisionRef{DocumentId: "doc", PublishedRevisionId: "publication", ContentHash: platform.HashBytes(raw), Revision: 1, SchemaVersion: 1, DataEpoch: "epoch"}, Body: raw, Blocks: map[string]string{"heading": platform.HashBytes([]byte("heading")), "body": platform.HashBytes([]byte("body"))}}
	document.Metadata.Title = "配置"
	var text strings.Builder
	offset := 0
	for pages := 0; ; pages++ {
		if pages > 200 {
			t.Fatal("正文续读没有前进")
		}
		page, citations, err := buildAgentDocumentPage(document, agentDocumentArguments{Offset: offset}, AgentToolInput{TurnID: "turn", MaximumBytes: 4096})
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(page)
		if len(encoded) > 4096 || len(page.Sections) != len(citations) {
			t.Fatal("分页超出窗口或引用丢失")
		}
		for index, section := range page.Sections {
			if !utf8.ValidString(section.Text) || section.CitationID != citations[index].CitationId || citations[index].Document != document.Reference || citations[index].ExcerptHash != platform.HashBytes([]byte(section.Text)) {
				t.Fatal("Unicode边界或发布引用损坏")
			}
			text.WriteString(section.Text)
		}
		if page.Next == nil {
			break
		}
		if *page.Next <= offset {
			t.Fatal("正文分页起点没有前进")
		}
		offset = *page.Next
	}
	if text.String() != "# 配置\n\n"+paragraph+"\n\n" {
		t.Fatal("续读后正文有遗漏或重复")
	}
}
