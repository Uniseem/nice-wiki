package config

import (
	"errors"
	"nice-wiki/internal/fsutil"
	"os"
	"sync"
)

// Secrets 没有公开 JSON 读取接口；调用方只能按用途取用单个密钥。
type Secrets struct {
	mu     sync.RWMutex
	path   string
	values map[string]string
}

func OpenSecrets(path string) (*Secrets, error) {
	values := map[string]string{}
	if err := fsutil.ReadJSON(path, &values); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return &Secrets{path: path, values: values}, nil
}
func (s *Secrets) Use(key string, fn func(string) error) error {
	s.mu.RLock()
	value, ok := s.values[key]
	s.mu.RUnlock()
	if !ok {
		return errors.New("密钥尚未配置")
	}
	return fn(value)
}
func (s *Secrets) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[key] != ""
}
func (s *Secrets) Set(key, value string) error {
	if key == "" || len(key) > 128 || len(value) > 16384 {
		return errors.New("密钥参数无效")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := map[string]string{}
	for k, v := range s.values {
		next[k] = v
	}
	if value == "" {
		delete(next, key)
	} else {
		next[key] = value
	}
	if err := fsutil.WriteJSON(s.path, next); err != nil {
		return err
	}
	s.values = next
	return nil
}
