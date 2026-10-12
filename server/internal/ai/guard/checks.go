package guard

import (
	"strings"
	"sync"
	"time"
)

type Signal struct{ Frequency, Repetition, Regular bool }
type Checks struct {
	mu        sync.Mutex
	last      map[string]time.Time
	questions map[string]string
}

func NewChecks() *Checks {
	return &Checks{last: map[string]time.Time{}, questions: map[string]string{}}
}
func (c *Checks) Inspect(key, question string, now time.Time) Signal {
	c.mu.Lock()
	defer c.mu.Unlock()
	previous := c.last[key]
	old := c.questions[key]
	signal := Signal{Frequency: !previous.IsZero() && now.Sub(previous) < 2*time.Second, Repetition: old != "" && strings.EqualFold(strings.TrimSpace(old), strings.TrimSpace(question))}
	c.last[key] = now
	c.questions[key] = question
	return signal
}
