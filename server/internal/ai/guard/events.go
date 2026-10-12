package guard

import (
	"sync"
	"time"
)

type Event struct {
	Scope, Reason string
	Risk          Risk
	Created       time.Time
}
type Events struct {
	mu     sync.Mutex
	values []Event
}

func (e *Events) Add(value Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.values = append(e.values, value)
}
func (e *Events) List(limit int) []Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	if limit < 1 || limit > len(e.values) {
		limit = len(e.values)
	}
	return append([]Event{}, e.values[len(e.values)-limit:]...)
}
