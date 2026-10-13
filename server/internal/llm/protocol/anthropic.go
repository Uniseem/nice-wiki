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
	"time"
)

type Anthropic struct {
	BaseURL, Key  string
	Client        *http.Client
	Headers       map[string]string
	StreamTimeout time.Duration
}

func anthropicBody(r Request, stream bool) ([]byte, error) {
	messages := []map[string]any{}
	system := ""
	for _, m := range r.Messages {
		if m.Role == "system" {
			system += m.Content + "\n"
		} else {
			if len(m.Images) == 0 {
				messages = append(messages, map[string]any{"role": m.Role, "content": m.Content})
				continue
			}
			content := []map[string]any{{"type": "text", "text": m.Content}}
			for _, image := range m.Images {
				mime, data, err := parseDataURL(image)
				if err != nil {
					return nil, err
				}
				content = append(content, map[string]any{"type": "image", "source": map[string]string{"type": "base64", "media_type": mime, "data": data}})
			}
			messages = append(messages, map[string]any{"role": m.Role, "content": content})
		}
	}
	return json.Marshal(map[string]any{"model": r.Model, "system": system, "messages": messages, "max_tokens": r.MaxTokens, "temperature": r.Temperature, "stream": stream})
}
func (a Anthropic) request(ctx context.Context, data []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(a.BaseURL, "/")+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyHeaders(req, a.Headers)
	req.Header.Set("x-api-key", a.Key)
	req.Header.Set("anthropic-version", "2023-06-01")
	return a.Client.Do(req)
}
func (a Anthropic) Complete(ctx context.Context, r Request) (Response, error) {
	data, err := anthropicBody(r, false)
	if err != nil {
		return Response{}, err
	}
	resp, err := a.request(ctx, data)
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
	streamCtx, stop, touch := idleContext(ctx, a.StreamTimeout)
	defer stop()
	data, err := anthropicBody(r, true)
	if err != nil {
		return err
	}
	resp, err := a.request(streamCtx, data)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("模型渠道返回状态 %d", resp.StatusCode)
	}
	usage := Usage{}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		touch()
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Text string `json:"text"`
			} `json:"delta"`
			Message struct {
				Usage struct {
					Input int64 `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				Output int64 `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			return err
		}
		switch event.Type {
		case "message_start":
			usage.InputTokens = event.Message.Usage.Input
		case "content_block_delta":
			if event.Delta.Text != "" {
				if err := emit(Chunk{Text: event.Delta.Text, Usage: usage}); err != nil {
					return err
				}
			}
		case "message_delta":
			usage.OutputTokens = event.Usage.Output
		case "message_stop":
			return emit(Chunk{Done: true, Usage: usage})
		case "error":
			return errors.New("模型流式响应失败")
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return errors.New("模型流式响应未正常结束")
}
