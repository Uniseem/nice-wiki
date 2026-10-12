package llm

import (
	_ "embed"
	"encoding/json"
	"errors"
	"nice-wiki/internal/fsutil"
	"os"
	"sync"
)

//go:embed catalog_snapshot.json
var catalogSnapshot []byte

type Catalog struct {
	mu     sync.RWMutex
	path   string
	models map[string]Model
}

func OpenCatalog(path string) (*Catalog, error) {
	var built struct {
		Models []Model `json:"models"`
	}
	if err := json.Unmarshal(catalogSnapshot, &built); err != nil {
		return nil, err
	}
	values := map[string]Model{}
	for _, model := range built.Models {
		values[model.Channel+":"+model.ID] = model
	}
	var cached []Model
	if err := fsutil.ReadJSON(path, &cached); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, model := range cached {
		values[model.Channel+":"+model.ID] = model
	}
	return &Catalog{path: path, models: values}, nil
}
func (c *Catalog) List() []Model {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Model, 0, len(c.models))
	for _, model := range c.models {
		result = append(result, model)
	}
	return result
}
func (c *Catalog) Merge(values []Model) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, model := range values {
		if model.ID == "" || model.Channel == "" {
			return errors.New("模型资料缺少身份")
		}
		c.models[model.Channel+":"+model.ID] = model
	}
	all := make([]Model, 0, len(c.models))
	for _, model := range c.models {
		all = append(all, model)
	}
	return fsutil.WriteJSON(c.path, all)
}
