package content

import (
	"context"
	"errors"
	"nice-wiki/internal/fracindex"
	"nice-wiki/internal/fsutil"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func (r *Repo) List() []Meta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := []Meta{}
	for _, value := range r.files {
		result = append(result, value.Meta)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.ParentID != b.ParentID {
			return a.ParentID < b.ParentID
		}
		if a.SortKey != b.SortKey {
			return a.SortKey < b.SortKey
		}
		return a.ID < b.ID
	})
	return result
}
func (r *Repo) Move(ctx context.Context, id, parent, before, after string) (Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, exists := r.files[id]
	if !exists {
		return Meta{}, ErrNotFound
	}
	if id == parent {
		return Meta{}, errors.New("文档不能移动到自身")
	}
	ancestors, err := r.ancestorsLocked(parent)
	if err != nil {
		return Meta{}, err
	}
	for _, ancestor := range ancestors {
		if ancestor == id {
			return Meta{}, errors.New("不能移动到子文档")
		}
	}
	for otherID, other := range r.files {
		if otherID != id && other.Meta.ParentID == parent && other.Meta.Slug == value.Meta.Slug {
			return Meta{}, ErrConflict
		}
	}
	lower, upper := "", ""
	for target, bound := range map[string]*string{after: &lower, before: &upper} {
		if target == "" {
			continue
		}
		other, exists := r.files[target]
		if !exists || other.Meta.ParentID != parent || target == id {
			return Meta{}, errors.New("排序参照文档无效")
		}
		*bound = other.Meta.SortKey
	}
	if before == "" && after == "" {
		for otherID, other := range r.files {
			if otherID != id && other.Meta.ParentID == parent && other.Meta.SortKey > lower {
				lower = other.Meta.SortKey
			}
		}
	}
	key, err := fracindex.Between(lower, upper)
	if err != nil {
		return Meta{}, err
	}
	oldPaths := map[string]string{}
	for key := range r.files {
		path, err := PathFor(key, r.files)
		if err != nil {
			return Meta{}, err
		}
		oldPaths[key] = path
	}
	newPath, err := r.layout.Document(ancestors, id)
	if err != nil {
		return Meta{}, err
	}
	oldPath := r.paths[id]
	oldValue := value
	value.Meta.ParentID = parent
	value.Meta.SortKey = key
	value.Meta.Revision++
	value.Meta.UpdatedAt = time.Now().Unix()
	if err := os.MkdirAll(filepath.Dir(newPath), 0700); err != nil {
		return Meta{}, err
	}
	movedChildren := false
	if oldPath != newPath {
		if err := os.Rename(oldPath, newPath); err != nil {
			return Meta{}, err
		}
		oldChildren := oldPath[:len(oldPath)-5]
		newChildren := newPath[:len(newPath)-5]
		if _, err := os.Stat(oldChildren); err == nil {
			if err := os.Rename(oldChildren, newChildren); err != nil {
				return Meta{}, errors.Join(err, os.Rename(newPath, oldPath))
			}
			movedChildren = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return Meta{}, errors.Join(err, os.Rename(newPath, oldPath))
		}
	}
	rollback := func(cause error) error {
		r.files[id] = oldValue
		if movedChildren {
			cause = errors.Join(cause, os.Rename(newPath[:len(newPath)-5], oldPath[:len(oldPath)-5]))
		}
		if oldPath != newPath {
			cause = errors.Join(cause, os.Rename(newPath, oldPath))
		}
		cause = errors.Join(cause, fsutil.WriteJSON(oldPath, oldValue))
		for key, entry := range r.files {
			parents, err := r.ancestorsLocked(entry.Meta.ParentID)
			if err != nil {
				return errors.Join(cause, err)
			}
			path, err := r.layout.Document(parents, key)
			if err != nil {
				return errors.Join(cause, err)
			}
			r.paths[key] = path
		}
		return cause
	}
	if err := fsutil.WriteJSON(newPath, value); err != nil {
		return Meta{}, rollback(err)
	}
	r.files[id] = value
	for key, entry := range r.files {
		parents, err := r.ancestorsLocked(entry.Meta.ParentID)
		if err != nil {
			return Meta{}, rollback(err)
		}
		path, err := r.layout.Document(parents, key)
		if err != nil {
			return Meta{}, rollback(err)
		}
		r.paths[key] = path
	}
	if err := r.rebuildLocked(ctx); err != nil {
		return Meta{}, rollback(err)
	}
	if err := r.recordChangedPathsLocked(oldPaths); err != nil {
		return Meta{}, err
	}
	return value.Meta, nil
}
