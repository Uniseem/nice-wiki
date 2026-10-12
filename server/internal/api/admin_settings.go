package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/settings"
)

func siteResponse(v settings.Site) gen.SiteSettings {
	return gen.SiteSettings{Name: v.Name, Description: v.Description, LogoHash: v.LogoHash, BaseURL: v.BaseURL, ThemeColor: v.ThemeColor, Footer: v.Footer}
}
func (s *Server) GetSiteSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, siteResponse(s.Settings.Site()))
}
func (s *Server) SaveSiteSettings(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.SiteSettings](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value := settings.Site{Name: body.Name, Description: body.Description, LogoHash: body.LogoHash, BaseURL: body.BaseURL, ThemeColor: body.ThemeColor, Footer: body.Footer}
	if err := s.Settings.SaveSite(value); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "save_site", "") {
		return
	}
	writeJSON(w, siteResponse(s.Settings.Site()))
}
func (s *Server) GetHomeSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Settings.Home())
}
func (s *Server) SaveHomeSettings(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.HomeSettings](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	links := make([]settings.HomeLink, len(body.Links))
	for i, v := range body.Links {
		links[i] = settings.HomeLink{Label: v.Label, DocumentID: v.DocumentId, URL: v.Url}
	}
	value := settings.Home{Title: body.Title, Subtitle: body.Subtitle, BackgroundHash: body.BackgroundHash, BackgroundKind: string(body.BackgroundKind), VideoPosterHash: body.VideoPosterHash, TranslateBackgroundText: body.TranslateBackgroundText, Links: links}
	if err := s.Settings.SaveHome(value); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "save_home", "") {
		return
	}
	writeJSON(w, s.Settings.Home())
}
func (s *Server) GetBannerSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Settings.Banner())
}
func (s *Server) SaveBannerSettings(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.BannerSettings](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value := settings.Banner{Enabled: body.Enabled, Text: body.Text, URL: body.Url, Dismissible: body.Dismissible}
	if err := s.Settings.SaveBanner(value); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "save_banner", "") {
		return
	}
	writeJSON(w, s.Settings.Banner())
}
