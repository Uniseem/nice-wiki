package memory

import (
	"errors"
	"sync"
	"time"
)

type Change string

const (
	Add       Change = "add"
	Update    Change = "update"
	Delete    Change = "delete"
	Unchanged Change = "unchanged"
)

type Proposal struct {
	ID     string `json:"id"`
	Change Change `json:"change"`
	Before *Item  `json:"before,omitempty"`
	After  *Item  `json:"after,omitempty"`
	Reason string `json:"reason"`
}
type Proposals struct {
	mu     sync.Mutex
	values map[string]pendingProposals
}
type pendingProposals struct {
	Values  []Proposal
	Expires time.Time
}

func NewProposals() *Proposals { return &Proposals{values: map[string]pendingProposals{}} }
func (p *Proposals) Put(conversation string, values []Proposal) error {
	for _, v := range values {
		if v.ID == "" {
			return errors.New("记忆提议缺少 ID")
		}
		if v.Before != nil && v.Before.ReadOnly && v.Change != Unchanged {
			return errors.New("只读记忆不能修改")
		}
		if v.After != nil {
			if err := Validate(*v.After); err != nil {
				return err
			}
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.values[conversation] = pendingProposals{Values: append([]Proposal{}, values...), Expires: time.Now().Add(30 * time.Minute)}
	return nil
}
func (p *Proposals) Get(conversation string) []Proposal {
	p.mu.Lock()
	defer p.mu.Unlock()
	value, ok := p.values[conversation]
	if !ok || time.Now().After(value.Expires) {
		delete(p.values, conversation)
		return nil
	}
	return append([]Proposal{}, value.Values...)
}
func (p *Proposals) Clear(conversation string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.values, conversation)
}
