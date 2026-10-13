package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
)

func (g Gemini) Embed(ctx context.Context, model string, input []string) ([][]float32, Usage, error) {
	requests := make([]map[string]any, len(input))
	for i, text := range input {
		requests[i] = map[string]any{"model": "models/" + model, "content": map[string]any{"parts": []map[string]string{{"text": text}}}}
	}
	data, err := json.Marshal(map[string]any{"requests": requests})
	if err != nil {
		return nil, Usage{}, err
	}
	target := strings.TrimSuffix(g.BaseURL, "/") + "/v1beta/models/" + url.PathEscape(model) + ":batchEmbedContents?key=" + url.QueryEscape(g.Key)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return nil, Usage{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	applyHeaders(request, g.Headers)
	response, err := g.Client.Do(request)
	if err != nil {
		return nil, Usage{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return nil, Usage{}, errors.New("Gemini 嵌入请求失败")
	}
	var value struct {
		Embeddings []struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(&value); err != nil {
		return nil, Usage{}, err
	}
	if len(value.Embeddings) != len(input) {
		return nil, Usage{}, errors.New("Gemini 嵌入响应不完整")
	}
	result := make([][]float32, len(input))
	dimensions := 0
	for i := range value.Embeddings {
		if len(value.Embeddings[i].Values) == 0 {
			return nil, Usage{}, errors.New("Gemini 嵌入响应包含空向量")
		}
		if dimensions == 0 {
			dimensions = len(value.Embeddings[i].Values)
		}
		if len(value.Embeddings[i].Values) != dimensions {
			return nil, Usage{}, errors.New("Gemini 嵌入响应维度不一致")
		}
		for _, number := range value.Embeddings[i].Values {
			if math.IsNaN(float64(number)) || math.IsInf(float64(number), 0) {
				return nil, Usage{}, errors.New("Gemini 嵌入响应包含无效向量")
			}
		}
		result[i] = value.Embeddings[i].Values
	}
	return result, Usage{}, nil
}
