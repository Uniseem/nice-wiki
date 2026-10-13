package translate

import (
	"context"
	"errors"
	"time"
)

type Model interface {
	Translate(context.Context, string, []Segment, []Term) (map[string]string, error)
}
type Pipeline struct {
	Files        *Files
	Glossary     *Glossary
	Model        Model
	BatchSize    int
	ReuseMinimum int
	Commit       func(func() error) error
}

func (p Pipeline) Run(ctx context.Context, document, language string, segments []Segment, hashes map[string]string) (File, error) {
	if p.Files == nil || p.Glossary == nil || p.Model == nil {
		return File{}, errors.New("翻译流水线尚未配置")
	}
	before, err := p.Files.ReadOptional(language, document)
	if err != nil {
		return File{}, err
	}
	result := before
	result.Blocks = map[string]Block{}
	pending := []Segment{}
	for _, segment := range segments {
		if human, ok := before.Blocks[segment.ID]; ok && human.Source == SourceHuman {
			human.NeedsReview = human.NeedsReview || human.SourceHash != hashes[segment.ID] || !compatibleInline(segment.Text, human.Text, true)
			result.Blocks[segment.ID] = human
			continue
		}
		if reused, ok := Reuse(before, segment.ID, hashes[segment.ID], p.ReuseMinimum); ok && compatibleInline(segment.Text, reused.Text, false) {
			result.Blocks[segment.ID] = reused
		} else {
			pending = append(pending, segment)
		}
	}
	for _, batch := range Batch(pending, p.BatchSize) {
		terms := p.Glossary.Relevant(language, joinSegments(batch))
		translated, err := p.translateWithFallback(ctx, language, batch, terms)
		if err != nil {
			return File{}, err
		}
		for _, segment := range batch {
			result.Blocks[segment.ID] = Block{ID: segment.ID, SourceHash: hashes[segment.ID], Text: translated[segment.ID], Source: SourceMachine}
		}
	}
	result.UpdatedAt = time.Now().Unix()
	commit := func() error { var err error; result, err = p.Files.MergeMachine(before, result, hashes); return err }
	if p.Commit != nil {
		err = p.Commit(commit)
	} else {
		err = commit()
	}
	return result, err
}
func (p Pipeline) translateWithFallback(ctx context.Context, language string, values []Segment, terms []Term) (map[string]string, error) {
	translated, err := p.Model.Translate(ctx, language, values, terms)
	if err == nil {
		err = ValidateBatch(values, translated)
	}
	if err == nil {
		return translated, nil
	}
	if len(values) == 1 {
		return nil, err
	}
	left, right := Split(values)
	a, err := p.translateWithFallback(ctx, language, left, terms)
	if err != nil {
		return nil, err
	}
	b, err := p.translateWithFallback(ctx, language, right, terms)
	if err != nil {
		return nil, err
	}
	for key, value := range b {
		a[key] = value
	}
	return a, nil
}
func joinSegments(values []Segment) string {
	result := ""
	for _, value := range values {
		result += value.Text + "\n"
	}
	return result
}
