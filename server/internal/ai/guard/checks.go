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
	escalated map[string]time.Time
}

func NewChecks() *Checks {
	return &Checks{last: map[string]time.Time{}, questions: map[string]string{}, escalated: map[string]time.Time{}}
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
func (c *Checks) Escalate(key string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.escalated[key] = now.Add(10 * time.Minute)
}
func (c *Checks) Difficulty(key string, now time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	until, ok := c.escalated[key]
	if !ok || !now.Before(until) {
		delete(c.escalated, key)
		return 2
	}
	return 3
}
func (c *Checks) Reset(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.last, key)
	delete(c.questions, key)
	delete(c.escalated, key)
}
