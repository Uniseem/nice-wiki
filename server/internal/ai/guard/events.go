package guard

import (
	"sync"
	"time"
)

type Event struct {
	Scope   string    `json:"scope"`
	Reason  string    `json:"reason"`
	Risk    Risk      `json:"risk"`
	Created time.Time `json:"created"`
}
type Events struct {
	mu     sync.Mutex
	values []Event
}

func (e *Events) Add(value Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.values = append(e.values, value)
	if len(e.values) > 1000 {
		e.values = append([]Event{}, e.values[len(e.values)-1000:]...)
	}
}
func (e *Events) List(limit int) []Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	if limit < 1 || limit > len(e.values) {
		limit = len(e.values)
	}
	return append([]Event{}, e.values[len(e.values)-limit:]...)
}
