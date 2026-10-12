package settings

import (
	"errors"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/fsutil"
	"os"
	"sync"
)

type Store struct {
	mu            sync.RWMutex
	layout        datadir.Layout
	site          Site
	home          Home
	banner        Banner
	languages     Languages
	ai            AI
	onChange      func(string)
	knownLanguage func(string) bool
	aiReady       func() bool
}

func (s *Store) SetAIReadiness(check func() bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiReady = check
}

func Open(layout datadir.Layout, onChange func(string), knownLanguage func(string) bool) (*Store, error) {
	s := &Store{layout: layout, site: DefaultSite(), home: DefaultHome(), languages: DefaultLanguages(), ai: DefaultAI(), onChange: onChange, knownLanguage: knownLanguage}
	for _, entry := range []struct {
		path  string
		value any
	}{{layout.Site(), &s.site}, {layout.Home(), &s.home}, {layout.Banner(), &s.banner}, {layout.Languages(), &s.languages}, {layout.AI(), &s.ai}} {
		if err := fsutil.ReadJSON(entry.path, entry.value); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	s.home = s.home.normalized()
	if err := s.site.Validate(); err != nil {
		return nil, err
	}
	if err := s.home.Validate(); err != nil {
		return nil, err
	}
	if err := s.banner.Validate(); err != nil {
		return nil, err
	}
	if err := s.languages.Validate(); err != nil {
		return nil, err
	}
	if err := s.ai.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Store) changed(kind string) {
	if s.onChange != nil {
		s.onChange(kind)
	}
}
func (s *Store) SetOnChange(callback func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = callback
}

// WithAssetSettings 与正文快照共同保护资源清理，不向外暴露可修改的状态。
func (s *Store) WithAssetSettings(visit func(Site, Home) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	home := s.home
	home.Links = append([]HomeLink{}, home.Links...)
	return visit(s.site, home)
}

// ValidationError 的消息只包含可向调用者展示的静态规则，不包含凭据或路径。
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
func invalid(message string) error       { return &ValidationError{Message: message} }
