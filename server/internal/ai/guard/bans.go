package guard

import (
	"sync"
	"time"
)

type Bans struct {
	mu    sync.Mutex
	until map[string]time.Time
}

func NewBans() *Bans { return &Bans{until: map[string]time.Time{}} }
func (b *Bans) Ban(key string, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.until[key] = now.Add(10 * time.Minute)
}
func (b *Bans) Blocked(key string, now time.Time) (time.Time, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	until, ok := b.until[key]
	if ok && now.After(until) {
		delete(b.until, key)
		return time.Time{}, false
	}
	return until, ok
}
func (b *Bans) Unban(key string) { b.mu.Lock(); defer b.mu.Unlock(); delete(b.until, key) }
