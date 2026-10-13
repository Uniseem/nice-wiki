package translate

import (
	"errors"
	"nice-wiki/internal/fsutil"
	"os"
	"sort"
	"strings"
	"sync"
)

type TermSource string

const (
	TermManual    TermSource = "manual"
	TermAutomatic TermSource = "automatic"
)

type Term struct {
	ID         string     `json:"id"`
	SourceText string     `json:"sourceText"`
	TargetText string     `json:"targetText"`
	Language   string     `json:"language"`
	Source     TermSource `json:"source"`
	DocumentID string     `json:"documentId"`
}
type Glossary struct {
	mu     sync.RWMutex
	path   string
	values []Term
}

func OpenGlossary(path string) (*Glossary, error) {
	values := []Term{}
	if err := fsutil.ReadJSON(path, &values); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return &Glossary{path: path, values: values}, nil
}
func (g *Glossary) List(language, source string) []Term {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := []Term{}
	for _, v := range g.values {
		if language != "" && v.Language != language || source != "" && string(v.Source) != source {
			continue
		}
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SourceText < result[j].SourceText })
	return result
}
func (g *Glossary) Save(value Term) error {
	if strings.TrimSpace(value.SourceText) == "" || strings.TrimSpace(value.TargetText) == "" || value.Language == "" {
		return errors.New("术语内容无效")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	next := append([]Term{}, g.values...)
	replaced := false
	for i, v := range next {
		if v.Language == value.Language && strings.EqualFold(v.SourceText, value.SourceText) {
			if v.Source == TermManual && value.Source == TermAutomatic {
				return nil
			}
			next[i] = value
			replaced = true
		}
	}
	if !replaced {
		next = append(next, value)
	}
	if err := fsutil.WriteJSON(g.path, next); err != nil {
		return err
	}
	g.values = next
	return nil
}
func (g *Glossary) Relevant(language, text string) []Term {
	values := g.List(language, "")
	result := values[:0]
	lower := strings.ToLower(text)
	for _, v := range values {
		if strings.Contains(lower, strings.ToLower(v.SourceText)) {
			result = append(result, v)
		}
	}
	return result
}
func (g *Glossary) UndoAutomatic(document string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	next := g.values[:0]
	for _, v := range g.values {
		if v.Source == TermAutomatic && v.DocumentID == document {
			continue
		}
		next = append(next, v)
	}
	if err := fsutil.WriteJSON(g.path, next); err != nil {
		return err
	}
	g.values = append([]Term{}, next...)
	return nil
}
