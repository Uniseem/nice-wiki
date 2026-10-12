package protocol

import "context"

type Message struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
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
