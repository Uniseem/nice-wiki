package transport

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/data-protection/internal/gen"
)

// 简易在线备份：把配置里列出的各服务数据目录打成一个 zip。
// SQLite 库用 VACUUM INTO 取一致快照，其余文件原样复制；不含密钥目录。
// 创建同步完成，下载用一次性短期令牌。

var (
	backupMu     sync.Mutex
	downloadMu   sync.Mutex
	downloadAuth = map[string]downloadGrant{}
)

type downloadGrant struct {
	backupID string
	expires  time.Time
}

type backupDomain struct {
	Slug  string `json:"slug"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

type backupManifest struct {
	BackupID       string         `json:"backupId"`
	CreatedAt      string         `json:"createdAt"`
	Format         string         `json:"format"`
	Domains        []backupDomain `json:"domains"`
	ByteSize       int64          `json:"byteSize"`
	SHA256         string         `json:"sha256"`
	IncludeSecrets bool           `json:"includeSecrets"`
}

func (s *server) backupRoot() string {
	return filepath.Join(s.app.config.DataRoot, "simple-backups")
}

func (s *server) simpleCreateBackup(ctx context.Context, op platform.OperationContext, includeSecrets bool) (gen.CreateBackupResult, error) {
	if op.Actor.Kind != "admin" {
		return gen.CreateBackupResult{}, platform.NewError(platform.CodeForbidden, false, "只有后台管理员可以创建备份")
	}
	if len(s.app.config.BackupSources) == 0 {
		return gen.CreateBackupResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "未配置备份来源目录")
	}
	backupMu.Lock()
	defer backupMu.Unlock()
	id := platform.NewIdentifier()
	dir := filepath.Join(s.backupRoot(), id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return gen.CreateBackupResult{}, platform.NewError(platform.CodeInternal, true, "无法创建备份目录")
	}
	manifest, err := writeBackupArchive(ctx, filepath.Join(dir, "backup.zip"), s.app.config.BackupSources)
	if err != nil {
		_ = os.RemoveAll(dir)
		return gen.CreateBackupResult{}, err
	}
	manifest.BackupID = id
	manifest.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	manifest.Format = "nice-wiki-simple-zip/1"
	manifest.IncludeSecrets = false
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return gen.CreateBackupResult{}, platform.NewError(platform.CodeInternal, true, "无法写入备份清单")
	}
	return gen.CreateBackupResult{BackupId: id, Job: gen.AcceptedJob{JobId: platform.NewIdentifier(), OperationId: op.OperationID, State: "succeeded"}}, nil
}

func writeBackupArchive(ctx context.Context, target string, sources map[string]string) (backupManifest, error) {
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return backupManifest{}, platform.NewError(platform.CodeInternal, true, "无法创建备份文件")
	}
	defer out.Close()
	hash := sha256.New()
	zw := zip.NewWriter(io.MultiWriter(out, hash))
	scratch, err := os.MkdirTemp(filepath.Dir(target), "snap-")
	if err != nil {
		return backupManifest{}, platform.NewError(platform.CodeInternal, true, "无法创建临时目录")
	}
	defer os.RemoveAll(scratch)
	slugs := make([]string, 0, len(sources))
	for slug := range sources {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	var manifest backupManifest
	for _, slug := range slugs {
		domain := backupDomain{Slug: slug}
		root := sources[slug]
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			if entry.IsDir() || !entry.Type().IsRegular() || strings.HasSuffix(rel, "-wal") || strings.HasSuffix(rel, "-shm") || strings.HasSuffix(rel, "-journal") {
				return nil
			}
			source := path
			if strings.HasSuffix(rel, ".db") {
				snap := filepath.Join(scratch, slug+"-"+hex.EncodeToString(randomBytes(6))+".db")
				if err := vacuumInto(ctx, path, snap); err != nil {
					return err
				}
				source = snap
				defer os.Remove(snap)
			}
			size, err := addZipFile(zw, slug+"/"+filepath.ToSlash(rel), source)
			if err != nil {
				return err
			}
			domain.Files++
			domain.Bytes += size
			return nil
		})
		if err != nil {
			return backupManifest{}, platform.NewError(platform.CodeInternal, true, "打包服务数据失败")
		}
		manifest.Domains = append(manifest.Domains, domain)
	}
	if err := zw.Close(); err != nil {
		return backupManifest{}, platform.NewError(platform.CodeInternal, true, "备份压缩收尾失败")
	}
	info, err := out.Stat()
	if err != nil {
		return backupManifest{}, platform.NewError(platform.CodeInternal, true, "备份文件不可读")
	}
	manifest.ByteSize = info.Size()
	manifest.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return manifest, nil
}

func vacuumInto(ctx context.Context, path, target string) error {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, "VACUUM INTO ?", target)
	return err
}

func addZipFile(zw *zip.Writer, name, source string) (int64, error) {
	in, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	w, err := zw.Create(name)
	if err != nil {
		return 0, err
	}
	return io.Copy(w, in)
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func (s *server) simpleListBackups() (gen.ListBackupsResult, error) {
	entries, err := os.ReadDir(s.backupRoot())
	if err != nil && !os.IsNotExist(err) {
		return gen.ListBackupsResult{}, platform.NewError(platform.CodeInternal, true, "无法读取备份目录")
	}
	type item struct {
		id string
		at time.Time
	}
	var items []item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(s.backupRoot(), e.Name(), "manifest.json")); err == nil {
			items = append(items, item{e.Name(), info.ModTime()})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].at.After(items[j].at) })
	result := gen.ListBackupsResult{BackupIds: []string{}}
	for _, it := range items {
		result.BackupIds = append(result.BackupIds, it.id)
	}
	return result, nil
}

func (s *server) backupDir(id string) (string, error) {
	if !platform.ValidIdentifier(platform.Identifier(id)) {
		return "", platform.NewError(platform.CodeNotFound, false, "备份不存在")
	}
	dir := filepath.Join(s.backupRoot(), id)
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		return "", platform.NewError(platform.CodeNotFound, false, "备份不存在")
	}
	return dir, nil
}

func (s *server) simpleReadBackup(id string) (gen.ReadBackupResult, error) {
	dir, err := s.backupDir(id)
	if err != nil {
		return gen.ReadBackupResult{}, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return gen.ReadBackupResult{}, platform.NewError(platform.CodeInternal, true, "无法读取备份清单")
	}
	return gen.ReadBackupResult{Issues: []gen.SafeIssue{}, ManifestJSON: string(raw), State: "ready"}, nil
}

func (s *server) simpleAuthorizeDownload(op platform.OperationContext, id string) (gen.AuthorizeBackupDownloadResult, error) {
	if op.Actor.Kind != "admin" {
		return gen.AuthorizeBackupDownloadResult{}, platform.NewError(platform.CodeForbidden, false, "只有后台管理员可以下载备份")
	}
	if _, err := s.backupDir(id); err != nil {
		return gen.AuthorizeBackupDownloadResult{}, err
	}
	token := hex.EncodeToString(randomBytes(24))
	expires := time.Now().UTC().Add(2 * time.Minute)
	downloadMu.Lock()
	defer downloadMu.Unlock()
	for k, g := range downloadAuth {
		if g.expires.Before(time.Now()) {
			delete(downloadAuth, k)
		}
	}
	downloadAuth[token] = downloadGrant{backupID: id, expires: expires}
	return gen.AuthorizeBackupDownloadResult{DownloadToken: token, ExpiresAt: expires}, nil
}

func (s *server) simpleStreamBackup(w http.ResponseWriter, r *http.Request, id string) error {
	token := r.Header.Get("X-Backup-Download-Token")
	downloadMu.Lock()
	grant, ok := downloadAuth[token]
	delete(downloadAuth, token)
	downloadMu.Unlock()
	if !ok || grant.backupID != id || grant.expires.Before(time.Now()) {
		return platform.NewError(platform.CodeForbidden, false, "下载授权无效或已过期")
	}
	dir, err := s.backupDir(id)
	if err != nil {
		return err
	}
	f, err := os.Open(filepath.Join(dir, "backup.zip"))
	if err != nil {
		return platform.NewError(platform.CodeNotFound, false, "备份文件不存在")
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="backup-`+id+`.zip"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, f)
	return nil
}
