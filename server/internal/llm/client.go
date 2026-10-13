package llm

import (
	"context"
	"errors"
	"net/http"
	"nice-wiki/internal/aitask"
	"nice-wiki/internal/llm/protocol"
	"strings"
	"time"
)

type Protocols interface {
	Chat(Model) (protocol.Chat, error)
}
type channelLimiter interface {
	Acquire(context.Context, string) (func(), error)
}
type ProtocolRegistry struct {
	Channels *Channels
	HTTP     *http.Client
}

func (r ProtocolRegistry) headers(id string) (map[string]string, error) {
	values := map[string]string{}
	err := r.Channels.UseHeaders(id, func(headers map[string]string) error {
		for name, value := range headers {
			values[name] = value
		}
		return nil
	})
	return values, err
}

func (r ProtocolRegistry) Chat(model Model) (protocol.Chat, error) {
	var selected *Channel
	for _, channel := range r.Channels.List() {
		if channel.ID == model.Channel {
			value := channel
			selected = &value
			break
		}
	}
	if selected == nil {
		return nil, errors.New("模型渠道不存在")
	}
	if selected.Disabled {
		return nil, errors.New("模型渠道已停用")
	}
	headers, err := r.headers(selected.ID)
	if err != nil {
		return nil, err
	}
	client := r.HTTP
	if selected.TimeoutSeconds > 0 {
		copy := *r.HTTP
		copy.Timeout = time.Duration(selected.TimeoutSeconds) * time.Second
		client = &copy
	}
	var result protocol.Chat
	err = r.Channels.UseKey(selected.ID, func(key string) error {
		switch selected.Protocol {
		case "openai":
			result = protocol.OpenAI{BaseURL: selected.BaseURL, Key: key, Client: client, Headers: headers, StreamTimeout: time.Duration(selected.StreamTimeoutSeconds) * time.Second}
		case "anthropic":
			result = protocol.Anthropic{BaseURL: selected.BaseURL, Key: key, Client: client, Headers: headers, StreamTimeout: time.Duration(selected.StreamTimeoutSeconds) * time.Second}
		case "gemini":
			result = protocol.Gemini{BaseURL: selected.BaseURL, Key: key, Client: client, Headers: headers, StreamTimeout: time.Duration(selected.StreamTimeoutSeconds) * time.Second}
		default:
			return errors.New("渠道不支持对话")
		}
		return nil
	})
	return result, err
}
func (r ProtocolRegistry) Embedder(model Model) (protocol.Embedder, error) {
	var selected *Channel
	for _, channel := range r.Channels.List() {
		if channel.ID == model.Channel {
			value := channel
			selected = &value
			break
		}
	}
	if selected == nil {
		return nil, errors.New("模型渠道不存在")
	}
	if selected.Disabled {
		return nil, errors.New("模型渠道已停用")
	}
	headers, err := r.headers(selected.ID)
	if err != nil {
		return nil, err
	}
	client := r.HTTP
	if selected.TimeoutSeconds > 0 {
		copy := *r.HTTP
		copy.Timeout = time.Duration(selected.TimeoutSeconds) * time.Second
		client = &copy
	}
	var result protocol.Embedder
	err = r.Channels.UseKey(selected.ID, func(key string) error {
		switch selected.Protocol {
		case "openai":
			result = protocol.OpenAI{BaseURL: selected.BaseURL, Key: key, Client: client, Headers: headers, StreamTimeout: time.Duration(selected.StreamTimeoutSeconds) * time.Second}
		case "gemini":
			result = protocol.Gemini{BaseURL: selected.BaseURL, Key: key, Client: client, Headers: headers, StreamTimeout: time.Duration(selected.StreamTimeoutSeconds) * time.Second}
		default:
			return errors.New("渠道不支持嵌入")
		}
		return nil
	})
	return result, err
}

func (r ProtocolRegistry) Reranker(model Model) (protocol.Reranker, error) {
	var selected *Channel
	for _, channel := range r.Channels.List() {
		if channel.ID == model.Channel {
			value := channel
			selected = &value
			break
		}
	}
	if selected == nil {
		return nil, errors.New("模型渠道不存在")
	}
	if selected.Disabled {
		return nil, errors.New("模型渠道已停用")
	}
	headers, err := r.headers(selected.ID)
	if err != nil {
		return nil, err
	}
	client := r.HTTP
	if selected.TimeoutSeconds > 0 {
		copy := *r.HTTP
		copy.Timeout = time.Duration(selected.TimeoutSeconds) * time.Second
		client = &copy
	}
	var result protocol.Reranker
	err = r.Channels.UseKey(selected.ID, func(key string) error {
		if selected.Protocol != "rerank" {
			return errors.New("渠道不支持重排")
		}
		result = protocol.CohereRerank{BaseURL: selected.BaseURL, Key: key, Client: client, Headers: headers}
		return nil
	})
	return result, err
}

type UsageWriter interface {
	WriteUsage(context.Context, string, Model, protocol.Usage, float64, string) error
}
type Client struct {
	Tiers     *TierStore
	Protocols Protocols
	Usage     UsageWriter
}

func NewClient(t *TierStore, p Protocols, u UsageWriter) *Client {
	return &Client{Tiers: t, Protocols: p, Usage: u}
}
func (r ProtocolRegistry) Acquire(ctx context.Context, channel string) (func(), error) {
	return r.Channels.Acquire(ctx, channel)
}
func (c *Client) Complete(ctx context.Context, task string, messages []protocol.Message) (protocol.Response, error) {
	return c.complete(ctx, task, messages, nil)
}
func (c *Client) CompleteValidated(ctx context.Context, task string, messages []protocol.Message, validate func(protocol.Response) error) (protocol.Response, error) {
	return c.complete(ctx, task, messages, validate)
}
func (c *Client) complete(ctx context.Context, task string, messages []protocol.Message, validate func(protocol.Response) error) (protocol.Response, error) {
	definition, ok := aitask.Lookup(task)
	if !ok {
		return protocol.Response{}, errors.New("未知 AI 任务")
	}
	models, err := c.Tiers.Resolve(task)
	if err != nil {
		return protocol.Response{}, err
	}
	var joined error
	for modelIndex, model := range models {
		if err := ctx.Err(); err != nil {
			return protocol.Response{}, errors.Join(joined, err)
		}
		chat, err := c.Protocols.Chat(model)
		if err != nil {
			joined = errors.Join(joined, err)
			continue
		}
		maximum := definition.MaxOutput
		if model.MaxOutput > 0 {
			maximum = min(maximum, model.MaxOutput)
		}
		if limit, ok := ctx.Value(outputKey{}).(int); ok && limit > 0 {
			maximum = min(maximum, limit)
		}
		input := int64(0)
		for _, message := range messages {
			input += int64(len(message.Content) + len(message.Role) + 64)
		}
		attempts := 1
		if validate != nil {
			attempts = 2
		}
		for number := 0; number < attempts; number++ {
			release := func() {}
			if limiter, ok := c.Protocols.(channelLimiter); ok {
				release, err = limiter.Acquire(ctx, model.Channel)
				if err != nil {
					joined = errors.Join(joined, err)
					break
				}
			}
			settle, reserveErr := Reserve(ctx, Cost(model, protocol.Usage{InputTokens: input, OutputTokens: int64(maximum)}))
			if reserveErr != nil {
				release()
				return protocol.Response{}, reserveErr
			}
			attempt, cancel := context.WithTimeout(ctx, 90*time.Second)
			value, callErr := chat.Complete(attempt, protocol.Request{Model: model.ID, Messages: messages, MaxTokens: maximum})
			cancel()
			release()
			usage := value.Usage
			status := "success"
			if callErr != nil {
				status = "error"
			}
			if usage.InputTokens == 0 && usage.OutputTokens == 0 {
				usage = protocol.Usage{InputTokens: input, OutputTokens: int64(maximum)}
				status = "estimated_" + status
			}
			cost := Cost(model, usage)
			settle(cost)
			if c.Usage != nil {
				accounting, cancelAccounting := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				usageErr := c.Usage.WriteUsage(accounting, task, model, usage, cost, status)
				cancelAccounting()
				if usageErr != nil {
					return protocol.Response{}, errors.Join(callErr, usageErr)
				}
			}
			if callErr != nil {
				joined = errors.Join(joined, callErr)
				if ctx.Err() != nil {
					return protocol.Response{}, errors.Join(joined, ctx.Err())
				}
				break
			}
			if validate != nil {
				if validationErr := validate(value); validationErr != nil {
					joined = errors.Join(joined, validationErr)
					continue
				}
			}
			return value, nil
		}
		if modelIndex+1 < len(models) {
			delay := time.Duration(250*(1<<min(modelIndex, 3))) * time.Millisecond
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return protocol.Response{}, errors.Join(joined, ctx.Err())
			case <-timer.C:
			}
		}
	}
	return protocol.Response{}, joined
}

func (c *Client) Stream(ctx context.Context, task string, messages []protocol.Message, emit func(protocol.Chunk) error) (protocol.Response, error) {
	definition, ok := aitask.Lookup(task)
	if !ok {
		return protocol.Response{}, errors.New("未知 AI 任务")
	}
	models, err := c.Tiers.Resolve(task)
	if err != nil {
		return protocol.Response{}, err
	}
	var joined error
	for modelIndex, model := range models {
		if err := ctx.Err(); err != nil {
			return protocol.Response{}, errors.Join(joined, err)
		}
		chat, err := c.Protocols.Chat(model)
		if err != nil {
			joined = errors.Join(joined, err)
			continue
		}
		maximum := definition.MaxOutput
		if model.MaxOutput > 0 {
			maximum = min(maximum, model.MaxOutput)
		}
		if limit, ok := ctx.Value(outputKey{}).(int); ok && limit > 0 {
			maximum = min(maximum, limit)
		}
		input := int64(0)
		for _, message := range messages {
			input += int64(len(message.Content) + len(message.Role) + 64)
		}
		release := func() {}
		if limiter, ok := c.Protocols.(channelLimiter); ok {
			release, err = limiter.Acquire(ctx, model.Channel)
			if err != nil {
				joined = errors.Join(joined, err)
				continue
			}
		}
		settle, reserveErr := Reserve(ctx, Cost(model, protocol.Usage{InputTokens: input, OutputTokens: int64(maximum)}))
		if reserveErr != nil {
			release()
			return protocol.Response{}, reserveErr
		}
		var text strings.Builder
		usage := protocol.Usage{}
		started := false
		attempt, cancel := context.WithTimeout(ctx, 5*time.Minute)
		callErr := chat.Stream(attempt, protocol.Request{Model: model.ID, Messages: messages, MaxTokens: maximum}, func(chunk protocol.Chunk) error {
			if chunk.Usage.InputTokens > 0 || chunk.Usage.OutputTokens > 0 {
				usage = chunk.Usage
			}
			if chunk.Text != "" {
				started = true
				text.WriteString(chunk.Text)
			}
			return emit(chunk)
		})
		cancel()
		release()
		status := "success"
		if callErr != nil {
			status = "error"
		}
		if usage.InputTokens == 0 && usage.OutputTokens == 0 {
			usage.InputTokens = input
			if callErr == nil {
				usage.OutputTokens = int64(len(text.String())/3 + 1)
			} else {
				usage.OutputTokens = int64(maximum)
			}
			status = "estimated_" + status
		}
		cost := Cost(model, usage)
		settle(cost)
		if c.Usage != nil {
			accounting, cancelAccounting := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			usageErr := c.Usage.WriteUsage(accounting, task, model, usage, cost, status)
			cancelAccounting()
			if usageErr != nil {
				return protocol.Response{}, errors.Join(callErr, usageErr)
			}
		}
		response := protocol.Response{Text: text.String(), Usage: usage, Model: model.ID}
		if callErr == nil {
			if strings.TrimSpace(response.Text) == "" {
				joined = errors.Join(joined, errors.New("模型响应缺少内容"))
			} else {
				return response, nil
			}
		} else {
			joined = errors.Join(joined, callErr)
			if started || ctx.Err() != nil {
				return response, errors.Join(joined, ctx.Err())
			}
		}
		if modelIndex+1 < len(models) {
			timer := time.NewTimer(250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return response, errors.Join(joined, ctx.Err())
			case <-timer.C:
			}
		}
	}
	return protocol.Response{}, joined
}
