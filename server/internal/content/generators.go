package content

import (
	"context"
	"errors"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/fsutil"
	"path/filepath"
	"strings"
	"time"
)

// 后续模型功能以接口注入，基础文档管理不依赖模型实现。
type SlugGenerator interface {
	GenerateSlug(context.Context, string) (string, error)
}
type SEOGenerator interface {
	GenerateDescription(context.Context, File) (string, error)
}
type History interface {
	Commit(context.Context, string, string, []string) (string, error)
}
type PublishedHook func(context.Context, string) error

func (r *Repo) ApplyGeneratedDescriptions(ctx context.Context, id string, revision int64, descriptions map[string]string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.files[id]
	if !ok || !old.Meta.Published || old.Meta.Revision != revision {
		return false, ErrConflict
	}
	value := cloneFile(old)
	changed := false
	var walk func(*doc.Node)
	walk = func(node *doc.Node) {
		blockID, _ := node.Attrs["id"].(string)
		candidate := strings.TrimSpace(descriptions[blockID])
		if runes := []rune(candidate); len(runes) > 2000 {
			candidate = string(runes[:2000])
		}
		if candidate != "" && (node.Type == "image" || node.Type == "drawing") {
			alt, _ := node.Attrs["alt"].(string)
			if strings.TrimSpace(alt) == "" {
				node.Attrs["alt"] = candidate
				changed = true
			}
		}
		for index := range node.Content {
			walk(&node.Content[index])
		}
	}
	walk(&value.Document.Content)
	if !changed {
		return false, nil
	}
	value.Meta.Revision++
	value.Meta.UpdatedAt = time.Now().Unix()
	if err := value.Validate(); err != nil {
		return false, err
	}
	path := r.paths[id]
	rollback := func(cause error) error {
		r.files[id] = old
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		return errors.Join(cause, fsutil.WriteJSON(path, old), r.rebuildLocked(recovery))
	}
	if err := fsutil.WriteJSON(path, value); err != nil {
		return false, err
	}
	r.files[id] = value
	if err := r.rebuildLocked(ctx); err != nil {
		return false, rollback(err)
	}
	if r.history != nil {
		relative, err := filepath.Rel(r.layout.Root, path)
		if err != nil {
			return false, rollback(err)
		}
		if _, err := r.history.Commit(ctx, "nice-wiki", "补充媒体默认描述 "+value.Meta.Title, []string{filepath.ToSlash(relative)}); err != nil {
			return false, rollback(err)
		}
	}
	return true, nil
}
