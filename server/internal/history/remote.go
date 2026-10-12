package history

import (
	"context"
	"errors"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func (r *Repo) Push(ctx context.Context, url, username, password string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	remote, err := r.repository.Remote("backup")
	if errors.Is(err, git.ErrRemoteNotFound) {
		remote, err = r.repository.CreateRemote(&config.RemoteConfig{Name: "backup", URLs: []string{url}})
	}
	if err != nil {
		return err
	}
	options := &git.PushOptions{RemoteName: "backup", Force: false}
	if username != "" || password != "" {
		options.Auth = &http.BasicAuth{Username: username, Password: password}
	}
	err = remote.PushContext(ctx, options)
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}
