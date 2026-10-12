package memory

import (
	"errors"
	"nice-wiki/internal/aitask"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/fsutil"
	"os"
	"sync"
)

type Mapping struct {
	mu     sync.RWMutex
	path   string
	values map[string][]string
}

func OpenMapping(layout datadir.Layout) (*Mapping, error) {
	values := map[string][]string{}
	if err := fsutil.ReadJSON(layout.MemoryTasks(), &values); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for name, task := range aitask.Tasks {
		if _, ok := values[name]; !ok {
			values[name] = append([]string{}, task.Memory...)
		}
	}
	return &Mapping{path: layout.MemoryTasks(), values: values}, nil
}
func (m *Mapping) Get(task string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.values[task]...)
}
func (m *Mapping) All() map[string][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := map[string][]string{}
	for key, value := range m.values {
		result[key] = append([]string{}, value...)
	}
	return result
}
func (m *Mapping) Replace(values map[string][]string) error {
	for task := range values {
		if _, ok := aitask.Lookup(task); !ok {
			return errors.New("未知 AI 任务")
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := map[string][]string{}
	for key, value := range values {
		next[key] = append([]string{}, value...)
	}
	if err := fsutil.WriteJSON(m.path, next); err != nil {
		return err
	}
	m.values = next
	return nil
}
func (m *Mapping) Set(task string, ids []string) error {
	if _, ok := aitask.Lookup(task); !ok {
		return errors.New("未知 AI 任务")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := map[string][]string{}
	for k, v := range m.values {
		next[k] = append([]string{}, v...)
	}
	next[task] = append([]string{}, ids...)
	if err := fsutil.WriteJSON(m.path, next); err != nil {
		return err
	}
	m.values = next
	return nil
}
