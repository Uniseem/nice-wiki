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
	info, err := os.Stat(path)
	if err != nil {
		return err
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
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	defer func() { err = errors.Join(err, tarWriter.Close(), gzipWriter.Close(), file.Close()) }()
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
		if relative == "." || strings.HasPrefix(relative, "backups") || relative == "wiki.db" || !includeSecrets && relative == filepath.Join("config", "secrets.json") {
			if entry.IsDir() && relative != "." {
				return filepath.SkipDir
			}
			return nil
		}
		return addFile(tarWriter, path, relative)
	})
}
