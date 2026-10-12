package content

import (
	"context"
	"errors"
	"nice-wiki/internal/fsutil"
	"time"
)

func (r *Repo) Withdraw(ctx context.Context, id string) (Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, exists := r.files[id]
	if !exists {
		return Meta{}, ErrNotFound
	}
	value := old
	value.Meta.Published = false
	value.Meta.Revision++
	value.Meta.UpdatedAt = time.Now().Unix()
	if err := fsutil.WriteJSON(r.paths[id], value); err != nil {
		return Meta{}, err
	}
	r.files[id] = value
	if err := r.rebuildLocked(ctx); err != nil {
		r.files[id] = old
		return Meta{}, errors.Join(err, fsutil.WriteJSON(r.paths[id], old))
	}
	return value.Meta, nil
}
