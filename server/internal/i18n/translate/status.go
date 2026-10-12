package translate

type Status struct {
	Language                       string
	Total, Translated, NeedsReview int
	Percent                        int
}

func Progress(language string, total int, value File) Status {
	status := Status{Language: language, Total: total}
	for _, block := range value.Blocks {
		if block.Text != "" {
			status.Translated++
		}
		if block.NeedsReview {
			status.NeedsReview++
		}
	}
	if total > 0 {
		status.Percent = status.Translated * 100 / total
	}
	return status
}
