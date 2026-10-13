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
	"net/url"
	"strings"
	"time"
)

type Gemini struct {
	BaseURL, Key  string
	Client        *http.Client
	Headers       map[string]string
	StreamTimeout time.Duration
}

func geminiBody(r Request) ([]byte, error) {
	contents := []map[string]any{}
	system := strings.Builder{}
	for _, m := range r.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if role == "system" {
			system.WriteString(m.Content)
			system.WriteByte('\n')
			continue
		}
		parts := []map[string]any{{"text": m.Content}}
		for _, image := range m.Images {
			mime, data, err := parseDataURL(image)
			if err != nil {
				return nil, err
			}
			parts = append(parts, map[string]any{"inlineData": map[string]string{"mimeType": mime, "data": data}})
		}
		contents = append(contents, map[string]any{"role": role, "parts": parts})
	}
	config := map[string]any{"maxOutputTokens": r.MaxTokens, "temperature": r.Temperature}
	if r.JSON {
		config["responseMimeType"] = "application/json"
	}
	body := map[string]any{"contents": contents, "generationConfig": config}
	if system.Len() > 0 {
		body["systemInstruction"] = map[string]any{"parts": []map[string]string{{"text": strings.TrimSpace(system.String())}}}
	}
	return json.Marshal(body)
}
func (g Gemini) Complete(ctx context.Context, r Request) (Response, error) {
	data, err := geminiBody(r)
	if err != nil {
		return Response{}, err
	}
	target := strings.TrimSuffix(g.BaseURL, "/") + "/v1beta/models/" + url.PathEscape(r.Model) + ":generateContent?key=" + url.QueryEscape(g.Key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyHeaders(req, g.Headers)
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
	streamCtx, stop, touch := idleContext(ctx, g.StreamTimeout)
	defer stop()
	data, err := geminiBody(r)
	if err != nil {
		return err
	}
	target := strings.TrimSuffix(g.BaseURL, "/") + "/v1beta/models/" + url.PathEscape(r.Model) + ":streamGenerateContent?alt=sse&key=" + url.QueryEscape(g.Key)
	request, err := http.NewRequestWithContext(streamCtx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	applyHeaders(request, g.Headers)
	response, err := g.Client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return fmt.Errorf("模型渠道返回状态 %d", response.StatusCode)
	}
	usage := Usage{}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		touch()
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
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
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &value); err != nil {
			return err
		}
		if value.Usage.Input > 0 {
			usage.InputTokens = value.Usage.Input
		}
		if value.Usage.Output > 0 {
			usage.OutputTokens = value.Usage.Output
		}
		for _, candidate := range value.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					if err := emit(Chunk{Text: part.Text, Usage: usage}); err != nil {
						return err
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return emit(Chunk{Done: true, Usage: usage})
}
