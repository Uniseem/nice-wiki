package settings

import (
	"nice-wiki/internal/doc"
	"nice-wiki/internal/fsutil"
)

type HomeLink struct {
	Label      string `json:"label"`
	DocumentID string `json:"documentId"`
	URL        string `json:"url"`
}
type Home struct {
	Title                   string     `json:"title"`
	Subtitle                string     `json:"subtitle"`
	BackgroundHash          string     `json:"backgroundHash"`
	BackgroundKind          string     `json:"backgroundKind"`
	VideoPosterHash         string     `json:"videoPosterHash"`
	TranslateBackgroundText bool       `json:"translateBackgroundText"`
	Links                   []HomeLink `json:"links"`
}

func DefaultHome() Home {
	return Home{Title: "欢迎来到文档站", BackgroundKind: "none", Links: []HomeLink{}}
}
func (v Home) Validate() error {
	if len([]rune(v.Title)) > 200 || len([]rune(v.Subtitle)) > 1000 || len(v.Links) > 8 {
		return invalid("首页内容超出限制")
	}
	switch v.BackgroundKind {
	case "none", "image", "video":
	default:
		return invalid("首页背景类型无效")
	}
	for _, link := range v.Links {
		if link.Label == "" || len([]rune(link.Label)) > 80 {
			return invalid("首页链接文字无效")
		}
		if link.URL != "" {
			if _, ok := doc.SafeURL(link.URL); !ok {
				return invalid("首页链接地址无效")
			}
		}
		if link.DocumentID == "" && link.URL == "" {
			return invalid("首页链接缺少目标")
		}
	}
	return nil
}
func (s *Store) Home() Home {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := s.home
	v.Links = append([]HomeLink{}, v.Links...)
	return v
}
func (s *Store) SaveHome(v Home) error {
	if err := v.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	v.Links = append([]HomeLink{}, v.Links...)
	if err := fsutil.WriteJSON(s.layout.Home(), v); err != nil {
		s.mu.Unlock()
		return err
	}
	s.home = v
	s.mu.Unlock()
	s.changed("home")
	return nil
}
