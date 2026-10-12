package history

import (
	"context"
	"errors"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
	"path"
	"strings"
	"time"
)

func allowed(name string) bool {
	if name != path.Clean(name) || strings.HasPrefix(name, "/") {
		return false
	}
	if name == "redirects.json" || name == ".gitignore" {
		return true
	}
	for _, prefix := range []string{"docs/", "i18n/", "settings/", "memory/"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
func (r *Repo) Commit(ctx context.Context, account, message string, files []string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	worktree, err := r.repository.Worktree()
	if err != nil {
		return "", err
	}
	// 每次提交仅使用本次明确选择的文件，清除失败提交遗留的暂存状态。
	if _, headErr := r.repository.Head(); headErr == nil {
		if err := worktree.Reset(&git.ResetOptions{Mode: git.MixedReset}); err != nil {
			return "", err
		}
	} else if errors.Is(headErr, plumbing.ErrReferenceNotFound) {
		if err := r.repository.Storer.SetIndex(&index.Index{Version: 2}); err != nil {
			return "", err
		}
	} else {
		return "", headErr
	}
	for _, name := range files {
		if !allowed(name) {
			return "", errors.New("版本仓库不允许提交该路径")
		}
		if _, err := worktree.Add(name); err != nil {
			return "", err
		}
	}
	status, err := worktree.Status()
	if err != nil {
		return "", err
	}
	if status.IsClean() {
		head, err := r.repository.Head()
		if err != nil {
			return "", err
		}
		return head.Hash().String(), nil
	}
	// data 历史只有后台账号名，使用保留域名代用地址。
	signature := &object.Signature{Name: account, Email: "wiki@example.com", When: time.Now()}
	hash, err := worktree.Commit(message, &git.CommitOptions{Author: signature})
	if err != nil {
		return "", err
	}
	return hash.String(), nil
}
