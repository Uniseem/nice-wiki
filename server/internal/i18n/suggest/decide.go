package suggest

import (
	"context"
	"sync"
	"time"
)

type Model interface {
	DecideLanguage(context.Context, Signals, []string) (string, error)
}
type cached struct {
	language string
	expires  time.Time
}
type Decider struct {
	Model Model
	mu    sync.Mutex
	cache map[string]cached
}

func NewDecider(model Model) *Decider { return &Decider{Model: model, cache: map[string]cached{}} }
func (d *Decider) Decide(ctx context.Context, signals Signals, enabled []string, base string) string {
	key := signals.Key()
	d.mu.Lock()
	value, ok := d.cache[key]
	d.mu.Unlock()
	if ok && time.Now().Before(value.expires) {
		return value.language
	}
	language := Rule(signals, enabled, base)
	if d.Model != nil {
		call, cancel := context.WithTimeout(ctx, 2*time.Second)
		candidate, err := d.Model.DecideLanguage(call, signals, enabled)
		cancel()
		if err == nil {
			for _, allowed := range enabled {
				if candidate == allowed {
					language = candidate
				}
			}
		}
	}
	d.mu.Lock()
	if len(d.cache) >= 1000 {
		for candidate, cachedValue := range d.cache {
			if time.Now().After(cachedValue.expires) || len(d.cache) >= 1000 {
				delete(d.cache, candidate)
				break
			}
		}
	}
	d.cache[key] = cached{language: language, expires: time.Now().Add(24 * time.Hour)}
	d.mu.Unlock()
	return language
}
