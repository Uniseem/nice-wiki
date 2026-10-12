package site

import "net/http"

func (s *Site) home(w http.ResponseWriter, r *http.Request, lang string) {
	data := s.page(lang, s.Settings.Site().Name)
	data.Description = data.Site.Description
	data.HomePage = true
	switch data.Home.BackgroundKind {
	case "image":
		data.HeroBackground = "/assets/images/" + data.Home.BackgroundHash + ".webp"
	case "video":
		data.HeroBackground = "/assets/videos/" + data.Home.BackgroundHash + ".mp4"
		if data.Home.VideoPosterHash != "" {
			data.HeroPoster = "/assets/images/" + data.Home.VideoPosterHash + ".webp"
		}
	}
	for _, link := range data.Home.Links {
		target := link.URL
		if link.DocumentID != "" {
			value, err := s.Content.Get(link.DocumentID)
			if err != nil || !value.Meta.Published {
				continue
			}
			path, err := s.Content.Path(link.DocumentID)
			if err != nil {
				continue
			}
			target = "/" + lang + "/" + path
		}
		data.HeroLinks = append(data.HeroLinks, heroLink{Label: link.Label, URL: target, Style: link.Style})
	}
	s.render(w, r, 200, "home", data)
}
