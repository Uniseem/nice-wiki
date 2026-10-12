package settings

import (
	"net/url"
	"nice-wiki/internal/fsutil"
	"regexp"
	"strings"
)

type Site struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LogoHash    string `json:"logoHash"`
	BaseURL     string `json:"baseURL"`
	ThemeColor  string `json:"themeColor"`
	Footer      string `json:"footer"`
	Initialized bool   `json:"initialized"`
}

func DefaultSite() Site { return Site{Name: "nice-wiki", ThemeColor: "#2563eb"} }
func (v Site) Validate() error {
	if strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 120 || len([]rune(v.Description)) > 500 || len([]rune(v.Footer)) > 4000 {
		return invalid("站点文字长度无效")
	}
	if !regexp.MustCompile(`^#[A-Fa-f0-9]{6}$`).MatchString(v.ThemeColor) {
		return invalid("主题色必须是六位十六进制颜色")
	}
	if v.LogoHash != "" && !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(v.LogoHash) {
		return invalid("Logo 哈希无效")
	}
	if v.BaseURL != "" {
		u, err := url.Parse(v.BaseURL)
		if err != nil || u.User != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return invalid("站点地址需要完整的 HTTP 或 HTTPS 根地址")
		}
	}
	return nil
}
func (s *Store) Site() Site { s.mu.RLock(); defer s.mu.RUnlock(); return s.site }
func (s *Store) SaveSite(v Site) error {
	if err := v.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	v.Initialized = s.site.Initialized
	if err := fsutil.WriteJSON(s.layout.Site(), v); err != nil {
		s.mu.Unlock()
		return err
	}
	s.site = v
	s.mu.Unlock()
	s.changed("site")
	return nil
}
func (s *Store) CompleteSetup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.site
	v.Initialized = true
	if err := fsutil.WriteJSON(s.layout.Site(), v); err != nil {
		return err
	}
	s.site = v
	return nil
}
