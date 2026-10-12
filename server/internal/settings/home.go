package settings

import (
	"nice-wiki/internal/doc"
	"nice-wiki/internal/fsutil"
	"regexp"
)

type HomeLink struct {
	Label      string `json:"label"`
	DocumentID string `json:"documentId"`
	URL        string `json:"url"`
	Style      string `json:"style"`
}
type Home struct {
	Title                   string     `json:"title"`
	Subtitle                string     `json:"subtitle"`
	BackgroundText          string     `json:"backgroundText"`
	BackgroundHash          string     `json:"backgroundHash"`
	BackgroundKind          string     `json:"backgroundKind"`
	VideoPosterHash         string     `json:"videoPosterHash"`
	Overlay                 int        `json:"overlay"`
	TextColor               string     `json:"textColor"`
	TranslateBackgroundText bool       `json:"translateBackgroundText"`
	Links                   []HomeLink `json:"links"`
}

func DefaultHome() Home {
	return Home{Title: "欢迎来到文档站", BackgroundText: "Welcome To", BackgroundKind: "none", Overlay: 35, TextColor: "light", Links: []HomeLink{}}
}
func (v Home) normalized() Home {
	if v.BackgroundText == "" {
		v.BackgroundText = "Welcome To"
	}
	if v.TextColor == "" {
		v.TextColor = "light"
	}
	for index := range v.Links {
		if v.Links[index].Style == "" {
			v.Links[index].Style = "secondary"
		}
	}
	return v
}
func (v Home) Validate() error {
	if len([]rune(v.Title)) > 200 || len([]rune(v.Subtitle)) > 1000 || len([]rune(v.BackgroundText)) > 200 || len(v.Links) > 3 {
		return invalid("首页内容超出限制")
	}
	switch v.BackgroundKind {
	case "none", "image", "video":
	default:
		return invalid("首页背景类型无效")
	}
	if v.Overlay < 0 || v.Overlay > 100 || v.TextColor != "light" && v.TextColor != "dark" {
		return invalid("首页遮罩或文字颜色无效")
	}
	digest := regexp.MustCompile(`^[a-f0-9]{64}$`)
	if v.BackgroundKind != "none" && !digest.MatchString(v.BackgroundHash) {
		return invalid("首页背景资源无效")
	}
	if v.VideoPosterHash != "" && !digest.MatchString(v.VideoPosterHash) {
		return invalid("首页视频封面无效")
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
		if link.DocumentID != "" && link.URL != "" {
			return invalid("首页链接只能选择一种目标")
		}
		if link.Style != "primary" && link.Style != "secondary" {
			return invalid("首页链接样式无效")
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
