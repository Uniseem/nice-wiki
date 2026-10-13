package llm

import (
	_ "embed"
	"encoding/json"
	"errors"
	"math"
	"nice-wiki/internal/fsutil"
	"os"
	"sort"
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
	sort.Slice(result, func(i, j int) bool { return result[i].Channel+":"+result[i].ID < result[j].Channel+":"+result[j].ID })
	return result
}
func (c *Catalog) Merge(values []Model) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	next := map[string]Model{}
	for key, model := range c.models {
		next[key] = model
	}
	for _, model := range values {
		if model.ID == "" || model.Channel == "" || model.InputPrice < 0 || model.OutputPrice < 0 || math.IsNaN(model.InputPrice) || math.IsNaN(model.OutputPrice) || math.IsInf(model.InputPrice, 0) || math.IsInf(model.OutputPrice, 0) {
			return errors.New("模型资料无效")
		}
		next[model.Channel+":"+model.ID] = model
	}
	all := make([]Model, 0, len(next))
	for _, model := range next {
		all = append(all, model)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Channel+":"+all[i].ID < all[j].Channel+":"+all[j].ID })
	if err := fsutil.WriteJSON(c.path, all); err != nil {
		return err
	}
	c.models = next
	return nil
}
