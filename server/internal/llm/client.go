package llm

import (
	"context"
	"errors"
	"net/http"
	"nice-wiki/internal/aitask"
	"nice-wiki/internal/llm/protocol"
	"time"
)

type Protocols interface {
	Chat(Model) (protocol.Chat, error)
}
type ProtocolRegistry struct {
	Channels *Channels
	HTTP     *http.Client
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
	var result protocol.Chat
	err := r.Channels.UseKey(selected.ID, func(key string) error {
		switch selected.Protocol {
		case "openai":
			result = protocol.OpenAI{BaseURL: selected.BaseURL, Key: key, Client: r.HTTP}
		case "anthropic":
			result = protocol.Anthropic{BaseURL: selected.BaseURL, Key: key, Client: r.HTTP}
		case "gemini":
			result = protocol.Gemini{BaseURL: selected.BaseURL, Key: key, Client: r.HTTP}
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
	var result protocol.Embedder
	err := r.Channels.UseKey(selected.ID, func(key string) error {
		switch selected.Protocol {
		case "openai":
			result = protocol.OpenAI{BaseURL: selected.BaseURL, Key: key, Client: r.HTTP}
		case "gemini":
			result = protocol.Gemini{BaseURL: selected.BaseURL, Key: key, Client: r.HTTP}
		default:
			return errors.New("渠道不支持嵌入")
		}
		return nil
	})
	return result, err
}

type UsageWriter interface {
	WriteUsage(context.Context, string, Model, protocol.Usage, float64) error
}
type Client struct {
	Tiers     *TierStore
	Protocols Protocols
	Usage     UsageWriter
}

func NewClient(t *TierStore, p Protocols, u UsageWriter) *Client {
	return &Client{Tiers: t, Protocols: p, Usage: u}
}
func (c *Client) Complete(ctx context.Context, task string, messages []protocol.Message) (protocol.Response, error) {
	definition, ok := aitask.Lookup(task)
	if !ok {
		return protocol.Response{}, errors.New("未知 AI 任务")
	}
	models, err := c.Tiers.Resolve(task)
	if err != nil {
		return protocol.Response{}, err
	}
	var joined error
	for _, model := range models {
		chat, err := c.Protocols.Chat(model)
		if err != nil {
			joined = errors.Join(joined, err)
			continue
		}
		attempt, cancel := context.WithTimeout(ctx, 90*time.Second)
		value, err := chat.Complete(attempt, protocol.Request{Model: model.ID, Messages: messages, MaxTokens: definition.MaxOutput})
		cancel()
		if err != nil {
			joined = errors.Join(joined, err)
			continue
		}
		if c.Usage != nil {
			if err := c.Usage.WriteUsage(ctx, task, model, value.Usage, Cost(model, value.Usage)); err != nil {
				return protocol.Response{}, err
			}
		}
		return value, nil
	}
	return protocol.Response{}, joined
}
