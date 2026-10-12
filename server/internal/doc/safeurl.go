package doc

import (
	"net/url"
	"strings"
	"unicode"
)

func SafeURL(raw string) (string, bool) {
	if strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\\\r\n\t") {
		return "", false
	}
	for _, r := range raw {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil {
		return "", false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		if parsed.Host == "" {
			return "", false
		}
		return parsed.String(), true
	case "mailto":
		if parsed.Opaque == "" || strings.Contains(parsed.Opaque, "?") {
			return "", false
		}
		return parsed.String(), true
	case "":
		if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
			return raw, true
		}
		if strings.HasPrefix(raw, "#") {
			return raw, true
		}
	}
	return "", false
}
