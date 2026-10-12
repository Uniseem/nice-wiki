package cache

func (s *Store) Curate(question, answer, language string) {
	s.Put(Entry{Question: question, Answer: answer, Language: language, Curated: true, References: map[string]string{}})
}
func (s *Store) Delete(question, language string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, Key(question, language))
}
