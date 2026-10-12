package qa

import (
	"sync"
	"time"
)

type DebugStep struct {
	Name          string
	Input, Output string
	Cost          float64
	Duration      time.Duration
}
type DebugTrace struct {
	ID      string
	Steps   []DebugStep
	Created time.Time
}
type DebugStore struct {
	mu     sync.Mutex
	values map[string]DebugTrace
}

func NewDebugStore() *DebugStore { return &DebugStore{values: map[string]DebugTrace{}} }
func (d *DebugStore) Add(id string, step DebugStep) {
	d.mu.Lock()
	defer d.mu.Unlock()
	v := d.values[id]
	v.ID = id
	v.Created = time.Now()
	v.Steps = append(v.Steps, step)
	d.values[id] = v
}
func (d *DebugStore) Get(id string) (DebugTrace, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	v, ok := d.values[id]
	v.Steps = append([]DebugStep{}, v.Steps...)
	return v, ok
}
