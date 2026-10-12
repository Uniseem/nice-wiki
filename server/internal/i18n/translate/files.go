package translate

import (
	"errors"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/fsutil"
	"os"
	"sync"
)

type Source string

const (
	SourceMachine Source = "machine"
	SourceHuman   Source = "human"
)

type Block struct {
	ID          string `json:"id"`
	SourceHash  string `json:"sourceHash"`
	Text        string `json:"text"`
	Source      Source `json:"source"`
	NeedsReview bool   `json:"needsReview"`
}
type File struct {
	SchemaVersion int              `json:"schemaVersion"`
	DocumentID    string           `json:"documentId"`
	Language      string           `json:"language"`
	Blocks        map[string]Block `json:"blocks"`
	UpdatedAt     int64            `json:"updatedAt"`
}
type Files struct {
	mu     sync.RWMutex
	layout datadir.Layout
}

func NewFiles(layout datadir.Layout) *Files { return &Files{layout: layout} }
func (f *Files) Read(language, document string) (File, error) {
	path, err := f.layout.Translation(language, document)
	if err != nil {
		return File{}, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	var value File
	if err := fsutil.ReadJSON(path, &value); err != nil {
		return value, err
	}
	return value, ValidateFile(value)
}
func (f *Files) ReadOptional(language, document string) (File, error) {
	value, err := f.Read(language, document)
	if errors.Is(err, os.ErrNotExist) {
		return File{SchemaVersion: 1, DocumentID: document, Language: language, Blocks: map[string]Block{}}, nil
	}
	return value, err
}
func (f *Files) Write(value File) error {
	if err := ValidateFile(value); err != nil {
		return err
	}
	path, err := f.layout.Translation(value.Language, value.DocumentID)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return fsutil.WriteJSON(path, value)
}
func ValidateFile(value File) error {
	if value.SchemaVersion != 1 || !datadir.ValidID(value.DocumentID) || value.Language == "" || value.Blocks == nil {
		return errors.New("译文文件结构无效")
	}
	for id, block := range value.Blocks {
		if id != block.ID || !datadir.ValidID(id) || block.SourceHash == "" {
			return errors.New("译文块结构无效")
		}
	}
	return nil
}
func (f *Files) ReadStrings(language, name string) (map[string]string, error) {
	path, err := f.layout.TranslationStrings(language, name)
	if err != nil {
		return nil, err
	}
	value := map[string]string{}
	if err := fsutil.ReadJSON(path, &value); errors.Is(err, os.ErrNotExist) {
		return value, nil
	} else if err != nil {
		return nil, err
	}
	return value, nil
}

func Apply(document doc.Document, translation File) doc.Document {
	result := document
	var replace func(*doc.Node)
	replace = func(node *doc.Node) {
		if id, ok := node.Attrs["id"].(string); ok {
			if block, exists := translation.Blocks[id]; exists && block.Text != "" {
				assigned := false
				var text func(*doc.Node)
				text = func(value *doc.Node) {
					if value.Type == "text" {
						if !assigned {
							value.Text = block.Text
							assigned = true
						} else {
							value.Text = ""
						}
						return
					}
					for index := range value.Content {
						text(&value.Content[index])
					}
				}
				text(node)
			}
		}
		for index := range node.Content {
			replace(&node.Content[index])
		}
	}
	replace(&result.Content)
	return result
}
