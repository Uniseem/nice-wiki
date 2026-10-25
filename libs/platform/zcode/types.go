// Package zcode 连接原生 ZCode 运行时与 owning-service 的模型、工具端口。
package zcode

import (
	"encoding/json"
	"nice-wiki/libs/platform"
)

type Model struct {
	ProviderID      string `json:"providerId"`
	ModelID         string `json:"modelId"`
	ContextWindow   int    `json:"contextWindow"`
	MaxOutputTokens int    `json:"maxOutputTokens"`
}

type Input struct {
	Model     Model                     `json:"model"`
	Language  string                    `json:"language"`
	Messages  []platform.GatewayMessage `json:"messages"`
	Tools     []platform.GatewayTool    `json:"tools"`
	TurnStart int                       `json:"turnStart"`
	State     string                    `json:"state,omitempty"`
}

type Frame struct {
	RunID    string `json:"runId,omitempty"`
	Sequence int    `json:"sequence,omitempty"`
	Kind     string `json:"kind"`
	ID       string `json:"id,omitempty"`
	Payload  string `json:"payload"`
}

type Events struct {
	Frames []Frame `json:"frames"`
	Done   bool    `json:"done"`
}

type Invocation struct {
	Public              bool                      `json:"public"`
	Messages            []platform.GatewayMessage `json:"messages"`
	Tools               []platform.GatewayTool    `json:"tools,omitempty"`
	MaximumOutputTokens int                       `json:"maximumOutputTokens"`
	ToolChoice          string                    `json:"toolChoice"`
}

type ToolRequest struct {
	Call         platform.GatewayToolCall `json:"call"`
	MaximumBytes int                      `json:"maximumBytes"`
}

type Result struct {
	Text      string                    `json:"text"`
	State     string                    `json:"state"`
	Messages  []platform.GatewayMessage `json:"-"`
	Rounds    int                       `json:"-"`
	ToolCalls int                       `json:"-"`
}

type InvokeResult struct {
	Output platform.ModelOutput `json:"output"`
	Usage  map[string]int       `json:"usage,omitempty"`
	Model  *Model               `json:"model,omitempty"`
}

type Reply struct {
	Payload string `json:"payload"`
}

func payload(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
