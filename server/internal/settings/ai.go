package settings

import (
	"nice-wiki/internal/fsutil"
	"strings"
)

type AI struct {
	Enabled           bool    `json:"enabled"`
	ExactCache        bool    `json:"exactCache"`
	SemanticCache     bool    `json:"semanticCache"`
	SemanticCacheMode string  `json:"semanticCacheMode"`
	SemanticThreshold float64 `json:"semanticThreshold"`
	MaxQuestionLength int     `json:"maxQuestionLength"`
	MaxTurns          int     `json:"maxTurns"`
	MaxOutputTokens   int     `json:"maxOutputTokens"`
	DailySpendLimit   float64 `json:"dailySpendLimit"`
}

func DefaultAI() AI {
	return AI{ExactCache: true, SemanticCache: true, SemanticCacheMode: "ai", SemanticThreshold: 0.90, MaxQuestionLength: 500, MaxTurns: 20, MaxOutputTokens: 1500}
}
func (v AI) Validate() error {
	if v.MaxQuestionLength < 1 || v.MaxQuestionLength > 10000 || v.MaxTurns < 1 || v.MaxTurns > 100 || v.MaxOutputTokens < 1 || v.MaxOutputTokens > 32768 || v.DailySpendLimit < 0 || v.SemanticCacheMode != "ai" && v.SemanticCacheMode != "threshold" || v.SemanticThreshold < 0.5 || v.SemanticThreshold > 0.999 {
		return invalid("AI 设置范围无效")
	}
	return nil
}
func (s *Store) AI() AI { s.mu.RLock(); defer s.mu.RUnlock(); return s.ai }
func (s *Store) SaveAI(v AI) error {
	if err := v.Validate(); err != nil {
		return err
	}
	if v.Enabled {
		s.mu.RLock()
		ready := s.aiReady
		s.mu.RUnlock()
		if ready == nil || !ready() {
			return invalid("快速、精确与嵌入模型尚未配置完整，不能启用 AI")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fsutil.WriteJSON(s.layout.AI(), aiFile{AI: v, ModelTiers: cloneModelTiers(s.modelTiers)}); err != nil {
		return err
	}
	s.ai = v
	return nil
}

func (s *Store) ModelTiers() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneModelTiers(s.modelTiers)
}

func validateModelTiers(values map[string][]string) error {
	allowed := map[string]bool{"fast": true, "smart": true, "vision": true, "embedding": true, "rerank": true}
	for tier, models := range values {
		if !allowed[tier] || len(models) > 8 {
			return invalid("模型档位配置无效")
		}
		seen := map[string]bool{}
		for _, model := range models {
			if strings.TrimSpace(model) == "" || len(model) > 512 || seen[model] {
				return invalid("模型档位配置无效")
			}
			seen[model] = true
		}
	}
	return nil
}

func (s *Store) SaveModelTiers(values map[string][]string) error {
	if err := validateModelTiers(values); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneModelTiers(values)
	if err := fsutil.WriteJSON(s.layout.AI(), aiFile{AI: s.ai, ModelTiers: next}); err != nil {
		return err
	}
	s.modelTiers = next
	return nil
}
