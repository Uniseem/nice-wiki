package site

func (s *Site) navigation(lang string) []navigation {
	values, _, _ := s.documentNavigation(lang, "")
	return values
}

func (s *Site) documentNavigation(lang, current string) ([]navigation, *navigation, *navigation) {
	values := s.Content.List()
	result := []navigation{}
	var visit func(string, int)
	visit = func(parent string, depth int) {
		for _, value := range values {
			if value.ParentID != parent {
				continue
			}
			if value.Published {
				if path, err := s.Content.Path(value.ID); err == nil {
					title, _ := s.translatedMetadata(lang, value.ID, value.Title, "")
					result = append(result, navigation{Title: title, URL: "/" + lang + "/" + path, Depth: depth, ID: value.ID, Active: value.ID == current})
				}
			}
			visit(value.ID, depth+1)
		}
	}
	visit("", 0)
	var previous, next *navigation
	for index := range result {
		if result[index].ID != current {
			continue
		}
		if index > 0 {
			value := result[index-1]
			previous = &value
		}
		if index+1 < len(result) {
			value := result[index+1]
			next = &value
		}
		break
	}
	return result, previous, next
}
