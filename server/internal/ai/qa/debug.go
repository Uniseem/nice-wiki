package qa

import (
	"sync"
	"time"
)

type DebugStep struct {
	Name     string        `json:"name"`
	Input    string        `json:"input"`
	Output   string        `json:"output"`
	Cost     float64       `json:"cost"`
	Duration time.Duration `json:"duration"`
}
type DebugTrace struct {
	ID      string      `json:"id"`
	Steps   []DebugStep `json:"steps"`
	Created time.Time   `json:"created"`
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
	if len(d.values) > 200 {
		oldestID := ""
		var oldest time.Time
		for candidate, trace := range d.values {
			if oldestID == "" || trace.Created.Before(oldest) {
				oldestID, oldest = candidate, trace.Created
			}
		}
		delete(d.values, oldestID)
	}
}
func (d *DebugStore) Get(id string) (DebugTrace, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	v, ok := d.values[id]
	if ok && time.Since(v.Created) > time.Hour {
		delete(d.values, id)
		return DebugTrace{}, false
	}
	v.Steps = append([]DebugStep{}, v.Steps...)
	return v, ok
}
