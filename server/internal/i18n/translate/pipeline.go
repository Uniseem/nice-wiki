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
}

func (p Pipeline) Run(ctx context.Context, document, language string, segments []Segment, hashes map[string]string) (File, error) {
	if p.Files == nil || p.Glossary == nil || p.Model == nil {
		return File{}, errors.New("翻译流水线尚未配置")
	}
	result, err := p.Files.ReadOptional(language, document)
	if err != nil {
		return File{}, err
	}
	pending := []Segment{}
	for _, segment := range segments {
		if reused, ok := Reuse(result, segment.ID, hashes[segment.ID], p.ReuseMinimum); ok {
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
	if err := p.Files.Write(result); err != nil {
		return File{}, err
	}
	return result, nil
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
