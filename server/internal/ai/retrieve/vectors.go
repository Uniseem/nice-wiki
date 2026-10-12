package retrieve

import (
	"math"
	"sort"
	"sync"
)

type Hit struct {
	ID    string
	Score float64
}
type VectorIndex struct {
	mu     sync.RWMutex
	values map[string][]float32
}

func NewVectorIndex() *VectorIndex { return &VectorIndex{values: map[string][]float32{}} }
func (v *VectorIndex) Set(id string, value []float32) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.values[id] = append([]float32{}, value...)
}
func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	dot, aa, bb := float64(0), float64(0), float64(0)
	for i := range a {
		dot += float64(a[i] * b[i])
		aa += float64(a[i] * a[i])
		bb += float64(b[i] * b[i])
	}
	if aa == 0 || bb == 0 {
		return 0
	}
	return dot / math.Sqrt(aa*bb)
}
func (v *VectorIndex) Search(query []float32, limit int) []Hit {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := make([]Hit, 0, len(v.values))
	for id, value := range v.values {
		result = append(result, Hit{id, cosine(query, value)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	if limit < len(result) {
		result = result[:limit]
	}
	return result
}
