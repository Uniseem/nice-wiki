package site

func (s *Site) navigation(lang string) []navigation {
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
					result = append(result, navigation{Title: value.Title, URL: "/" + lang + "/" + path, Depth: depth})
				}
			}
			visit(value.ID, depth+1)
		}
	}
	visit("", 0)
	return result
}
