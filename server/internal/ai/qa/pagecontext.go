package qa

type PageContext struct {
	DocumentID, Title, Path string
	Priority                bool
}

func PrioritizePage(values []string, page string) []string {
	if page == "" {
		return append([]string{}, values...)
	}
	result := []string{}
	for _, value := range values {
		if value == page {
			result = append(result, value)
		}
	}
	for _, value := range values {
		if value != page {
			result = append(result, value)
		}
	}
	return result
}
