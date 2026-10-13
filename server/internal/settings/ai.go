package settings

import (
	"nice-wiki/internal/fsutil"
)

type AI struct {
	Enabled           bool    `json:"enabled"`
	ExactCache        bool    `json:"exactCache"`
	SemanticCache     bool    `json:"semanticCache"`
	MaxQuestionLength int     `json:"maxQuestionLength"`
	MaxTurns          int     `json:"maxTurns"`
	MaxOutputTokens   int     `json:"maxOutputTokens"`
	DailySpendLimit   float64 `json:"dailySpendLimit"`
}

func DefaultAI() AI {
	return AI{ExactCache: true, SemanticCache: true, MaxQuestionLength: 2000, MaxTurns: 20, MaxOutputTokens: 2048}
}
func (v AI) Validate() error {
	if v.MaxQuestionLength < 1 || v.MaxQuestionLength > 10000 || v.MaxTurns < 1 || v.MaxTurns > 100 || v.MaxOutputTokens < 1 || v.MaxOutputTokens > 32768 || v.DailySpendLimit < 0 {
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
	if err := fsutil.WriteJSON(s.layout.AI(), v); err != nil {
		return err
	}
	s.ai = v
	return nil
}
