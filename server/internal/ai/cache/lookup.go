package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
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
	Vector                     []float32
	EmbeddingModel             string
}
type Candidate struct {
	Entry      Entry
	Similarity float64
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
	if !ok || !v.Curated && (time.Now().After(v.Expires) || v.MemoryVersion != memory) {
		return Entry{}, false
	}
	for id, hash := range v.References {
		if blocks != nil && blocks[id] != hash {
			return Entry{}, false
		}
	}
	return v, true
}
func (s *Store) GetAny(question, memory string) (Entry, bool) {
	wanted := Normalize(question)
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result Entry
	for _, value := range s.values {
		if Normalize(value.Question) != wanted || !value.Curated && (time.Now().After(value.Expires) || value.MemoryVersion != memory) {
			continue
		}
		if result.Question == "" || value.Curated && !result.Curated {
			result = value
		}
	}
	return result, result.Question != ""
}
func (s *Store) FindSemantic(question, language, memory string, threshold float64) (Entry, bool) {
	return s.findSemantic(question, language, memory, threshold, false)
}
func (s *Store) FindSemanticAny(question, memory string, threshold float64) (Entry, bool) {
	return s.findSemantic(question, "", memory, threshold, true)
}
func (s *Store) findSemantic(question, language, memory string, threshold float64, anyLanguage bool) (Entry, bool) {
	wanted := map[string]bool{}
	for _, token := range strings.Fields(Normalize(question)) {
		wanted[token] = true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	best := Entry{}
	bestScore := 0.0
	for _, value := range s.values {
		if !anyLanguage && value.Language != language || !value.Curated && (value.MemoryVersion != memory || time.Now().After(value.Expires)) {
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
		if score > bestScore && (best.Curated == value.Curated || !best.Curated) || value.Curated && !best.Curated && score >= threshold {
			bestScore = score
			best = value
		}
	}
	return best, bestScore >= threshold
}
func (s *Store) Put(v Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v.Vector = append([]float32{}, v.Vector...)
	s.values[Key(v.Question, v.Language)] = v
}
func (s *Store) InvalidateAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := map[string]Entry{}
	for key, value := range s.values {
		if value.Curated {
			next[key] = value
		}
	}
	s.values = next
}
func (s *Store) VectorCandidates(vector []float32, model, memory string, minimum float64, limit int) []Candidate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []Candidate{}
	for _, value := range s.values {
		if value.EmbeddingModel != model || len(value.Vector) != len(vector) || !value.Curated && (value.MemoryVersion != memory || time.Now().After(value.Expires)) {
			continue
		}
		dot, left, right := float64(0), float64(0), float64(0)
		for index := range vector {
			a, b := float64(vector[index]), float64(value.Vector[index])
			dot += a * b
			left += a * a
			right += b * b
		}
		if left == 0 || right == 0 {
			continue
		}
		similarity := dot / math.Sqrt(left*right)
		if similarity >= minimum {
			copy := value
			copy.Vector = append([]float32{}, value.Vector...)
			result = append(result, Candidate{Entry: copy, Similarity: similarity})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Entry.Curated != result[j].Entry.Curated {
			return result[i].Entry.Curated
		}
		return result[i].Similarity > result[j].Similarity
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}
