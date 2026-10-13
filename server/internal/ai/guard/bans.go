package guard

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"strings"
	"sync"
	"time"
)

func IPScope(secret []byte, raw string) string {
	address, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return "ip:unknown"
	}
	address = address.Unmap()
	value := address.String()
	if address.Is6() {
		value = netip.PrefixFrom(address, 64).Masked().String()
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(value))
	return "ip:" + hex.EncodeToString(mac.Sum(nil))
}

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
