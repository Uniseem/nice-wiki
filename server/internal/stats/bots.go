package stats

import "strings"

func Bot(userAgent string) (bool, bool) {
	value := strings.ToLower(userAgent)
	ai := []string{"gptbot", "chatgpt-user", "claudebot", "anthropic-ai", "perplexitybot", "google-extended"}
	for _, name := range ai {
		if strings.Contains(value, name) {
			return true, true
		}
	}
	for _, name := range []string{"bot", "crawler", "spider", "slurp"} {
		if strings.Contains(value, name) {
			return true, false
		}
	}
	return false, false
}
