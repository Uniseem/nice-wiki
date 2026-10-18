package publicsite

import (
	"net/url"
	"strings"

)

// safeLink 只接受明确协议和单斜杠同源路径；不输出未知资源地址。
func safeLink(raw string) bool {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\\\x00\r\n\t") {
		return false
	}
	if strings.HasPrefix(raw, "/") {
		return !strings.HasPrefix(raw, "//")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil {
		return false
	}
	switch parsed.Scheme {
	case "https", "http":
		return parsed.Host != ""
	case "mailto":
		return parsed.Opaque != ""
	}
	return false
}
