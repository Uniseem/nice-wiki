package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
)

type CohereRerank struct {
	BaseURL, Key string
	Client       *http.Client
	Headers      map[string]string
}

func (c CohereRerank) Rerank(ctx context.Context, model, query string, documents []string) ([]float64, Usage, error) {
	data, err := json.Marshal(map[string]any{"model": model, "query": query, "documents": documents, "top_n": len(documents)})
	if err != nil {
		return nil, Usage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(c.BaseURL, "/")+"/rerank", bytes.NewReader(data))
	if err != nil {
		return nil, Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyHeaders(req, c.Headers)
	req.Header.Set("Authorization", "Bearer "+c.Key)
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, Usage{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, Usage{}, errors.New("重排渠道请求失败")
	}
	var value struct {
		Results []struct {
			Index int     `json:"index"`
			Score float64 `json:"relevance_score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&value); err != nil {
		return nil, Usage{}, err
	}
	scores := make([]float64, len(documents))
	seen := make([]bool, len(documents))
	for _, item := range value.Results {
		if item.Index < 0 || item.Index >= len(scores) || seen[item.Index] || math.IsNaN(item.Score) || math.IsInf(item.Score, 0) {
			return nil, Usage{}, errors.New("重排响应索引或分数无效")
		}
		seen[item.Index] = true
		scores[item.Index] = item.Score
	}
	for _, ok := range seen {
		if !ok {
			return nil, Usage{}, errors.New("重排响应不完整")
		}
	}
	return scores, Usage{}, nil
}
