package translate

import (
	"context"
	"errors"
	"nice-wiki/internal/datadir"
	"testing"
)

type translationFunc func(context.Context, string, []Segment, []Term) (map[string]string, error)

func (f translationFunc) Translate(ctx context.Context, language string, segments []Segment, terms []Term) (map[string]string, error) {
	return f(ctx, language, segments, terms)
}
func TestPipelineHumanAndStaleSource(t *testing.T) {
	layout, _ := datadir.New(t.TempDir())
	files := NewFiles(layout)
	glossary, err := OpenGlossary(layout.Glossary())
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Write(File{SchemaVersion: 1, DocumentID: "document", Language: "en", Blocks: map[string]Block{"block": {ID: "block", SourceHash: "old", Text: "人工", Source: SourceHuman}, "deleted": {ID: "deleted", SourceHash: "old", Text: "旧块", Source: SourceMachine}}}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	p := Pipeline{Files: files, Glossary: glossary, Model: translationFunc(func(context.Context, string, []Segment, []Term) (map[string]string, error) {
		calls++
		return nil, errors.New("人工译文不得调用模型")
	}), BatchSize: 100, ReuseMinimum: 20}
	result, err := p.Run(context.Background(), "document", "en", []Segment{{ID: "block", Text: "短"}}, map[string]string{"block": "new"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 || result.Blocks["block"].Text != "人工" || !result.Blocks["block"].NeedsReview || len(result.Blocks) != 1 {
		t.Fatal(result)
	}
	if err := files.Write(File{SchemaVersion: 1, DocumentID: "legacy", Language: "en", Blocks: map[string]Block{"block": {ID: "block", SourceHash: "same", Text: "Legacy translation without inline placeholders", Source: SourceMachine}}}); err != nil {
		t.Fatal(err)
	}
	p.Model = translationFunc(func(context.Context, string, []Segment, []Term) (map[string]string, error) {
		calls++
		return map[string]string{"block": "<nw-0>Translated</nw-0>"}, nil
	})
	legacy, err := p.Run(context.Background(), "legacy", "en", []Segment{{ID: "block", Text: "<nw-0>原文</nw-0>"}}, map[string]string{"block": "same"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || legacy.Blocks["block"].Text != "<nw-0>Translated</nw-0>" {
		t.Fatal("复用了丢失格式的机器译文")
	}
	p.Model = translationFunc(func(context.Context, string, []Segment, []Term) (map[string]string, error) {
		return map[string]string{"next": "Translated"}, nil
	})
	p.Commit = func(func() error) error { return errors.New("原文版本改变") }
	if _, err := p.Run(context.Background(), "document", "en", []Segment{{ID: "next", Text: "原文"}}, map[string]string{"next": "hash"}); err == nil {
		t.Fatal("接受旧版本写回")
	}
	saved, err := files.Read("en", "document")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := saved.Blocks["next"]; ok {
		t.Fatal("旧版本已落盘")
	}
}
func TestPipelineConcurrentManualSave(t *testing.T) {
	layout, _ := datadir.New(t.TempDir())
	files := NewFiles(layout)
	glossary, err := OpenGlossary(layout.Glossary())
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	p := Pipeline{Files: files, Glossary: glossary, BatchSize: 100, Model: translationFunc(func(context.Context, string, []Segment, []Term) (map[string]string, error) {
		close(entered)
		<-release
		return map[string]string{"block": "Machine"}, nil
	})}
	go func() {
		_, err := p.Run(context.Background(), "document", "en", []Segment{{ID: "block", Text: "原文"}}, map[string]string{"block": "hash"})
		done <- err
	}()
	<-entered
	if err := files.Write(File{SchemaVersion: 1, DocumentID: "document", Language: "en", Blocks: map[string]Block{"block": {ID: "block", SourceHash: "hash", Text: "Human", Source: SourceHuman}}}); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	saved, err := files.Read("en", "document")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Blocks["block"].Text != "Human" {
		t.Fatal(saved)
	}
}
