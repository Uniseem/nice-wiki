package llm

type Capability string

const (
	Chat       Capability = "chat"
	Vision     Capability = "vision"
	Structured Capability = "structured"
	Embedding  Capability = "embedding"
	Rerank     Capability = "rerank"
)

type Model struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Channel      string       `json:"channel"`
	Capabilities []Capability `json:"capabilities"`
	InputPrice   float64      `json:"inputPrice"`
	OutputPrice  float64      `json:"outputPrice"`
	Context      int          `json:"context"`
	MaxOutput    int          `json:"maxOutput"`
}

func (m Model) Has(want Capability) bool {
	for _, v := range m.Capabilities {
		if v == want {
			return true
		}
	}
	return false
}
