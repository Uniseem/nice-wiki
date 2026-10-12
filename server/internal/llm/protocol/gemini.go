package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Gemini struct {
	BaseURL, Key string
	Client       *http.Client
}

func (g Gemini) Complete(ctx context.Context, r Request) (Response, error) {
	contents := []map[string]any{}
	for _, m := range r.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if role == "system" {
			role = "user"
		}
		contents = append(contents, map[string]any{"role": role, "parts": []map[string]string{{"text": m.Content}}})
	}
	body := map[string]any{"contents": contents, "generationConfig": map[string]any{"maxOutputTokens": r.MaxTokens, "temperature": r.Temperature}}
	data, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	target := strings.TrimSuffix(g.BaseURL, "/") + "/v1beta/models/" + url.PathEscape(r.Model) + ":generateContent?key=" + url.QueryEscape(g.Key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.Client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return Response{}, fmt.Errorf("模型渠道返回状态 %d", resp.StatusCode)
	}
	var value struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Usage struct {
			Input  int64 `json:"promptTokenCount"`
			Output int64 `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&value); err != nil {
		return Response{}, err
	}
	if len(value.Candidates) == 0 {
		return Response{}, errors.New("模型响应缺少候选")
	}
	text := ""
	for _, part := range value.Candidates[0].Content.Parts {
		text += part.Text
	}
	return Response{Text: text, Model: r.Model, Usage: Usage{InputTokens: value.Usage.Input, OutputTokens: value.Usage.Output}}, nil
}
func (g Gemini) Stream(ctx context.Context, r Request, emit func(Chunk) error) error {
	value, err := g.Complete(ctx, r)
	if err != nil {
		return err
	}
	if err := emit(Chunk{Text: value.Text, Usage: value.Usage}); err != nil {
		return err
	}
	return emit(Chunk{Done: true, Usage: value.Usage})
}
