package content

import (
	"errors"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/fsutil"
	"os"
	"time"
)

type Draft struct {
	DocumentID string       `json:"documentId"`
	Version    int64        `json:"version"`
	Document   doc.Document `json:"document"`
	UpdatedAt  int64        `json:"updatedAt"`
}

func (r *Repo) draftLocked(id string) (Draft, error) {
	value, ok := r.files[id]
	if !ok {
		return Draft{}, ErrNotFound
	}
	path, err := r.layout.Draft(id)
	if err != nil {
		return Draft{}, err
	}
	var draft Draft
	err = fsutil.ReadJSON(path, &draft)
	if errors.Is(err, os.ErrNotExist) {
		return Draft{DocumentID: id, Version: 0, Document: cloneFile(value).Document, UpdatedAt: value.Meta.UpdatedAt}, nil
	}
	if err != nil {
		return draft, err
	}
	if draft.DocumentID != id {
		return draft, errors.New("草稿身份不匹配")
	}
	return draft, doc.Validate(draft.Document)
}
func (r *Repo) Draft(id string) (Draft, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.draftLocked(id)
}
func (r *Repo) SaveDraft(id, account, token string, version int64, document doc.Document) (Draft, error) {
	if err := doc.Validate(document); err != nil {
		return Draft{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.locks.Check(id, account, token) {
		return Draft{}, ErrLocked
	}
	old, err := r.draftLocked(id)
	if err != nil {
		return Draft{}, err
	}
	if old.Version != version {
		return Draft{}, ErrConflict
	}
	next := Draft{DocumentID: id, Version: version + 1, Document: document, UpdatedAt: time.Now().Unix()}
	path, err := r.layout.Draft(id)
	if err != nil {
		return Draft{}, err
	}
	if err := fsutil.WriteJSON(path, next); err != nil {
		return Draft{}, err
	}
	return next, nil
}
