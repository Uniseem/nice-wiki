package site

import (
	"encoding/xml"
	"net/http"
	"strings"
	"time"
)

type sitemapIndex struct {
	XMLName   xml.Name           `xml:"sitemapindex"`
	Namespace string             `xml:"xmlns,attr"`
	Sitemaps  []sitemapReference `xml:"sitemap"`
}
type sitemapReference struct {
	Location string `xml:"loc"`
}
type sitemapURLs struct {
	XMLName   xml.Name     `xml:"urlset"`
	Namespace string       `xml:"xmlns,attr"`
	URLs      []sitemapURL `xml:"url"`
}
type sitemapURL struct {
	Location     string `xml:"loc"`
	LastModified string `xml:"lastmod,omitempty"`
}

func (s *Site) sitemap(w http.ResponseWriter, r *http.Request, language string) {
	site := s.Settings.Site()
	if site.BaseURL == "" {
		http.Error(w, "请先配置站点地址", http.StatusServiceUnavailable)
		return
	}
	base := strings.TrimSuffix(site.BaseURL, "/")
	var value any
	if language == "" {
		index := sitemapIndex{Namespace: "http://www.sitemaps.org/schemas/sitemap/0.9", Sitemaps: []sitemapReference{}}
		for _, lang := range s.Settings.Languages().Enabled {
			index.Sitemaps = append(index.Sitemaps, sitemapReference{base + "/" + lang + "/sitemap.xml"})
		}
		value = index
	} else {
		urls := sitemapURLs{Namespace: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: []sitemapURL{{Location: base + "/" + language + "/"}}}
		for _, document := range s.Content.List() {
			if !document.Published {
				continue
			}
			path, err := s.Content.Path(document.ID)
			if err != nil {
				http.Error(w, "站点索引暂时不可用", http.StatusServiceUnavailable)
				return
			}
			urls.URLs = append(urls.URLs, sitemapURL{Location: base + "/" + language + "/" + path, LastModified: time.Unix(document.UpdatedAt, 0).UTC().Format(time.RFC3339)})
		}
		value = urls
	}
	data, err := xml.MarshalIndent(value, "", "  ")
	if err != nil {
		http.Error(w, "站点索引生成失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if r.Method != http.MethodHead {
		_, _ = w.Write(append([]byte(xml.Header), data...))
	}
}
