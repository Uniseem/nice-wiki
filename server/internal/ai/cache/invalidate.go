package cache

func (s *Store) InvalidateBlocks(changed map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range s.values {
		remove := len(value.References) == 0
		for id, hash := range value.References {
			if current, ok := changed[id]; ok && current != hash {
				remove = true
			}
		}
		if remove && !value.Curated {
			delete(s.values, key)
		}
	}
}
