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
	if strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 120 || v.Protocol != "openai" {
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
