package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type CohereRerank struct {
	BaseURL, Key string
	Client       *http.Client
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
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")
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
	for _, item := range value.Results {
		if item.Index >= 0 && item.Index < len(scores) {
			scores[item.Index] = item.Score
		}
	}
	return scores, Usage{}, nil
}
