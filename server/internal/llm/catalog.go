package llm

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"nice-wiki/internal/fsutil"
	"os"
	"sort"
	"strings"
	"sync"
)

type remoteProvider struct {
	Models map[string]remoteModel `json:"models"`
}
type remoteModel struct {
	ID, Name         string
	Attachment       bool                            `json:"attachment"`
	StructuredOutput bool                            `json:"structured_output"`
	Cost             struct{ Input, Output float64 } `json:"cost"`
	Limit            struct{ Context, Output int }   `json:"limit"`
}

//go:embed catalog_snapshot.json
var catalogSnapshot []byte

type Catalog struct {
	mu     sync.RWMutex
	path   string
	models map[string]Model
}

func OpenCatalog(path string) (*Catalog, error) {
	return openCatalog(path, false)
}
func OpenCatalogWithSnapshot(path string) (*Catalog, error) {
	return openCatalog(path, true)
}
func openCatalog(path string, includeSnapshot bool) (*Catalog, error) {
	var built struct {
		Models []Model `json:"models"`
	}
	if includeSnapshot {
		if err := json.Unmarshal(catalogSnapshot, &built); err != nil {
			return nil, err
		}
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
func (c *Catalog) Refresh(ctx context.Context, client *http.Client, url string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return errors.New("模型目录更新失败")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return err
	}
	providers := map[string]remoteProvider{}
	if err := json.Unmarshal(data, &providers); err != nil {
		return err
	}
	models := []Model{}
	for providerID, provider := range providers {
		for modelID, value := range provider.Models {
			id := value.ID
			if id == "" {
				id = modelID
			}
			name := value.Name
			if name == "" {
				name = id
			}
			kind := strings.ToLower(id + " " + name)
			capabilities := []Capability{Chat}
			if strings.Contains(kind, "rerank") || strings.Contains(kind, "reranker") {
				capabilities = []Capability{Rerank}
			} else if strings.Contains(kind, "embedding") || strings.Contains(kind, "-embed") || strings.Contains(kind, "/embed") {
				capabilities = []Capability{Embedding}
			} else {
				if value.Attachment {
					capabilities = append(capabilities, Vision)
				}
				if value.StructuredOutput {
					capabilities = append(capabilities, Structured)
				}
			}
			models = append(models, Model{ID: id, Name: name, Channel: providerID, Capabilities: capabilities, InputPrice: value.Cost.Input, OutputPrice: value.Cost.Output, Context: value.Limit.Context, MaxOutput: value.Limit.Output})
		}
	}
	if len(models) == 0 {
		return errors.New("模型目录为空")
	}
	return c.Merge(models)
}
