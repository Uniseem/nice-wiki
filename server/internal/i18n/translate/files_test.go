package translate

import (
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/doc/nodes"
	"reflect"
	"testing"
)

func TestMachineMergePreservesHuman(t *testing.T) {
	layout, _ := datadir.New(t.TempDir())
	files := NewFiles(layout)
	before := File{SchemaVersion: 1, DocumentID: "document", Language: "en", Blocks: map[string]Block{"block": {ID: "block", SourceHash: "old", Text: "机器", Source: SourceMachine}}}
	if err := files.Write(before); err != nil {
		t.Fatal(err)
	}
	human := before
	human.Blocks = map[string]Block{"block": {ID: "block", SourceHash: "old", Text: "人工", Source: SourceHuman}}
	if err := files.Write(human); err != nil {
		t.Fatal(err)
	}
	proposed := before
	proposed.Blocks = map[string]Block{"block": {ID: "block", SourceHash: "new", Text: "覆盖", Source: SourceMachine}}
	result, err := files.MergeMachine(before, proposed, map[string]string{"block": "new"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Blocks["block"].Text != "人工" || !result.Blocks["block"].NeedsReview {
		t.Fatal(result)
	}
}
func TestApplyPreservesInlineAndAttributes(t *testing.T) {
	document := doc.Document{SchemaVersion: doc.SchemaVersion, Content: doc.Node{Type: "doc", Content: []doc.Node{
		{Type: "paragraph", Attrs: map[string]any{"id": "paragraph"}, Content: []doc.Node{
			{Type: "text", Text: "原文一", Marks: []nodes.Mark{{Type: "bold"}}},
			{Type: "text", Text: "constant", Marks: []nodes.Mark{{Type: "code"}}},
			{Type: "text", Text: "原文二", Marks: []nodes.Mark{{Type: "link", Attrs: map[string]any{"href": "https://example.com"}}}},
		}},
		{Type: "image", Attrs: map[string]any{"id": "image", "alt": "替代", "caption": "图注", "hash": "unchanged"}},
		{Type: "codeBlock", Attrs: map[string]any{"id": "code"}, Content: []doc.Node{{Type: "text", Text: "immutable"}}},
	}}}
	result := Apply(document, File{Blocks: map[string]Block{"paragraph": {Text: "<nw-0>First</nw-0><nw-1>Second</nw-1>"}, "image": {Text: "<nw-0>Alt</nw-0><nw-1>Caption</nw-1>"}, "code": {Text: "破坏代码"}}})
	p := result.Content.Content[0]
	if p.Content[0].Text != "First" || p.Content[1].Text != "constant" || p.Content[2].Text != "Second" || !reflect.DeepEqual(p.Content[2].Marks, document.Content.Content[0].Content[2].Marks) {
		t.Fatal(p)
	}
	if document.Content.Content[0].Content[0].Text != "原文一" || result.Content.Content[1].Attrs["alt"] != "Alt" || result.Content.Content[2].Content[0].Text != "immutable" {
		t.Fatal(result)
	}
	bad := Apply(document, File{Blocks: map[string]Block{"paragraph": {Text: "丢失格式的旧译文"}}})
	if bad.Content.Content[0].Content[0].Text != "原文一" {
		t.Fatal("损坏格式的旧译文被应用")
	}
}
