package protocol

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OpenAI struct {
	BaseURL, Key string
	Client       *http.Client
}

func (o OpenAI) request(ctx context.Context, path string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(o.BaseURL, "/")+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.Key)
	return o.Client.Do(req)
}
func (o OpenAI) Complete(ctx context.Context, r Request) (Response, error) {
	body := map[string]any{"model": r.Model, "messages": r.Messages, "max_tokens": r.MaxTokens, "temperature": r.Temperature}
	if r.JSON {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	resp, err := o.request(ctx, "/chat/completions", body)
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return Response{}, fmt.Errorf("模型渠道返回状态 %d", resp.StatusCode)
	}
	var value struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			Prompt     int64 `json:"prompt_tokens"`
			Completion int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&value)
	if err != nil || len(value.Choices) == 0 {
		return Response{}, errors.Join(err, errors.New("模型响应缺少内容"))
	}
	return Response{Text: value.Choices[0].Message.Content, Model: value.Model, Usage: Usage{InputTokens: value.Usage.Prompt, OutputTokens: value.Usage.Completion}}, nil
}
func (o OpenAI) Stream(ctx context.Context, r Request, emit func(Chunk) error) error {
	resp, err := o.request(ctx, "/chat/completions", map[string]any{"model": r.Model, "messages": r.Messages, "max_tokens": r.MaxTokens, "stream": true})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("模型渠道返回状态 %d", resp.StatusCode)
	}
	scan := bufio.NewScanner(resp.Body)
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		line := scan.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		raw := strings.TrimPrefix(line, "data: ")
		if raw == "[DONE]" {
			return emit(Chunk{Done: true})
		}
		var value struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(raw), &value) == nil && len(value.Choices) > 0 && value.Choices[0].Delta.Content != "" {
			if err := emit(Chunk{Text: value.Choices[0].Delta.Content}); err != nil {
				return err
			}
		}
	}
	return scan.Err()
}
