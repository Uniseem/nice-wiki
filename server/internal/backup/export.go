package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Snapshotter interface {
	Snapshot(context.Context, string) error
}
type SQLSnapshotter struct{ DB *sql.DB }

func (s SQLSnapshotter) Snapshot(ctx context.Context, target string) error {
	_, err := s.DB.ExecContext(ctx, "VACUUM INTO ?", target)
	return err
}
func addFile(writer *tar.Writer, path, name string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return errors.New("备份不支持链接或特殊文件")
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(name)
	header.ModTime = time.Unix(0, 0)
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(writer, input)
	return errors.Join(copyErr, input.Close())
}
func Export(ctx context.Context, root, target string, database Snapshotter, includeSecrets bool) (err error) {
	temporary, err := os.MkdirTemp(filepath.Dir(target), ".nw-backup-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(temporary)) }()
	snapshot := filepath.Join(temporary, "wiki.db")
	if err := database.Snapshot(ctx, snapshot); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".nw-archive-")
	if err != nil {
		return err
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		err = errors.Join(err, tarWriter.Close(), gzipWriter.Close(), file.Close())
		if err == nil {
			err = os.Rename(file.Name(), target)
		}
		if removeErr := os.Remove(file.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	if err := addFile(tarWriter, snapshot, "wiki.db"); err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		skip := path == target || path == file.Name() || path == temporary || parts[0] == "backups" || parts[0] == "cache" || parts[0] == "wiki.db" || parts[0] == "wiki.db-wal" || parts[0] == "wiki.db-shm" || parts[0] == ".nice-wiki.lock" || strings.HasPrefix(parts[0], ".restore-") || strings.HasPrefix(parts[0], ".nw-") || entry.Name() == "restore-pending.json" || strings.HasPrefix(entry.Name(), ".tmp-") || !includeSecrets && relative == filepath.Join("config", "secrets.json")
		// 只保留启动所需模型资料，运行缓存与临时目录不进入备份。
		if parts[0] == "cache" {
			skip = relative != "cache" && relative != filepath.Join("cache", "models.json")
		}
		if strings.HasPrefix(entry.Name(), ".nw-") {
			skip = true
		}
		if skip {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return addFile(tarWriter, path, relative)
	})
}
