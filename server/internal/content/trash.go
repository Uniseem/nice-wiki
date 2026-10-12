package content

import (
	"context"
	"errors"
	"nice-wiki/internal/fracindex"
	"nice-wiki/internal/fsutil"
	"os"
	"sort"
	"strings"
	"time"
)

func (r *Repo) Trash(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, exists := r.files[id]
	if !exists {
		return ErrNotFound
	}
	for _, other := range r.files {
		if other.Meta.ParentID == id {
			return errors.New("请先移走或删除子文档")
		}
	}
	stored := value
	draft, err := r.draftLocked(id)
	if err != nil {
		return err
	}
	if draft.Version > 0 {
		value.Draft = &draft
	}
	path, err := r.layout.Trashed(id)
	if err != nil {
		return err
	}
	if err := fsutil.WriteJSON(path, value); err != nil {
		return err
	}
	oldPath := r.paths[id]
	oldRoute, err := PathFor(id, r.files)
	if err != nil {
		return err
	}
	if err := os.Remove(oldPath); err != nil {
		return err
	}
	delete(r.files, id)
	delete(r.paths, id)
	if err := r.rebuildLocked(ctx); err != nil {
		r.files[id] = stored
		r.paths[id] = oldPath
		return errors.Join(err, fsutil.WriteJSON(oldPath, stored))
	}
	redirects, err := r.readRedirectsLocked()
	if err != nil {
		return err
	}
	redirects[oldRoute] = Redirect{DocumentID: id, Gone: true}
	r.locks.ForceRelease(id)
	return fsutil.WriteJSON(r.layout.Redirects(), redirects)
}

func (r *Repo) ListTrash() ([]Meta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries, err := os.ReadDir(r.layout.Trash())
	if err != nil {
		return nil, err
	}
	result := []Meta{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		path, err := r.layout.Trashed(id)
		if err != nil {
			return nil, err
		}
		var value File
		if err := fsutil.ReadJSON(path, &value); err != nil {
			return nil, err
		}
		if value.Meta.ID != id {
			return nil, errors.New("回收站文档身份不一致")
		}
		result = append(result, value.Meta)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt > result[j].UpdatedAt })
	return result, nil
}

func (r *Repo) Restore(ctx context.Context, id string) (Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.files[id]; exists {
		return Meta{}, ErrConflict
	}
	trashPath, err := r.layout.Trashed(id)
	if err != nil {
		return Meta{}, err
	}
	var value File
	if err := fsutil.ReadJSON(trashPath, &value); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Meta{}, ErrNotFound
		}
		return Meta{}, err
	}
	if value.Meta.ID != id {
		return Meta{}, errors.New("回收站文档身份不一致")
	}
	if err := value.Validate(); err != nil {
		return Meta{}, err
	}
	ancestors, err := r.ancestorsLocked(value.Meta.ParentID)
	if err != nil {
		return Meta{}, invalid("请先恢复父文档，或调整恢复位置")
	}
	last := ""
	for _, other := range r.files {
		if other.Meta.ParentID != value.Meta.ParentID {
			continue
		}
		if other.Meta.Slug == value.Meta.Slug {
			return Meta{}, invalid("同级已有相同路径名的文档，暂时不能恢复")
		}
		if other.Meta.SortKey > last {
			last = other.Meta.SortKey
		}
	}
	sortKey, err := fracindex.Between(last, "")
	if err != nil {
		return Meta{}, err
	}
	path, err := r.layout.Document(ancestors, id)
	if err != nil {
		return Meta{}, err
	}
	draft := value.Draft
	value.Draft = nil
	value.Meta.SortKey = sortKey
	value.Meta.Revision++
	value.Meta.UpdatedAt = time.Now().Unix()
	if err := fsutil.WriteJSON(path, value); err != nil {
		return Meta{}, err
	}
	r.files[id] = value
	r.paths[id] = path
	rollback := func(cause error) error {
		delete(r.files, id)
		delete(r.paths, id)
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		cause = errors.Join(cause, os.Remove(path), r.rebuildLocked(recovery))
		return cause
	}
	if err := r.rebuildLocked(ctx); err != nil {
		return Meta{}, rollback(err)
	}
	if draft != nil {
		draftPath, err := r.layout.Draft(id)
		if err != nil {
			return Meta{}, rollback(err)
		}
		if err := fsutil.WriteJSON(draftPath, draft); err != nil {
			return Meta{}, rollback(err)
		}
	}
	redirects, err := r.readRedirectsLocked()
	if err != nil {
		return Meta{}, rollback(err)
	}
	for route, redirect := range redirects {
		if redirect.DocumentID == id && redirect.Gone {
			delete(redirects, route)
		}
	}
	if err := fsutil.WriteJSON(r.layout.Redirects(), redirects); err != nil {
		return Meta{}, rollback(err)
	}
	if err := os.Remove(trashPath); err != nil {
		return value.Meta, err
	}
	return value.Meta, nil
}
