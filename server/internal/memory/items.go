package memory

type Source string

const (
	SourceManual    Source = "manual"
	SourceAssistant Source = "assistant"
	SourceAutomatic Source = "automatic"
)

type Item struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
	Source      Source   `json:"source"`
	ReadOnly    bool     `json:"readOnly"`
	Limit       int      `json:"limit"`
	Content     string   `json:"content,omitempty"`
}

var Builtins = []Item{{ID: "project-overview", Title: "项目概述", Description: "产品、受众与核心能力", Source: SourceAutomatic, Limit: 1536}, {ID: "terminology", Title: "术语与命名", Description: "全站统一术语", Source: SourceManual, Limit: 1536}, {ID: "writing-style", Title: "写作风格", Description: "回答、翻译与文案的风格约定", Source: SourceManual, Limit: 1536}}
