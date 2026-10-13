package memory

import (
	"bytes"
	"encoding/json"
	"errors"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/fsutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

type Store struct {
	mu     sync.RWMutex
	layout datadir.Layout
}

func Open(layout datadir.Layout) (*Store, error) {
	s := &Store{layout: layout}
	for _, v := range Builtins {
		if _, err := os.Stat(mustPath(layout, v.ID)); errors.Is(err, os.ErrNotExist) {
			if err := s.Save(v); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}
func mustPath(layout datadir.Layout, id string) string {
	path, err := layout.MemoryEntry(id)
	if err != nil {
		panic(err)
	}
	return path
}
func encode(v Item) ([]byte, error) {
	meta := v
	meta.Content = ""
	head, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	return bytes.Join([][]byte{[]byte("---\n"), head, []byte("\n---\n"), []byte(v.Content), []byte("\n")}, nil), nil
}
func decode(data []byte) (Item, error) {
	parts := bytes.SplitN(data, []byte("\n---\n"), 2)
	if len(parts) != 2 || !bytes.HasPrefix(parts[0], []byte("---\n")) {
		return Item{}, errors.New("记忆文件格式无效")
	}
	var v Item
	if err := json.Unmarshal(bytes.TrimPrefix(parts[0], []byte("---\n")), &v); err != nil {
		return v, err
	}
	v.Content = strings.TrimSuffix(string(parts[1]), "\n")
	return v, Validate(v)
}
func (s *Store) Save(v Item) error {
	if err := Validate(v); err != nil {
		return err
	}
	if current, err := s.Get(v.ID); err == nil {
		if current.ReadOnly && !reflect.DeepEqual(current, v) {
			return errors.New("只读记忆不能修改")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	path, err := s.layout.MemoryEntry(v.ID)
	if err != nil {
		return err
	}
	data, err := encode(v)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fsutil.WriteFile(path, data, 0600)
}
func (s *Store) Delete(id string) error {
	current, err := s.Get(id)
	if err != nil {
		return err
	}
	if current.ReadOnly {
		return errors.New("只读记忆不能删除")
	}
	path, err := s.layout.MemoryEntry(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Remove(path)
}
func (s *Store) Get(id string) (Item, error) {
	path, err := s.layout.MemoryEntry(id)
	if err != nil {
		return Item{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return Item{}, err
	}
	return decode(data)
}
func (s *Store) List() ([]Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.layout.Memory())
	if err != nil {
		return nil, err
	}
	result := []Item{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.layout.Memory(), e.Name()))
		if err != nil {
			return nil, err
		}
		v, err := decode(data)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}
