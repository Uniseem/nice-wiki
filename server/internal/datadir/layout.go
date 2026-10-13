package datadir

import (
	"errors"
	"path/filepath"
	"regexp"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var language = regexp.MustCompile(`^[A-Za-z]{2,8}(?:-[A-Za-z0-9]{1,8})*$`)
var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Layout 是运行时路径的唯一来源，动态路径必须通过验证。
type Layout struct{ Root string }

func New(root string) (Layout, error) {
	if root == "" {
		return Layout{}, errors.New("数据目录不能为空")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, err
	}
	return Layout{Root: filepath.Clean(absolute)}, nil
}

func (l Layout) Docs() string            { return filepath.Join(l.Root, "docs") }
func (l Layout) Drafts() string          { return filepath.Join(l.Root, "drafts") }
func (l Layout) Trash() string           { return filepath.Join(l.Root, "trash") }
func (l Layout) Assets() string          { return filepath.Join(l.Root, "assets") }
func (l Layout) Config() string          { return filepath.Join(l.Root, "config") }
func (l Layout) Settings() string        { return filepath.Join(l.Root, "settings") }
func (l Layout) I18n() string            { return filepath.Join(l.Root, "i18n") }
func (l Layout) Memory() string          { return filepath.Join(l.Root, "memory") }
func (l Layout) Backups() string         { return filepath.Join(l.Root, "backups") }
func (l Layout) Cache() string           { return filepath.Join(l.Root, "cache") }
func (l Layout) Database() string        { return filepath.Join(l.Root, "wiki.db") }
func (l Layout) LockFile() string        { return filepath.Join(l.Root, ".nice-wiki.lock") }
func (l Layout) GitIgnore() string       { return filepath.Join(l.Root, ".gitignore") }
func (l Layout) Bootstrap() string       { return filepath.Join(l.Config(), "config.toml") }
func (l Layout) Runtime() string         { return filepath.Join(l.Config(), "runtime.json") }
func (l Layout) Secrets() string         { return filepath.Join(l.Config(), "secrets.json") }
func (l Layout) Channels() string        { return filepath.Join(l.Config(), "channels.json") }
func (l Layout) Accounts() string        { return filepath.Join(l.Config(), "accounts.json") }
func (l Layout) ResetSuperadmin() string { return filepath.Join(l.Config(), "reset-superadmin.txt") }
func (l Layout) Site() string            { return filepath.Join(l.Settings(), "site.json") }
func (l Layout) Home() string            { return filepath.Join(l.Settings(), "home.json") }
func (l Layout) Banner() string          { return filepath.Join(l.Settings(), "banner.json") }
func (l Layout) Languages() string       { return filepath.Join(l.Settings(), "languages.json") }
func (l Layout) AI() string              { return filepath.Join(l.Settings(), "ai.json") }
func (l Layout) Glossary() string        { return filepath.Join(l.I18n(), "glossary.json") }
func (l Layout) MemoryTasks() string     { return filepath.Join(l.Memory(), "_tasks.json") }
func (l Layout) ModelCatalog() string    { return filepath.Join(l.Cache(), "models.json") }
func (l Layout) GeoIPDatabase() string {
	return filepath.Join(l.Cache(), "geoip", "dbip-country-lite.mmdb")
}
func (l Layout) PendingRestore() string  { return filepath.Join(l.Config(), "restore-pending.json") }
func (l Layout) RestoreJournal() string  { return filepath.Join(l.Root, ".restore-journal.json") }
func (l Layout) RestorePrevious() string { return filepath.Join(l.Root, ".restore-previous") }
func (l Layout) Redirects() string       { return filepath.Join(l.Root, "redirects.json") }

func ValidID(id string) bool { return identifier.MatchString(id) }
func (l Layout) Document(ancestors []string, id string) (string, error) {
	parts := []string{l.Docs()}
	for _, part := range append(append([]string{}, ancestors...), id) {
		if !ValidID(part) {
			return "", errors.New("文档 ID 无效")
		}
		parts = append(parts, part)
	}
	parts[len(parts)-1] += ".json"
	return filepath.Join(parts...), nil
}
func (l Layout) DocumentChildren(ancestors []string, id string) (string, error) {
	path, err := l.Document(ancestors, id)
	if err != nil {
		return "", err
	}
	return path[:len(path)-5], nil
}
func (l Layout) Draft(id string) (string, error) {
	if !ValidID(id) {
		return "", errors.New("草稿 ID 无效")
	}
	return filepath.Join(l.Drafts(), id+".json"), nil
}
func (l Layout) Trashed(id string) (string, error) {
	if !ValidID(id) {
		return "", errors.New("回收站 ID 无效")
	}
	return filepath.Join(l.Trash(), id+".json"), nil
}
func (l Layout) Translation(lang, id string) (string, error) {
	if !language.MatchString(lang) || !ValidID(id) {
		return "", errors.New("译文路径参数无效")
	}
	return filepath.Join(l.I18n(), lang, id+".json"), nil
}
func (l Layout) TranslationStrings(lang, name string) (string, error) {
	if !language.MatchString(lang) || name != "site" && name != "ui" {
		return "", errors.New("译文字符串路径参数无效")
	}
	return filepath.Join(l.I18n(), lang, "_"+name+".json"), nil
}
func (l Layout) MemoryEntry(id string) (string, error) {
	if !ValidID(id) || id == "_tasks" {
		return "", errors.New("记忆 ID 无效")
	}
	return filepath.Join(l.Memory(), id+".md"), nil
}
func (l Layout) Asset(kind, hash, extension string) (string, error) {
	switch kind {
	case "images", "files", "videos", "drawings":
	default:
		return "", errors.New("资源类型无效")
	}
	if !digest.MatchString(hash) || !regexp.MustCompile(`^\.[a-z0-9]{1,16}$`).MatchString(extension) {
		return "", errors.New("资源名称无效")
	}
	return filepath.Join(l.Assets(), kind, hash+extension), nil
}
