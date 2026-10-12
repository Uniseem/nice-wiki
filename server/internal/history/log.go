package history

import (
	"context"
	"errors"
	"github.com/go-git/go-git/v5"
	"io"
	"time"
)

type Revision struct {
	Hash    string    `json:"hash"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	Time    time.Time `json:"time"`
}

func (r *Repo) Log(ctx context.Context, file string, limit int) ([]Revision, error) {
	if !allowed(file) {
		return nil, errors.New("历史路径无效")
	}
	if limit < 1 || limit > 100 {
		limit = 30
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	iterator, err := r.repository.Log(&git.LogOptions{FileName: &file})
	if err != nil {
		return nil, err
	}
	defer iterator.Close()
	result := []Revision{}
	for len(result) < limit {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		commit, err := iterator.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		result = append(result, Revision{Hash: commit.Hash.String(), Message: commit.Message, Author: commit.Author.Name, Time: commit.Author.When})
	}
	return result, nil
}
