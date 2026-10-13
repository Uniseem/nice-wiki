package qa

import (
	"errors"
	"sync"
	"time"
)

type Turn struct {
	Role     string    `json:"role"`
	Text     string    `json:"text"`
	Created  time.Time `json:"createdAt"`
	RecordID string    `json:"recordId,omitempty"`
}
type Conversation struct {
	ID      string    `json:"id"`
	Summary string    `json:"summary"`
	Turns   []Turn    `json:"turns"`
	Updated time.Time `json:"-"`
}
type Conversations struct {
	mu        sync.Mutex
	values    map[string]Conversation
	Retention time.Duration
}

func NewConversations(retention time.Duration) *Conversations {
	return &Conversations{values: map[string]Conversation{}, Retention: retention}
}
func (s *Conversations) Get(id string) (Conversation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.values[id]
	if ok && s.Retention > 0 && time.Since(v.Updated) > s.Retention {
		delete(s.values, id)
		return Conversation{}, false
	}
	v.Turns = append([]Turn{}, v.Turns...)
	return v, ok
}
func (s *Conversations) Append(id string, turn Turn) error {
	if id == "" || turn.Text == "" {
		return errors.New("对话或消息无效")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.values[id]
	v.ID = id
	v.Turns = append(v.Turns, turn)
	v.Updated = time.Now()
	s.values[id] = v
	return nil
}
func (s *Conversations) Put(value Conversation) error {
	if value.ID == "" {
		return errors.New("对话无效")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value.Turns = append([]Turn{}, value.Turns...)
	value.Updated = time.Now()
	s.values[value.ID] = value
	return nil
}
func (s *Conversations) ReplaceSummary(id, summary string, keep []Turn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.values[id]
	v.Summary = summary
	v.Turns = append([]Turn{}, keep...)
	v.Updated = time.Now()
	s.values[id] = v
}
func (s *Conversations) Clear(id string) { s.mu.Lock(); defer s.mu.Unlock(); delete(s.values, id) }
