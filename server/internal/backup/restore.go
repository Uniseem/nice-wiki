package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Restore(ctx context.Context, archive, target string) (result error) {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		_ = file.Close()
		return err
	}
	reader := tar.NewReader(gzipReader)
	temporary, err := os.MkdirTemp(filepath.Dir(target), ".nw-restore-")
	if err != nil {
		return errors.Join(err, gzipReader.Close(), file.Close())
	}
	defer func() {
		result = errors.Join(result, gzipReader.Close(), file.Close())
		if result != nil {
			result = errors.Join(result, os.RemoveAll(temporary))
		}
	}()
	cleanup := func(e error) error { return e }
	total := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return cleanup(err)
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return cleanup(err)
		}
		clean := filepath.Clean(header.Name)
		if clean == "." || clean == ".." || strings.Contains(header.Name, "\\") || strings.Contains(header.Name, ":") || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return cleanup(errors.New("备份包含越界路径"))
		}
		top := strings.Split(filepath.ToSlash(clean), "/")[0]
		if strings.HasPrefix(top, ".restore-") || strings.HasPrefix(top, ".nw-") || top == ".nice-wiki.lock" || top == "wiki.db-wal" || top == "wiki.db-shm" || clean == filepath.Join("config", "restore-pending.json") {
			return cleanup(errors.New("备份包含运行状态文件"))
		}
		if header.Typeflag != tar.TypeDir && header.Typeflag != tar.TypeReg {
			return cleanup(errors.New("备份包含链接或特殊文件"))
		}
		path := filepath.Join(temporary, clean)
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0700); err != nil {
				return cleanup(err)
			}
			continue
		}
		total += header.Size
		if header.Size < 0 || header.Size > 1<<30 || total > 8<<30 {
			return cleanup(errors.New("备份文件大小无效"))
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return cleanup(err)
		}
		output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return cleanup(err)
		}
		_, copyErr := io.CopyN(output, reader, header.Size)
		if err := errors.Join(copyErr, output.Close()); err != nil {
			return cleanup(err)
		}
	}
	if _, err := io.Copy(io.Discard, gzipReader); err != nil {
		return cleanup(err)
	}
	if _, err := os.Stat(target); err == nil {
		return cleanup(errors.New("恢复目标已存在"))
	} else if !errors.Is(err, os.ErrNotExist) {
		return cleanup(err)
	}
	if err := os.Rename(temporary, target); err != nil {
		return cleanup(err)
	}
	return nil
}
