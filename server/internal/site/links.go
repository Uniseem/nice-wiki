package site

func (s *Site) ResolveDocument(id, block, lang string) (string, bool) {
	value, err := s.Content.Get(id)
	if err != nil || !value.Meta.Published {
		return "", false
	}
	path, err := s.Content.Path(id)
	if err != nil {
		return "", false
	}
	url := "/" + lang + "/" + path
	if block != "" {
		url += "#b-" + block
	}
	return url, true
}
