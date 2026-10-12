package guard

import (
	"errors"
	"sync"
	"time"
)

type bucket struct {
	count  int
	reset  time.Time
	active bool
}
type Limiter struct {
	mu        sync.Mutex
	values    map[string]bucket
	PerMinute int
}

func NewLimiter(perMinute int) *Limiter {
	return &Limiter{values: map[string]bucket{}, PerMinute: perMinute}
}
func (l *Limiter) Begin(key string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	v := l.values[key]
	if now.After(v.reset) {
		v = bucket{reset: now.Add(time.Minute)}
	}
	if v.active {
		return errors.New("同一会话已有问题正在处理")
	}
	if v.count >= l.PerMinute {
		return errors.New("提问过于频繁")
	}
	v.count++
	v.active = true
	l.values[key] = v
	return nil
}
func (l *Limiter) End(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	v := l.values[key]
	v.active = false
	l.values[key] = v
}
