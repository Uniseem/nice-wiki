package settings

import (
	"golang.org/x/text/language"
	"nice-wiki/internal/fsutil"
)

type Languages struct {
	Base    string   `json:"base"`
	Enabled []string `json:"enabled"`
}

func DefaultLanguages() Languages { return Languages{Base: "zh-Hans", Enabled: []string{"zh-Hans"}} }
func (v Languages) Validate() error {
	if _, err := language.Parse(v.Base); err != nil {
		return invalid("基准语言不在内置清单")
	}
	seen := map[string]bool{}
	for _, code := range v.Enabled {
		if _, err := language.Parse(code); err != nil || seen[code] {
			return invalid("启用语言无效或重复")
		}
		seen[code] = true
	}
	if !seen[v.Base] {
		return invalid("基准语言必须启用")
	}
	return nil
}
func (s *Store) Languages() Languages {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := s.languages
	v.Enabled = append([]string{}, v.Enabled...)
	return v
}
func (s *Store) SetBaseLanguage(v Languages) error {
	if err := v.Validate(); err != nil {
		return err
	}
	if s.knownLanguage == nil || !s.knownLanguage(v.Base) {
		return invalid("基准语言不在内置清单")
	}
	if len(v.Enabled) != 1 || v.Enabled[0] != v.Base {
		return invalid("当前阶段只开放基准语言")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.site.Initialized {
		return invalid("初始化后不能更改基准语言")
	}
	if err := fsutil.WriteJSON(s.layout.Languages(), v); err != nil {
		return err
	}
	v.Enabled = append([]string{}, v.Enabled...)
	s.languages = v
	return nil
}
