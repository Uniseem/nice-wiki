package settings

import (
	"nice-wiki/internal/doc"
	"nice-wiki/internal/fsutil"
)

type Banner struct {
	Enabled     bool   `json:"enabled"`
	Text        string `json:"text"`
	URL         string `json:"url"`
	Dismissible bool   `json:"dismissible"`
}

func (v Banner) Validate() error {
	if len([]rune(v.Text)) > 2000 {
		return invalid("公告超出长度限制")
	}
	if v.Enabled && v.Text == "" {
		return invalid("启用公告需要文字")
	}
	if v.URL != "" {
		if _, ok := doc.SafeURL(v.URL); !ok {
			return invalid("公告链接无效")
		}
	}
	return nil
}
func (s *Store) Banner() Banner { s.mu.RLock(); defer s.mu.RUnlock(); return s.banner }
func (s *Store) SaveBanner(v Banner) error {
	if err := v.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	if err := fsutil.WriteJSON(s.layout.Banner(), v); err != nil {
		s.mu.Unlock()
		return err
	}
	s.banner = v
	s.mu.Unlock()
	s.changed("banner")
	return nil
}
