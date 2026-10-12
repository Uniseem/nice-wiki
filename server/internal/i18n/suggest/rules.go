package suggest

import "strings"

func Rule(signals Signals, enabled []string, base string) string {
	for _, browser := range signals.Languages {
		browser = strings.ToLower(browser)
		for _, code := range enabled {
			lower := strings.ToLower(code)
			if browser == lower || strings.HasPrefix(browser, lower+"-") || strings.HasPrefix(lower, browser+"-") {
				return code
			}
		}
	}
	return base
}
