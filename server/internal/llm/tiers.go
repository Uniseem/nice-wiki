package llm

import (
	"errors"
	"nice-wiki/internal/aitask"
	"sync"
)

type TierStore struct {
	mu     sync.RWMutex
	models map[aitask.Tier][]Model
	Source func(aitask.Tier) []Model
}

func NewTiers() *TierStore { return &TierStore{models: map[aitask.Tier][]Model{}} }
func (s *TierStore) Set(tier aitask.Tier, values []Model) error {
	if len(values) == 0 {
		return errors.New("档位至少需要一个模型")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.models[tier] = append([]Model{}, values...)
	return nil
}
func (s *TierStore) Resolve(task string) ([]Model, error) {
	definition, ok := aitask.Lookup(task)
	if !ok {
		return nil, errors.New("未知 AI 任务")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := append([]Model{}, s.models[definition.Tier]...)
	if s.Source != nil {
		values = s.Source(definition.Tier)
	}
	if len(values) == 0 {
		return nil, errors.New("任务所需模型档位尚未配置")
	}
	return values, nil
}
