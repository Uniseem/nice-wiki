package llm

import "nice-wiki/internal/llm/protocol"

func Cost(model Model, usage protocol.Usage) float64 {
	return float64(usage.InputTokens)/1_000_000*model.InputPrice + float64(usage.OutputTokens)/1_000_000*model.OutputPrice
}
