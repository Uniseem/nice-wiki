package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Question, Answer, Language string
	References                 map[string]string
	Sources                    map[string]string
	Titles                     map[string]string
	MemoryVersion              string
	Created                    time.Time
	Expires                    time.Time
	Curated                    bool
}
type Store struct {
	mu     sync.RWMutex
	values map[string]Entry
}

func New() *Store                   { return &Store{values: map[string]Entry{}} }
func Normalize(value string) string { return strings.Join(strings.Fields(strings.ToLower(value)), " ") }
func Key(question, language string) string {
	sum := sha256.Sum256([]byte(language + "\x00" + Normalize(question)))
	return hex.EncodeToString(sum[:])
}
func (s *Store) Get(question, language, memory string, blocks map[string]string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.values[Key(question, language)]
	if !ok || !v.Curated && time.Now().After(v.Expires) || v.MemoryVersion != memory {
		return Entry{}, false
	}
	for id, hash := range v.References {
		if blocks != nil && blocks[id] != hash {
			return Entry{}, false
		}
	}
	return v, true
}
func (s *Store) FindSemantic(question, language, memory string, threshold float64) (Entry, bool) {
	wanted := map[string]bool{}
	for _, token := range strings.Fields(Normalize(question)) {
		wanted[token] = true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	best := Entry{}
	bestScore := 0.0
	for _, value := range s.values {
		if value.Language != language || value.MemoryVersion != memory || !value.Curated && time.Now().After(value.Expires) {
			continue
		}
		candidate := map[string]bool{}
		for _, token := range strings.Fields(Normalize(value.Question)) {
			candidate[token] = true
		}
		intersection := 0
		for token := range wanted {
			if candidate[token] {
				intersection++
			}
		}
		union := len(wanted)
		for token := range candidate {
			if !wanted[token] {
				union++
			}
		}
		if union == 0 {
			continue
		}
		score := float64(intersection) / float64(union)
		if score > bestScore {
			bestScore = score
			best = value
		}
	}
	return best, bestScore >= threshold
}
func (s *Store) Put(v Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[Key(v.Question, v.Language)] = v
}
func (s *Store) InvalidateAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values = map[string]Entry{}
}
