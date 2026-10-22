package transport

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/data-protection/internal/gen"
)

// 简易在线备份：把配置里列出的各服务数据目录打成一个 tar.gz。
// 先在一轮里给全部服务的 SQLite 库各取一份 VACUUM INTO 快照，再复制其余文件：各服务都是先写不可变文件再提交 SQL，
// 所以快照里引用到的正文、资源、译文文件一定在归档里。不暂停写入：各库快照之间相隔很短但不是同一时点，
// 跨服务的事件可能有一条落在缝里；要严格一致用 deploy/backup.sh 停机备份。不含密钥目录。
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
	if includeSecrets {
		if !slices.Contains(op.Actor.Grants, "superadmin") {
			return gen.CreateBackupResult{}, platform.NewError(platform.CodeForbidden, false, "只有超级管理员可以备份密钥")
		}
		if s.app.config.BackupSecretsDir == "" {
			return gen.CreateBackupResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "未配置密钥目录，无法包含密钥")
		}
	}
	id, err := s.createBackup(ctx, includeSecrets)
	if err != nil {
		return gen.CreateBackupResult{}, err
	}
	return gen.CreateBackupResult{BackupId: id, Job: gen.AcceptedJob{JobId: platform.NewIdentifier(), OperationId: op.OperationID, State: "succeeded"}}, nil
}

func (s *server) createBackup(ctx context.Context, includeSecrets bool) (string, error) {
	if len(s.app.config.BackupSources) == 0 {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "未配置备份来源目录")
	}
	backupMu.Lock()
	defer backupMu.Unlock()
	id := platform.NewIdentifier()
	dir := filepath.Join(s.backupRoot(), id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", platform.NewError(platform.CodeInternal, true, "无法创建备份目录")
	}
	sources := s.app.config.BackupSources
	secretsDir := ""
	if includeSecrets {
		secretsDir = s.app.config.BackupSecretsDir
	}
	manifest, err := writeBackupArchive(ctx, filepath.Join(dir, "backup.tar.gz"), sources, secretsDir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	manifest.BackupID = id
	manifest.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	manifest.Format = "nice-wiki-simple-tar/1"
	manifest.IncludeSecrets = includeSecrets
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", platform.NewError(platform.CodeInternal, true, "无法写入备份清单")
	}
	return id, nil
}

func writeBackupArchive(ctx context.Context, target string, sources map[string]string, secretsDir string) (backupManifest, error) {
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return backupManifest{}, platform.NewError(platform.CodeInternal, true, "无法创建备份文件")
	}
	defer out.Close()
	hash := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(out, hash))
	tw := tar.NewWriter(gz)
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
	// 第一轮：只快照数据库，尽量缩短各库之间的时间差。
	snapshots := map[string]string{}
	for _, slug := range slugs {
		root := sources[slug]
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() || !entry.Type().IsRegular() || !strings.HasSuffix(path, ".db") {
				return nil
			}
			snap := filepath.Join(scratch, slug+"-"+hex.EncodeToString(randomBytes(6))+".db")
			if err := vacuumInto(ctx, path, snap); err != nil {
				return err
			}
			snapshots[path] = snap
			return nil
		})
		if err != nil {
			return backupManifest{}, platform.NewError(platform.CodeInternal, true, "快照服务数据库失败")
		}
	}
	// 第二轮：数据库用快照，其余文件原样复制；复制时已经消失的临时文件跳过。
	var manifest backupManifest
	for _, slug := range slugs {
		domain := backupDomain{Slug: slug}
		root := sources[slug]
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if errors.Is(walkErr, fs.ErrNotExist) {
					return nil
				}
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
				snap, found := snapshots[path]
				if !found {
					return nil
				}
				source = snap
			}
			size, err := addTarFile(tw, slug+"/"+filepath.ToSlash(rel), source)
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
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
	if secretsDir != "" {
		domain := backupDomain{Slug: "secrets"}
		err := filepath.WalkDir(secretsDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || !entry.Type().IsRegular() {
				return walkErr
			}
			rel, _ := filepath.Rel(secretsDir, path)
			size, err := addTarFile(tw, "secrets/"+filepath.ToSlash(rel), path)
			domain.Files++
			domain.Bytes += size
			return err
		})
		if err != nil {
			return backupManifest{}, platform.NewError(platform.CodeInternal, true, "打包密钥失败")
		}
		manifest.Domains = append(manifest.Domains, domain)
	}
	if err := tw.Close(); err != nil {
		return backupManifest{}, platform.NewError(platform.CodeInternal, true, "备份归档收尾失败")
	}
	if err := gz.Close(); err != nil {
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

// vacuumInto 从只读挂载的来源库取一致快照。服务在运行时库旁有 -wal/-shm，只读打开即可读到 WAL 里的最新提交；
// 服务已停、-wal 不在时只读目录里建不了 -shm，此时库文件就是全部内容，按 immutable 打开。
func vacuumInto(ctx context.Context, path, target string) error {
	options := "?mode=ro&_pragma=busy_timeout(5000)"
	if _, err := os.Stat(path + "-wal"); errors.Is(err, fs.ErrNotExist) {
		options += "&immutable=1"
	}
	db, err := sql.Open("sqlite", "file:"+path+options)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, "VACUUM INTO ?", target)
	return err
}

func addTarFile(tw *tar.Writer, name, source string) (int64, error) {
	in, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return 0, err
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: info.Size(), ModTime: info.ModTime(), Typeflag: tar.TypeReg}); err != nil {
		return 0, err
	}
	return io.Copy(tw, in)
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
	dir, err := s.backupDir(id)
	if err != nil {
		return err
	}
	// 口令先校验再消耗一次性令牌，口令不合格时可以直接重试。
	passphrase := r.Header.Get("X-Backup-Passphrase")
	if raw, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil {
		var manifest backupManifest
		if json.Unmarshal(raw, &manifest) == nil && manifest.IncludeSecrets && passphrase == "" {
			return platform.NewError(platform.CodeValidation, false, "含密钥的备份必须设置下载口令")
		}
	}
	if passphrase != "" && len(passphrase) < 12 {
		return platform.NewError(platform.CodeValidation, false, "下载口令至少 12 个字符")
	}
	token := r.Header.Get("X-Backup-Download-Token")
	downloadMu.Lock()
	grant, ok := downloadAuth[token]
	delete(downloadAuth, token)
	downloadMu.Unlock()
	if !ok || grant.backupID != id || grant.expires.Before(time.Now()) {
		return platform.NewError(platform.CodeForbidden, false, "下载授权无效或已过期")
	}
	f, err := os.Open(filepath.Join(dir, "backup.tar.gz"))
	if err != nil {
		return platform.NewError(platform.CodeNotFound, false, "备份文件不存在")
	}
	defer f.Close()
	w.Header().Set("Cache-Control", "no-store")
	if passphrase == "" {
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", `attachment; filename="backup-`+id+`.tar.gz"`)
		_, _ = io.Copy(w, f)
		return nil
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="backup-`+id+`.tar.gz.enc"`)
	_ = encryptStream(w, f, passphrase)
	return nil
}

// encryptStream 输出 OpenSSL 兼容格式（"Salted__" + 8 字节盐 + AES-256-CBC 密文，PKCS7 填充，PBKDF2-SHA256 10 万轮），
// 备份可以直接用 openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 解密。
func encryptStream(w io.Writer, in io.Reader, passphrase string) error {
	salt := randomBytes(8)
	derived, err := pbkdf2.Key(sha256.New, passphrase, salt, 100000, 48)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(derived[:32])
	if err != nil {
		return err
	}
	mode := cipher.NewCBCEncrypter(block, derived[32:])
	if _, err := w.Write(append([]byte("Salted__"), salt...)); err != nil {
		return err
	}
	buf := make([]byte, 64<<10)
	var pending []byte
	for {
		n, readErr := in.Read(buf)
		pending = append(pending, buf[:n]...)
		if full := len(pending) / aes.BlockSize * aes.BlockSize; full > 0 {
			out := make([]byte, full)
			mode.CryptBlocks(out, pending[:full])
			if _, err := w.Write(out); err != nil {
				return err
			}
			pending = append(pending[:0], pending[full:]...)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	pad := aes.BlockSize - len(pending)
	last := append(pending, bytes.Repeat([]byte{byte(pad)}, pad)...)
	out := make([]byte, len(last))
	mode.CryptBlocks(out, last)
	_, err = w.Write(out)
	return err
}

// 每日本地备份：配置存成 simple-backups/schedule.json，后台每分钟检查一次，
// 当天（所选时区）03:00 之后还没有新备份就创建一份，并按 keepCount 删除最旧的。
// keepCount 为 0 表示关闭：删掉配置文件，已有备份不动。
type backupSchedule struct {
	ScheduleID string `json:"scheduleId"`
	KeepCount  int    `json:"keepCount"`
	Timezone   string `json:"timezone"`
	Revision   int64  `json:"revision"`
}

func (s *server) readSchedule() (backupSchedule, bool) {
	raw, err := os.ReadFile(filepath.Join(s.backupRoot(), "schedule.json"))
	if err != nil {
		return backupSchedule{}, false
	}
	var schedule backupSchedule
	if json.Unmarshal(raw, &schedule) != nil {
		return backupSchedule{}, false
	}
	return schedule, true
}

func (s *server) simpleSchedule(op platform.OperationContext, body gen.ScheduleLocalBackupRequestBody) (gen.ScheduleLocalBackupResult, error) {
	if op.Actor.Kind != "admin" {
		return gen.ScheduleLocalBackupResult{}, platform.NewError(platform.CodeForbidden, false, "只有后台管理员可以设置自动备份")
	}
	if _, err := time.LoadLocation(body.Timezone); err != nil {
		return gen.ScheduleLocalBackupResult{}, platform.NewError(platform.CodeValidation, false, "时区不合法")
	}
	schedule, ok := s.readSchedule()
	if !ok {
		schedule.ScheduleID = platform.NewIdentifier()
	}
	if body.KeepCount == 0 {
		if err := os.Remove(filepath.Join(s.backupRoot(), "schedule.json")); err != nil && !os.IsNotExist(err) {
			return gen.ScheduleLocalBackupResult{}, platform.NewError(platform.CodeInternal, true, "无法关闭自动备份")
		}
		return gen.ScheduleLocalBackupResult{ScheduleId: schedule.ScheduleID}, nil
	}
	schedule.KeepCount, schedule.Timezone, schedule.Revision = body.KeepCount, body.Timezone, schedule.Revision+1
	raw, _ := json.Marshal(schedule)
	if err := os.MkdirAll(s.backupRoot(), 0o700); err != nil {
		return gen.ScheduleLocalBackupResult{}, platform.NewError(platform.CodeInternal, true, "无法创建备份目录")
	}
	if err := os.WriteFile(filepath.Join(s.backupRoot(), "schedule.json"), raw, 0o600); err != nil {
		return gen.ScheduleLocalBackupResult{}, platform.NewError(platform.CodeInternal, true, "无法保存自动备份设置")
	}
	return gen.ScheduleLocalBackupResult{ScheduleId: schedule.ScheduleID}, nil
}

func (s *server) runSchedule(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.scheduledBackupOnce(ctx)
		}
	}
}

func (s *server) scheduledBackupOnce(ctx context.Context) {
	schedule, ok := s.readSchedule()
	if !ok || len(s.app.config.BackupSources) == 0 {
		return
	}
	loc, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return
	}
	now := time.Now().In(loc)
	due := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, loc)
	if now.Before(due) {
		return
	}
	list, err := s.simpleListBackups()
	if err == nil && len(list.BackupIds) > 0 {
		if info, err := os.Stat(filepath.Join(s.backupRoot(), list.BackupIds[0], "manifest.json")); err == nil && info.ModTime().After(due) {
			return
		}
	}
	if _, err := s.createBackup(ctx, false); err != nil {
		return
	}
	if list, err = s.simpleListBackups(); err == nil && len(list.BackupIds) > schedule.KeepCount {
		for _, id := range list.BackupIds[schedule.KeepCount:] {
			_ = os.RemoveAll(filepath.Join(s.backupRoot(), id))
		}
	}
}
