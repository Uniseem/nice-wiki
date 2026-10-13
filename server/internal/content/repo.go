package content

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"io/fs"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/fracindex"
	"nice-wiki/internal/fsutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("文档不存在")
var ErrConflict = errors.New("文档版本或状态冲突")
var ErrLocked = errors.New("文档编辑锁已失效或由其他账号持有")

type Repo struct {
	mu            sync.RWMutex
	layout        datadir.Layout
	db            *sql.DB
	files         map[string]File
	paths         map[string]string
	history       History
	onPublish     PublishedHook
	onChange      func()
	slugGenerator SlugGenerator
	seoGenerator  SEOGenerator
	locks         *Locks
}

func newID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
func Open(ctx context.Context, layout datadir.Layout, db *sql.DB, history History, onPublish PublishedHook) (*Repo, error) {
	r := &Repo{layout: layout, db: db, files: map[string]File{}, paths: map[string]string{}, history: history, onPublish: onPublish, locks: NewLocks()}
	if err := filepath.WalkDir(layout.Docs(), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("正文目录不允许符号链接")
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			return errors.New("正文目录含未知文件")
		}
		var value File
		if err := fsutil.ReadJSON(path, &value); err != nil {
			return err
		}
		if err := value.Validate(); err != nil {
			return err
		}
		if _, exists := r.files[value.Meta.ID]; exists {
			return errors.New("文档 ID 重复")
		}
		r.files[value.Meta.ID] = value
		r.paths[value.Meta.ID] = path
		return nil
	}); err != nil {
		return nil, err
	}
	for id, value := range r.files {
		ancestors, err := r.ancestorsLocked(value.Meta.ParentID)
		if err != nil {
			return nil, err
		}
		expected, err := layout.Document(ancestors, id)
		if err != nil {
			return nil, err
		}
		if expected != r.paths[id] {
			return nil, errors.New("正文文件路径与父子关系不一致")
		}
	}
	if err := r.rebuildLocked(ctx); err != nil {
		return nil, err
	}
	return r, nil
}
func (r *Repo) ancestorsLocked(parent string) ([]string, error) {
	result := []string{}
	seen := map[string]bool{}
	for parent != "" {
		if seen[parent] {
			return nil, errors.New("文档树循环")
		}
		seen[parent] = true
		value, ok := r.files[parent]
		if !ok {
			return nil, ErrNotFound
		}
		result = append([]string{parent}, result...)
		parent = value.Meta.ParentID
	}
	return result, nil
}
func (r *Repo) Get(id string) (File, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.files[id]
	if !ok {
		return File{}, ErrNotFound
	}
	return cloneFile(value), nil
}
func cloneFile(v File) File {
	// 通过序列化隔离 map 和 slice，调用方不能修改仓库内存状态。
	data, _ := jsonMarshal(v)
	var result File
	_ = jsonUnmarshal(data, &result)
	return result
}
func (r *Repo) Create(ctx context.Context, parent, title, slug string) (Meta, error) {
	if err := ValidateSlug(slug); err != nil {
		return Meta{}, err
	}
	id, err := newID()
	if err != nil {
		return Meta{}, err
	}
	blockID, err := newID()
	if err != nil {
		return Meta{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ancestors, err := r.ancestorsLocked(parent)
	if err != nil {
		return Meta{}, err
	}
	last := ""
	for _, value := range r.files {
		if value.Meta.ParentID == parent {
			if value.Meta.Slug == slug {
				return Meta{}, ErrConflict
			}
			if value.Meta.SortKey > last {
				last = value.Meta.SortKey
			}
		}
	}
	sortKey, err := fracindex.Between(last, "")
	if err != nil {
		return Meta{}, err
	}
	value := File{Meta: Meta{ID: id, ParentID: parent, Title: title, Slug: slug, SortKey: sortKey, Revision: 1, UpdatedAt: time.Now().Unix()}, Document: doc.Document{SchemaVersion: doc.SchemaVersion, Content: doc.Node{Type: "doc", Content: []doc.Node{{Type: "paragraph", Attrs: map[string]any{"id": blockID}}}}}}
	if err := value.Validate(); err != nil {
		return Meta{}, err
	}
	path, err := r.layout.Document(ancestors, id)
	if err != nil {
		return Meta{}, err
	}
	if err := fsutil.WriteJSON(path, value); err != nil {
		return Meta{}, err
	}
	r.files[id] = value
	r.paths[id] = path
	if err := r.rebuildLocked(ctx); err != nil {
		delete(r.files, id)
		delete(r.paths, id)
		return Meta{}, errors.Join(err, os.Remove(path))
	}
	return value.Meta, nil
}
func (r *Repo) Path(id string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return PathFor(id, r.files)
}
func (r *Repo) HistoryPath(id string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	path, ok := r.paths[id]
	if !ok {
		return "", ErrNotFound
	}
	relative, err := filepath.Rel(r.layout.Root, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(relative), nil
}
func (r *Repo) ResolvePath(path string) (File, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for id, value := range r.files {
		resolved, err := PathFor(id, r.files)
		if err == nil && resolved == path && value.Meta.Published {
			return cloneFile(value), true
		}
	}
	return File{}, false
}
func (r *Repo) Locks() *Locks { return r.locks }
func (r *Repo) SetPublishedHook(hook PublishedHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onPublish = hook
}
func (r *Repo) SetGenerators(slug SlugGenerator, seo SEOGenerator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slugGenerator = slug
	r.seoGenerator = seo
}

// SourceDocument 是资源清理所需的当前正文、共享草稿和回收站快照。
type SourceDocument struct {
	ID       string
	Source   string
	Document doc.Document
}

func (r *Repo) ResourceDocuments() ([]SourceDocument, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.resourceDocumentsLocked()
}

// WithResourceDocuments 在整个资源检查与清理过程中阻止正文和草稿变化。
func (r *Repo) WithResourceDocuments(visit func([]SourceDocument) error) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	values, err := r.resourceDocumentsLocked()
	if err != nil {
		return err
	}
	return visit(values)
}
func (r *Repo) resourceDocumentsLocked() ([]SourceDocument, error) {
	result := []SourceDocument{}
	for id, value := range r.files {
		result = append(result, SourceDocument{ID: id, Source: "document", Document: cloneFile(value).Document})
		draft, err := r.draftLocked(id)
		if err != nil {
			return nil, err
		}
		if draft.Version > 0 {
			result = append(result, SourceDocument{ID: id, Source: "draft", Document: draft.Document})
		}
	}
	entries, err := os.ReadDir(r.layout.Trash())
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		path, err := r.layout.Trashed(id)
		if err != nil {
			return nil, err
		}
		var value File
		if err := fsutil.ReadJSON(path, &value); err != nil {
			return nil, err
		}
		result = append(result, SourceDocument{ID: id, Source: "trash", Document: value.Document})
		if value.Draft != nil {
			result = append(result, SourceDocument{ID: id, Source: "trash-draft", Document: value.Draft.Document})
		}
	}
	return result, nil
}

func (r *Repo) SetChangedHook(hook func()) { r.mu.Lock(); defer r.mu.Unlock(); r.onChange = hook }

// WithPublished 保证异步派生写回期间正文版本和发布状态不变。
func (r *Repo) WithPublished(id string, revision int64, commit func() error) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	file, ok := r.files[id]
	if !ok || !file.Meta.Published || file.Meta.Revision != revision {
		return ErrConflict
	}
	return commit()
}
