package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type rawProvider struct {
	ID, Name string
	Models   map[string]rawModel `json:"models"`
}
type rawModel struct {
	ID, Name         string
	Attachment       bool `json:"attachment"`
	StructuredOutput bool `json:"structured_output"`
	Cost             struct {
		Input  float64 `json:"input"`
		Output float64 `json:"output"`
	} `json:"cost"`
	Limit struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
}
type model struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Channel      string   `json:"channel"`
	Capabilities []string `json:"capabilities"`
	InputPrice   float64  `json:"inputPrice"`
	OutputPrice  float64  `json:"outputPrice"`
	Context      int      `json:"context"`
	MaxOutput    int      `json:"maxOutput"`
}

func main() {
	source := flag.String("url", "https://models.dev/api.json?type=all", "models.dev API")
	output := flag.String("out", "internal/llm/catalog_snapshot.json", "输出文件")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, *source, nil)
	if err != nil {
		fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		fatal(err)
	}
	if response.StatusCode/100 != 2 {
		_ = response.Body.Close()
		fatal(fmt.Errorf("目录返回状态 %d", response.StatusCode))
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		fatal(err)
	}
	providers := map[string]rawProvider{}
	if err := json.Unmarshal(data, &providers); err != nil {
		fatal(err)
	}
	models := []model{}
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
			capabilities := []string{"chat"}
			if strings.Contains(kind, "rerank") || strings.Contains(kind, "reranker") {
				capabilities = []string{"rerank"}
			} else if strings.Contains(kind, "embedding") || strings.Contains(kind, "-embed") || strings.Contains(kind, "/embed") {
				capabilities = []string{"embedding"}
			} else {
				if value.Attachment {
					capabilities = append(capabilities, "vision")
				}
				if value.StructuredOutput {
					capabilities = append(capabilities, "structured")
				}
			}
			models = append(models, model{ID: id, Name: name, Channel: providerID, Capabilities: capabilities, InputPrice: value.Cost.Input, OutputPrice: value.Cost.Output, Context: value.Limit.Context, MaxOutput: value.Limit.Output})
		}
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].Channel != models[j].Channel {
			return models[i].Channel < models[j].Channel
		}
		return models[i].ID < models[j].ID
	})
	snapshot := map[string]any{"version": "models.dev", "generatedAt": time.Now().UTC().Format(time.RFC3339), "models": models}
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*output, append(encoded, '\n'), 0644); err != nil {
		fatal(err)
	}
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
