package history

import (
	"context"
	"errors"
	"github.com/go-git/go-git/v5/plumbing"
	"regexp"
)

var revisionHash = regexp.MustCompile(`^[a-f0-9]{40}$`)

func (r *Repo) Read(ctx context.Context, revision, file string) ([]byte, error) {
	if !revisionHash.MatchString(revision) || !allowed(file) {
		return nil, errors.New("历史版本或路径无效")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	commit, err := r.repository.CommitObject(plumbing.NewHash(revision))
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	entry, err := tree.File(file)
	if err != nil {
		return nil, err
	}
	value, err := entry.Contents()
	return []byte(value), err
}
