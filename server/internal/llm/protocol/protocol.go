package protocol

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Message struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

func applyHeaders(request *http.Request, headers map[string]string) {
	for name, value := range headers {
		request.Header.Set(name, value)
	}
}
func parseDataURL(value string) (string, string, error) {
	if !strings.HasPrefix(value, "data:") {
		return "", "", errors.New("图片输入必须使用 data URL")
	}
	head, data, ok := strings.Cut(strings.TrimPrefix(value, "data:"), ",")
	if !ok || !strings.HasSuffix(head, ";base64") {
		return "", "", errors.New("图片 data URL 无效")
	}
	mime := strings.TrimSuffix(head, ";base64")
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(decoded) == 0 || len(decoded) > 32<<20 {
		return "", "", errors.New("图片 data URL 内容无效")
	}
	return mime, data, nil
}

type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"maxTokens"`
	Temperature float64   `json:"temperature"`
	JSON        bool      `json:"json"`
}
type Usage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
}
type Response struct {
	Text  string `json:"text"`
	Usage Usage  `json:"usage"`
	Model string `json:"model"`
}
type Chunk struct {
	Text  string
	Done  bool
	Usage Usage
}
type Chat interface {
	Complete(context.Context, Request) (Response, error)
	Stream(context.Context, Request, func(Chunk) error) error
}
type Embedder interface {
	Embed(context.Context, string, []string) ([][]float32, Usage, error)
}
type Reranker interface {
	Rerank(context.Context, string, string, []string) ([]float64, Usage, error)
}

func idleContext(parent context.Context, timeout time.Duration) (context.Context, func(), func()) {
	if timeout <= 0 {
		return parent, func() {}, func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	activity := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-activity:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(timeout)
			case <-timer.C:
				cancel()
				return
			}
		}
	}()
	var once sync.Once
	stop := func() { once.Do(func() { close(done); cancel() }) }
	touch := func() {
		select {
		case activity <- struct{}{}:
		default:
		}
	}
	return ctx, stop, touch
}
