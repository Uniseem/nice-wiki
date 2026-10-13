package translate

import (
	"encoding/json"
	"errors"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/doc/nodes"
	"nice-wiki/internal/fsutil"
	"os"
	"reflect"
	"sync"
	"time"
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

// MergeMachine 在文件锁内重新读取最新值，避免任务覆盖人工保存。
func (f *Files) MergeMachine(before, proposed File, hashes map[string]string) (File, error) {
	path, err := f.layout.Translation(proposed.Language, proposed.DocumentID)
	if err != nil {
		return File{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	current := File{SchemaVersion: 1, DocumentID: proposed.DocumentID, Language: proposed.Language, Blocks: map[string]Block{}}
	if err := fsutil.ReadJSON(path, &current); err != nil && !errors.Is(err, os.ErrNotExist) {
		return File{}, err
	}
	if err := ValidateFile(current); err != nil {
		return File{}, err
	}
	merged := map[string]Block{}
	for id, hash := range hashes {
		value, exists := current.Blocks[id]
		if exists && value.Source == SourceHuman {
			value.NeedsReview = value.NeedsReview || value.SourceHash != hash
			if reflect.DeepEqual(current.Blocks[id], before.Blocks[id]) && proposed.Blocks[id].NeedsReview {
				value.NeedsReview = true
			}
			merged[id] = value
			continue
		}
		if !reflect.DeepEqual(value, before.Blocks[id]) {
			if exists {
				merged[id] = value
			}
			continue
		}
		if value, ok := proposed.Blocks[id]; ok {
			merged[id] = value
		}
	}
	current.Blocks = merged
	current.UpdatedAt = time.Now().Unix()
	if err := ValidateFile(current); err != nil {
		return File{}, err
	}
	return current, fsutil.WriteJSON(path, current)
}
func Apply(document doc.Document, translation File) doc.Document {
	// 克隆全部节点、属性和标记，渲染不能修改调用方正文。
	data, err := json.Marshal(document)
	if err != nil {
		return document
	}
	var result doc.Document
	if err := json.Unmarshal(data, &result); err != nil {
		return document
	}
	var replace func(*doc.Node)
	replace = func(node *doc.Node) {
		if id, ok := node.Attrs["id"].(string); ok {
			if block, exists := translation.Blocks[id]; exists && block.Text != "" {
				slots := nodes.TranslationSlots(node)
				values, ok := DecodeInline(block.Text, len(slots))
				if !ok && len(slots) == 1 && !placeholder.MatchString(block.Text) {
					values = []string{block.Text}
					ok = true
				}
				if ok {
					for i, slot := range slots {
						slot.Set(values[i])
					}
				}
			}
		}
		for i := range node.Content {
			replace(&node.Content[i])
		}
	}
	replace(&result.Content)
	return result
}
