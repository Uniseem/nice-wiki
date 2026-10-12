package content

import (
	"sync"
	"time"
)

type EditLock struct {
	DocumentID  string `json:"documentId"`
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	Token       string `json:"token"`
	ExpiresAt   int64  `json:"expiresAt"`
}
type Locks struct {
	mu     sync.Mutex
	values map[string]EditLock
}

func NewLocks() *Locks { return &Locks{values: map[string]EditLock{}} }
func (s *Locks) Acquire(id, account, name string) (EditLock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	if old, ok := s.values[id]; ok && old.ExpiresAt > now {
		if old.AccountID != account {
			return EditLock{}, ErrLocked
		}
		old.ExpiresAt = now + 120
		s.values[id] = old
		return old, nil
	}
	token, err := newID()
	if err != nil {
		return EditLock{}, err
	}
	value := EditLock{DocumentID: id, AccountID: account, AccountName: name, Token: token, ExpiresAt: now + 120}
	s.values[id] = value
	return value, nil
}
func (s *Locks) Heartbeat(id, account, token string) (EditLock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[id]
	if !ok || value.AccountID != account || value.Token != token || value.ExpiresAt <= time.Now().Unix() {
		return EditLock{}, ErrLocked
	}
	value.ExpiresAt = time.Now().Unix() + 120
	s.values[id] = value
	return value, nil
}
func (s *Locks) Check(id, account, token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[id]
	return ok && value.AccountID == account && value.Token == token && value.ExpiresAt > time.Now().Unix()
}
func (s *Locks) Release(id, account, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[id]
	if !ok {
		return nil
	}
	if value.AccountID != account || value.Token != token {
		return ErrLocked
	}
	delete(s.values, id)
	return nil
}
func (s *Locks) ForceRelease(id string) { s.mu.Lock(); defer s.mu.Unlock(); delete(s.values, id) }
