package site

import (
	"sync"
	"time"
)

type cachedPage struct {
	Body []byte
	ETag string
	Used time.Time
}

// Cache 仅保存可重建的公开 HTML，容量有限，不缓存登录预览或错误响应。
type Cache struct {
	mu       sync.Mutex
	capacity int
	values   map[string]cachedPage
}

func NewCache(capacity int) *Cache {
	if capacity < 1 {
		capacity = 128
	}
	return &Cache{capacity: capacity, values: map[string]cachedPage{}}
}
func (c *Cache) Get(key string) ([]byte, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.values[key]
	if !ok {
		return nil, "", false
	}
	value.Used = time.Now()
	c.values[key] = value
	return append([]byte{}, value.Body...), value.ETag, true
}
func (c *Cache) Set(key string, body []byte, etag string) {
	if len(body) > 8<<20 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.values[key]; !exists && len(c.values) >= c.capacity {
		oldestKey := ""
		var oldest time.Time
		for candidate, value := range c.values {
			if oldestKey == "" || value.Used.Before(oldest) {
				oldestKey = candidate
				oldest = value.Used
			}
		}
		delete(c.values, oldestKey)
	}
	c.values[key] = cachedPage{Body: append([]byte{}, body...), ETag: etag, Used: time.Now()}
}
func (c *Cache) Clear() { c.mu.Lock(); defer c.mu.Unlock(); c.values = map[string]cachedPage{} }
