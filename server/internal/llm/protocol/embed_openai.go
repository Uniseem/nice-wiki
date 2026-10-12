package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	for _, item := range value.Data {
		if item.Index >= 0 && item.Index < len(result) {
			result[item.Index] = item.Embedding
		}
	}
	for _, v := range result {
		if len(v) == 0 {
			return nil, Usage{}, errors.New("嵌入响应不完整")
		}
	}
	return result, Usage{InputTokens: value.Usage.Prompt}, nil
}
