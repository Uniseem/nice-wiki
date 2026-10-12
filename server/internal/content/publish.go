package content

import (
	"context"
	"errors"
	"nice-wiki/internal/fsutil"
	"os"
	"path/filepath"
	"time"
)

func (r *Repo) Publish(ctx context.Context, id, account, name, token string, version int64, title, slug, description string) (Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.locks.Check(id, account, token) {
		return Meta{}, ErrLocked
	}
	old, exists := r.files[id]
	if !exists {
		return Meta{}, ErrNotFound
	}
	draft, err := r.draftLocked(id)
	if err != nil {
		return Meta{}, err
	}
	if draft.Version != version {
		return Meta{}, ErrConflict
	}
	value := old
	value.Document = draft.Document
	value.Meta.Title = title
	value.Meta.Slug = slug
	value.Meta.SEODescription = description
	value.Meta.Published = true
	value.Meta.Revision++
	value.Meta.UpdatedAt = time.Now().Unix()
	if err := value.Validate(); err != nil {
		return Meta{}, err
	}
	for otherID, other := range r.files {
		if otherID != id && other.Meta.ParentID == value.Meta.ParentID && other.Meta.Slug == slug {
			return Meta{}, ErrConflict
		}
	}
	oldPaths := map[string]string{}
	for key := range r.files {
		path, err := PathFor(key, r.files)
		if err != nil {
			return Meta{}, err
		}
		oldPaths[key] = path
	}
	oldRedirects, err := r.readRedirectsLocked()
	if err != nil {
		return Meta{}, err
	}
	_, redirectStat := os.Stat(r.layout.Redirects())
	redirectsExisted := redirectStat == nil
	if redirectStat != nil && !errors.Is(redirectStat, os.ErrNotExist) {
		return Meta{}, redirectStat
	}
	relative, err := filepath.Rel(r.layout.Root, r.paths[id])
	if err != nil {
		return Meta{}, err
	}
	historyFiles := []string{filepath.ToSlash(relative), "redirects.json"}
	historyAttempted := false
	rollback := func(cause error) error {
		// 请求取消后仍完成恢复，避免留下半次发布；恢复有独立的时间上限。
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		r.files[id] = old
		cause = errors.Join(cause, fsutil.WriteJSON(r.paths[id], old))
		if redirectsExisted {
			cause = errors.Join(cause, fsutil.WriteJSON(r.layout.Redirects(), oldRedirects))
		} else {
			if removeErr := os.Remove(r.layout.Redirects()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				cause = errors.Join(cause, removeErr)
			}
		}
		cause = errors.Join(cause, r.rebuildLocked(recovery))
		if historyAttempted && r.history != nil {
			files := []string{filepath.ToSlash(relative)}
			if redirectsExisted {
				files = append(files, "redirects.json")
			}
			_, historyErr := r.history.Commit(recovery, name, "撤销未完成的发布 "+old.Meta.Title, files)
			cause = errors.Join(cause, historyErr)
		}
		return cause
	}
	if err := fsutil.WriteJSON(r.paths[id], value); err != nil {
		return Meta{}, rollback(err)
	}
	r.files[id] = value
	if err := r.rebuildLocked(ctx); err != nil {
		return Meta{}, rollback(err)
	}
	if err := r.recordChangedPathsLocked(oldPaths); err != nil {
		return Meta{}, rollback(err)
	}
	if r.history != nil {
		historyAttempted = true
		if _, err := r.history.Commit(ctx, name, "发布文档 "+title, historyFiles); err != nil {
			return Meta{}, rollback(err)
		}
	}
	// 后续付费任务只有在实现并注入后才投递，草稿保存从不调用此入口。
	if r.onPublish != nil {
		if err := r.onPublish(ctx, id); err != nil {
			return value.Meta, err
		}
	}
	return value.Meta, nil
}
