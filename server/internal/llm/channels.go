package llm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"nice-wiki/internal/fsutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type KeyStore interface {
	Set(string, string) error
	Has(string) bool
	Use(string, func(string) error) error
}
type Channel struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Provider             string   `json:"provider,omitempty"`
	BaseURL              string   `json:"baseURL"`
	Protocol             string   `json:"protocol"`
	Disabled             bool     `json:"disabled,omitempty"`
	TimeoutSeconds       int      `json:"timeoutSeconds,omitempty"`
	StreamTimeoutSeconds int      `json:"streamTimeoutSeconds,omitempty"`
	Concurrency          int      `json:"concurrency,omitempty"`
	HeaderNames          []string `json:"headerNames,omitempty"`
}
type Channels struct {
	mu      sync.RWMutex
	path    string
	keys    KeyStore
	values  []Channel
	limitMu sync.Mutex
	limits  map[string]chan struct{}
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
	return &Channels{path: path, keys: keys, values: values, limits: map[string]chan struct{}{}}, nil
}
func validateChannel(v Channel) error {
	if strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 120 || len(v.Provider) > 128 || v.TimeoutSeconds < 0 || v.TimeoutSeconds > 600 || v.StreamTimeoutSeconds < 0 || v.StreamTimeoutSeconds > 600 || v.Concurrency < 0 || v.Concurrency > 128 || v.Protocol != "openai" && v.Protocol != "anthropic" && v.Protocol != "gemini" && v.Protocol != "rerank" {
		return errors.New("渠道名称或协议无效")
	}
	u, err := url.Parse(v.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("渠道地址必须是无凭据的 HTTP 或 HTTPS 地址")
	}
	for _, name := range v.HeaderNames {
		if !headerName.MatchString(name) {
			return errors.New("渠道附加请求头名称无效")
		}
	}
	return nil
}

var headerName = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+.^_` + "`" + `|~-]+$`)

func normalizeHeaders(values map[string]string) (map[string]string, []string, error) {
	result := map[string]string{}
	names := []string{}
	for name, value := range values {
		name = strings.TrimSpace(name)
		if !headerName.MatchString(name) || strings.ContainsAny(value, "\r\n") || len(name) > 128 || len(value) > 8192 {
			return nil, nil, errors.New("附加请求头无效")
		}
		result[name] = value
		names = append(names, name)
	}
	sort.Strings(names)
	return result, names, nil
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
func (s *Channels) UseHeaders(id string, fn func(map[string]string) error) error {
	key := "channel-headers:" + id
	if !s.keys.Has(key) {
		return fn(map[string]string{})
	}
	return s.keys.Use(key, func(encoded string) error {
		values := map[string]string{}
		if err := json.Unmarshal([]byte(encoded), &values); err != nil {
			return errors.New("附加请求头配置无效")
		}
		return fn(values)
	})
}
func (s *Channels) Acquire(ctx context.Context, id string) (func(), error) {
	limit := 0
	for _, channel := range s.List() {
		if channel.ID == id {
			limit = channel.Concurrency
			break
		}
	}
	if limit <= 0 {
		return func() {}, nil
	}
	key := id + ":" + strconv.Itoa(limit)
	s.limitMu.Lock()
	semaphore := s.limits[key]
	if semaphore == nil {
		semaphore = make(chan struct{}, limit)
		s.limits[key] = semaphore
	}
	s.limitMu.Unlock()
	select {
	case semaphore <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-semaphore }) }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (s *Channels) Create(name, provider, address, protocol, key string, enabled bool, timeout, streamTimeout, concurrency int, headers map[string]string) (Channel, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return Channel{}, err
	}
	normalizedHeaders, names, err := normalizeHeaders(headers)
	if err != nil {
		return Channel{}, err
	}
	v := Channel{ID: hex.EncodeToString(bytes), Name: name, Provider: strings.TrimSpace(provider), BaseURL: strings.TrimSuffix(address, "/"), Protocol: protocol, Disabled: !enabled, TimeoutSeconds: timeout, StreamTimeoutSeconds: streamTimeout, Concurrency: concurrency, HeaderNames: names}
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
	encodedHeaders, err := json.Marshal(normalizedHeaders)
	if err != nil {
		return Channel{}, err
	}
	if err := s.keys.Set("channel-headers:"+v.ID, string(encodedHeaders)); err != nil {
		_ = s.keys.Set("channel:"+v.ID, "")
		return Channel{}, err
	}
	next := append(append([]Channel{}, s.values...), v)
	if err := fsutil.WriteJSON(s.path, next); err != nil {
		if cleanErr := s.keys.Set("channel:"+v.ID, ""); cleanErr != nil {
			return Channel{}, errors.Join(err, cleanErr)
		}
		_ = s.keys.Set("channel-headers:"+v.ID, "")
		return Channel{}, err
	}
	s.values = next
	return v, nil
}
func (s *Channels) Update(id, name, provider, address, protocol, key string, enabled bool, timeout, streamTimeout, concurrency int, headers *map[string]string) (Channel, error) {
	value := Channel{ID: id, Name: name, Provider: strings.TrimSpace(provider), BaseURL: strings.TrimSuffix(address, "/"), Protocol: protocol, Disabled: !enabled, TimeoutSeconds: timeout, StreamTimeoutSeconds: streamTimeout, Concurrency: concurrency}
	if err := validateChannel(value); err != nil {
		return Channel{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := append([]Channel{}, s.values...)
	found := false
	for index, current := range next {
		if current.ID == id {
			value.HeaderNames = append([]string{}, current.HeaderNames...)
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
	if headers != nil {
		normalized, names, err := normalizeHeaders(*headers)
		if err != nil {
			return Channel{}, err
		}
		encoded, err := json.Marshal(normalized)
		if err != nil {
			return Channel{}, err
		}
		if err := s.keys.Set("channel-headers:"+id, string(encoded)); err != nil {
			return Channel{}, err
		}
		value.HeaderNames = names
		for index := range next {
			if next[index].ID == id {
				next[index] = value
			}
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
	if err := s.keys.Set("channel-headers:"+id, ""); err != nil {
		return err
	}
	s.values = next
	return nil
}
