package qa

import (
	"sync"
	"time"
)

type Record struct {
	ID, ConversationID, Question, NormalizedQuestion, Answer, Language string
	Citations                                                          []Citation
	Cost                                                               float64
	Duration                                                           time.Duration
	Created                                                            time.Time
	Helpful                                                            *bool
}
type Records struct {
	mu        sync.Mutex
	values    []Record
	Retention time.Duration
}

func (r *Records) Add(value Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values = append(r.values, value)
}
func (r *Records) List() []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-r.Retention)
	result := []Record{}
	for _, v := range r.values {
		if r.Retention <= 0 || v.Created.After(cutoff) {
			result = append(result, v)
		}
	}
	return result
}
func (r *Records) Feedback(id, conversation string, helpful bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.values {
		if r.values[i].ID == id && r.values[i].ConversationID == conversation {
			r.values[i].Helpful = &helpful
			return true
		}
	}
	return false
}
