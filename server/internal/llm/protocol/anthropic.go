package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Anthropic struct {
	BaseURL, Key string
	Client       *http.Client
}

func (a Anthropic) Complete(ctx context.Context, r Request) (Response, error) {
	messages := []Message{}
	system := ""
	for _, m := range r.Messages {
		if m.Role == "system" {
			system += m.Content + "\n"
		} else {
			messages = append(messages, m)
		}
	}
	data, err := json.Marshal(map[string]any{"model": r.Model, "system": system, "messages": messages, "max_tokens": r.MaxTokens, "temperature": r.Temperature})
	if err != nil {
		return Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(a.BaseURL, "/")+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.Key)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := a.Client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return Response{}, fmt.Errorf("模型渠道返回状态 %d", resp.StatusCode)
	}
	var value struct {
		Model   string                        `json:"model"`
		Content []struct{ Type, Text string } `json:"content"`
		Usage   struct {
			Input  int64 `json:"input_tokens"`
			Output int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&value); err != nil {
		return Response{}, err
	}
	text := ""
	for _, part := range value.Content {
		if part.Type == "text" {
			text += part.Text
		}
	}
	if text == "" {
		return Response{}, errors.New("模型响应缺少内容")
	}
	return Response{Text: text, Model: value.Model, Usage: Usage{InputTokens: value.Usage.Input, OutputTokens: value.Usage.Output}}, nil
}
func (a Anthropic) Stream(ctx context.Context, r Request, emit func(Chunk) error) error {
	value, err := a.Complete(ctx, r)
	if err != nil {
		return err
	}
	if err := emit(Chunk{Text: value.Text, Usage: value.Usage}); err != nil {
		return err
	}
	return emit(Chunk{Done: true, Usage: value.Usage})
}
