package llm

import "sort"

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

func AvailableModels(catalog *Catalog, channels *Channels) []Model {
	configured := channels.List()
	byID := map[string]Channel{}
	for _, channel := range configured {
		if !channel.Disabled && channels.HasKey(channel.ID) {
			byID[channel.ID] = channel
		}
	}
	result := map[string]Model{}
	models := catalog.List()
	for _, model := range models {
		for _, channel := range byID {
			if channel.Provider != "" && channel.Provider == model.Channel {
				adapted := model
				adapted.Channel = channel.ID
				result[adapted.Channel+":"+adapted.ID] = adapted
			}
		}
	}
	// 以实际渠道 ID 保存的手工资料最后覆盖 provider 快照。
	for _, model := range models {
		if _, ok := byID[model.Channel]; ok {
			result[model.Channel+":"+model.ID] = model
		}
	}
	values := make([]Model, 0, len(result))
	for _, model := range result {
		values = append(values, model)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Channel+":"+values[i].ID < values[j].Channel+":"+values[j].ID })
	return values
}
