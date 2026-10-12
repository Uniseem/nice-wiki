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
	StartsAt    int64  `json:"startsAt"`
	EndsAt      int64  `json:"endsAt"`
}

func (v Banner) Validate() error {
	if len([]rune(v.Text)) > 2000 {
		return invalid("公告超出长度限制")
	}
	if v.Enabled && v.Text == "" {
		return invalid("启用公告需要文字")
	}
	if v.StartsAt < 0 || v.EndsAt < 0 || v.EndsAt != 0 && v.EndsAt <= v.StartsAt {
		return invalid("公告生效时间无效")
	}
	if v.URL != "" {
		if _, ok := doc.SafeURL(v.URL); !ok {
			return invalid("公告链接无效")
		}
	}
	return nil
}

func (v Banner) Active(now int64) bool {
	return v.Enabled && (v.StartsAt == 0 || now >= v.StartsAt) && (v.EndsAt == 0 || now < v.EndsAt)
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
