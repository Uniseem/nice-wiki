package llm

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"nice-wiki/internal/fsutil"
	"os"
	"strings"
	"sync"
)

type KeyStore interface {
	Set(string, string) error
	Has(string) bool
	Use(string, func(string) error) error
}
type Channel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	BaseURL  string `json:"baseURL"`
	Protocol string `json:"protocol"`
}
type Channels struct {
	mu     sync.RWMutex
	path   string
	keys   KeyStore
	values []Channel
}

func OpenChannels(path string, keys KeyStore) (*Channels, error) {
	values := []Channel{}
	if err := fsutil.ReadJSON(path, &values); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, value := range values {
		if err := validateChannel(value); err != nil {
			return nil, err
		}
	}
	return &Channels{path: path, keys: keys, values: values}, nil
}
func validateChannel(v Channel) error {
	if strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 120 || v.Protocol != "openai" && v.Protocol != "anthropic" && v.Protocol != "gemini" && v.Protocol != "rerank" {
		return errors.New("渠道名称或协议无效")
	}
	u, err := url.Parse(v.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("渠道地址必须是无凭据的 HTTP 或 HTTPS 地址")
	}
	return nil
}
func (s *Channels) Count() int { s.mu.RLock(); defer s.mu.RUnlock(); return len(s.values) }
func (s *Channels) List() []Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Channel{}, s.values...)
}
func (s *Channels) HasKey(id string) bool { return s.keys.Has("channel:" + id) }
func (s *Channels) UseKey(id string, fn func(string) error) error {
	return s.keys.Use("channel:"+id, fn)
}
func (s *Channels) Create(name, address, protocol, key string) (Channel, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return Channel{}, err
	}
	v := Channel{ID: hex.EncodeToString(bytes), Name: name, BaseURL: strings.TrimSuffix(address, "/"), Protocol: protocol}
	if err := validateChannel(v); err != nil {
		return Channel{}, err
	}
	if key == "" || len(key) > 16384 {
		return Channel{}, errors.New("渠道密钥不能为空或超长")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.keys.Set("channel:"+v.ID, key); err != nil {
		return Channel{}, err
	}
	next := append(append([]Channel{}, s.values...), v)
	if err := fsutil.WriteJSON(s.path, next); err != nil {
		if cleanErr := s.keys.Set("channel:"+v.ID, ""); cleanErr != nil {
			return Channel{}, errors.Join(err, cleanErr)
		}
		return Channel{}, err
	}
	s.values = next
	return v, nil
}
func (s *Channels) Update(id, name, address, protocol, key string) (Channel, error) {
	value := Channel{ID: id, Name: name, BaseURL: strings.TrimSuffix(address, "/"), Protocol: protocol}
	if err := validateChannel(value); err != nil {
		return Channel{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := append([]Channel{}, s.values...)
	found := false
	for index, current := range next {
		if current.ID == id {
			next[index] = value
			found = true
		}
	}
	if !found {
		return Channel{}, errors.New("渠道不存在")
	}
	if key != "" {
		if err := s.keys.Set("channel:"+id, key); err != nil {
			return Channel{}, err
		}
	}
	if err := fsutil.WriteJSON(s.path, next); err != nil {
		return Channel{}, err
	}
	s.values = next
	return value, nil
}
func (s *Channels) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := []Channel{}
	found := false
	for _, current := range s.values {
		if current.ID == id {
			found = true
			continue
		}
		next = append(next, current)
	}
	if !found {
		return errors.New("渠道不存在")
	}
	if err := fsutil.WriteJSON(s.path, next); err != nil {
		return err
	}
	if err := s.keys.Set("channel:"+id, ""); err != nil {
		return err
	}
	s.values = next
	return nil
}
