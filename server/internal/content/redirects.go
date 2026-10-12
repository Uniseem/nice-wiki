package content

import (
	"errors"
	"nice-wiki/internal/fsutil"
	"os"
)

type Redirect struct {
	DocumentID string `json:"documentId"`
	Gone       bool   `json:"gone"`
}

func (r *Repo) readRedirectsLocked() (map[string]Redirect, error) {
	result := map[string]Redirect{}
	if err := fsutil.ReadJSON(r.layout.Redirects(), &result); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return result, nil
}
func (r *Repo) recordChangedPathsLocked(old map[string]string) error {
	redirects, err := r.readRedirectsLocked()
	if err != nil {
		return err
	}
	for id, path := range old {
		current, err := PathFor(id, r.files)
		if err != nil {
			return err
		}
		if current != path {
			redirects[path] = Redirect{DocumentID: id}
		}
	}
	return fsutil.WriteJSON(r.layout.Redirects(), redirects)
}
func (r *Repo) Redirect(path string) (Redirect, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	values, err := r.readRedirectsLocked()
	if err != nil {
		return Redirect{}, false, err
	}
	value, ok := values[path]
	return value, ok, nil
}
