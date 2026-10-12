package history

import (
	"errors"
	"github.com/go-git/go-git/v5"
	"sync"
)

type Repo struct {
	mu         sync.Mutex
	repository *git.Repository
}

func Open(root string) (*Repo, error) {
	repository, err := git.PlainOpen(root)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		repository, err = git.PlainInit(root, false)
	}
	if err != nil {
		return nil, err
	}
	return &Repo{repository: repository}, nil
}
