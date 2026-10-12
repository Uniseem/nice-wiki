package site

import (
	"encoding/json"
	"html/template"
	"net/url"
	"nice-wiki/internal/settings"
	"strings"
)

type alternate struct {
	Language string
	URL      string
}

func canonical(site settings.Site, path string) string {
	if site.BaseURL == "" {
		return ""
	}
	return strings.TrimSuffix(site.BaseURL, "/") + path
}
func alternates(site settings.Site, path string, languages settings.Languages) []alternate {
	if site.BaseURL == "" {
		return nil
	}
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
	if len(parts) != 2 {
		return nil
	}
	result := []alternate{}
	for _, lang := range languages.Enabled {
		result = append(result, alternate{Language: lang, URL: canonical(site, "/"+lang+"/"+parts[1])})
	}
	result = append(result, alternate{Language: "x-default", URL: canonical(site, "/"+languages.Base+"/"+parts[1])})
	return result
}

// BreadcrumbJSON 只接受已解析的站内地址，json.Marshal 会转义 HTML 敏感字符。
func breadcrumbJSON(site settings.Site, path, title string) template.JS {
	root := canonical(site, "/")
	target := canonical(site, path)
	if root == "" || target == "" {
		return ""
	}
	if _, err := url.ParseRequestURI(target); err != nil {
		return ""
	}
	value := map[string]any{
		"@context": "https://schema.org",
		"@type":    "BreadcrumbList",
		"itemListElement": []map[string]any{
			{"@type": "ListItem", "position": 1, "name": site.Name, "item": root},
			{"@type": "ListItem", "position": 2, "name": title, "item": target},
		},
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return template.JS(data)
}
