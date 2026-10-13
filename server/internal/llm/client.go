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
		maximum := definition.MaxOutput
		if model.MaxOutput > 0 {
			maximum = min(maximum, model.MaxOutput)
		}
		if limit, ok := ctx.Value(outputKey{}).(int); ok && limit > 0 {
			maximum = min(maximum, limit)
		}
		// 字节数作为输入 token 的保守上界，并计入消息封装开销。
		input := int64(0)
		for _, message := range messages {
			input += int64(len(message.Content) + len(message.Role) + 64)
		}
		settle, reserveErr := Reserve(ctx, Cost(model, protocol.Usage{InputTokens: input, OutputTokens: int64(maximum)}))
		if reserveErr != nil {
			return protocol.Response{}, reserveErr
		}
		attempt, cancel := context.WithTimeout(ctx, 90*time.Second)
		value, err := chat.Complete(attempt, protocol.Request{Model: model.ID, Messages: messages, MaxTokens: maximum})
		cancel()
		usage := value.Usage
		status := "success"
		if err != nil {
			status = "error"
		}
		if usage.InputTokens == 0 && usage.OutputTokens == 0 {
			usage = protocol.Usage{InputTokens: input, OutputTokens: int64(maximum)}
			status = "estimated_" + status
		}
		cost := Cost(model, usage)
		settle(cost)
		if c.Usage != nil {
			// 请求被取消后仍保存已发生或无法确认的费用，不记录请求正文。
			accounting, cancelAccounting := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			usageErr := c.Usage.WriteUsage(accounting, task, model, usage, cost, status)
			cancelAccounting()
			if usageErr != nil {
				return protocol.Response{}, errors.Join(err, usageErr)
			}
		}
		if err != nil {
			joined = errors.Join(joined, err)
			continue
		}
		return value, nil
	}
	return protocol.Response{}, joined
}
