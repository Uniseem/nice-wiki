package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
)

func (o OpenAI) Embed(ctx context.Context, model string, input []string) ([][]float32, Usage, error) {
	resp, err := o.request(ctx, "/embeddings", map[string]any{"model": model, "input": input})
	if err != nil {
		return nil, Usage{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, Usage{}, errors.New("嵌入渠道请求失败")
	}
	var value struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Usage struct {
			Prompt int64 `json:"prompt_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&value); err != nil {
		return nil, Usage{}, err
	}
	result := make([][]float32, len(input))
	seen := make([]bool, len(input))
	dimensions := 0
	for _, item := range value.Data {
		if item.Index < 0 || item.Index >= len(result) || seen[item.Index] || len(item.Embedding) == 0 {
			return nil, Usage{}, errors.New("嵌入响应索引无效")
		}
		if dimensions == 0 {
			dimensions = len(item.Embedding)
		}
		if len(item.Embedding) != dimensions {
			return nil, Usage{}, errors.New("嵌入响应维度不一致")
		}
		for _, number := range item.Embedding {
			if math.IsNaN(float64(number)) || math.IsInf(float64(number), 0) {
				return nil, Usage{}, errors.New("嵌入响应包含无效向量")
			}
		}
		seen[item.Index] = true
		result[item.Index] = item.Embedding
	}
	for _, v := range result {
		if len(v) == 0 {
			return nil, Usage{}, errors.New("嵌入响应不完整")
		}
	}
	return result, Usage{InputTokens: value.Usage.Prompt}, nil
}
