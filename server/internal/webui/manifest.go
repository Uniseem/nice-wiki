package webui

import (
	"encoding/json"
	"errors"
	"io/fs"
)

type Entry struct {
	File    string   `json:"file"`
	CSS     []string `json:"css"`
	Imports []string `json:"imports"`
	IsEntry bool     `json:"isEntry"`
}
type Manifest map[string]Entry

func LoadManifest(files fs.FS) (Manifest, error) {
	data, err := fs.ReadFile(files, ".vite/manifest.json")
	if err != nil {
		return nil, err
	}
	var result Manifest
	err = json.Unmarshal(data, &result)
	return result, err
}
func (m Manifest) Entry(name string) (Entry, error) {
	value, ok := m[name]
	if !ok {
		return Entry{}, errors.New("前端构建入口缺失")
	}
	return value, nil
}
