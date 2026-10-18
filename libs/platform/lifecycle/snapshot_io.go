package lifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"nice-wiki/libs/platform"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func snapshotIOError(err error) error {
	if err == nil {
		return nil
	}
	return platform.NormalizeIOError(err, platform.CodeInternal, false, "快照私有IO失败")
}
func snapshotBudget(message string) error {
	return platform.NewError(platform.CodeBudgetExceeded, false, "%s", message)
}
func snapshotUnsupported(message string) error {
	return platform.NewError(platform.CodeUnsupportedSchema, false, "%s", message)
}
func decodeSnapshotCanonical(raw []byte, target any) error {
	if !json.Valid(raw) {
		return integrity("快照JSON不可读")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return integrity("快照JSON字段不符合版本")
	}
	encoded, err := platform.CanonicalJSON(target)
	if err != nil || !bytes.Equal(raw, encoded) {
		return integrity("快照JSON不是唯一canonical编码")
	}
	return nil
}
func readSnapshotJSON(path string, limit int64, target any) error {
	if err := ValidatePrivatePath(path, "regular-file", false); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return snapshotIOError(err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return snapshotIOError(err)
	}
	if int64(len(raw)) > limit {
		return snapshotBudget("快照控制记录超过预算")
	}
	return decodeSnapshotCanonical(raw, target)
}
func syncSnapshotDirectories(path, root string) error {
	for {
		f, err := os.Open(path)
		if err != nil {
			return snapshotIOError(err)
		}
		err = f.Sync()
		_ = f.Close()
		if err != nil {
			return snapshotIOError(err)
		}
		if path == root {
			return nil
		}
		next := filepath.Dir(path)
		if next == path {
			return integrity("目录同步超出私有根")
		}
		path = next
	}
}
func snapshotDirectory(path, root string) error {
	if err := ValidatePrivatePath(path, "directory", true); err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return snapshotIOError(err)
	}
	if err := ValidatePrivatePath(path, "directory", false); err != nil {
		return err
	}
	return syncSnapshotDirectories(path, root)
}
func writeSnapshotJSON(path, root string, value any) error {
	raw, err := platform.CanonicalJSON(value)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err = snapshotDirectory(dir, root); err != nil {
		return err
	}
	if err = ValidatePrivatePath(path, "regular-file", true); err != nil {
		return err
	}
	temporary := filepath.Join(dir, ".record-"+platform.NewIdentifier()+".tmp")
	f, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return snapshotIOError(err)
	}
	installed := false
	defer func() {
		_ = f.Close()
		if !installed {
			_ = os.Remove(temporary)
		}
	}()
	if _, err = f.Write(raw); err != nil {
		return snapshotIOError(err)
	}
	if err = f.Sync(); err != nil {
		return snapshotIOError(err)
	}
	if err = f.Close(); err != nil {
		return snapshotIOError(err)
	}
	if err = os.Rename(temporary, path); err != nil {
		return snapshotIOError(err)
	}
	installed = true
	return syncSnapshotDirectories(dir, root)
}
func safeSnapshotRelative(path string) bool {
	if path == "" || len(path) > 4096 || strings.ContainsAny(path, "\\\x00:") || strings.HasPrefix(path, "/") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// CheckDiskBudget只读真实文件系统剩余块；调用方必须先有真实inventory或明确有界目标，不能拿未知零字节通过。
// ROOT若支持其他OS，需把Statfs封装拆为平台文件；不以未支持平台返回无限容量。
func CheckSnapshotDiskBudget(ctx context.Context, root string, required int64) error {
	if err := ctx.Err(); err != nil {
		return snapshotIOError(err)
	}
	if required < 0 {
		return snapshotBudget("快照所需空间尚未确定")
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(root, &stat); err != nil {
		return snapshotIOError(err)
	}
	block := int64(stat.Bsize)
	if block <= 0 {
		return integrity("文件系统空间单位未知")
	}
	blocks := uint64(required / block)
	if required%block != 0 {
		blocks++
	}
	if stat.Bavail < blocks {
		return snapshotBudget("快照隔离空间不足")
	}
	return nil
}
func copySnapshotExact(ctx context.Context, dst io.Writer, src io.Reader, size int64, expectedHash string) error {
	if size < 0 || size > MaxArchiveBytes || !platform.ValidHash(expectedHash) {
		return integrity("快照文件长度或hash不合法")
	}
	digest := sha256.New()
	buffer := make([]byte, 128<<10)
	remaining := size
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return snapshotIOError(err)
		}
		length := int64(len(buffer))
		if length > remaining {
			length = remaining
		}
		part := buffer[:int(length)]
		if _, err := io.ReadFull(src, part); err != nil {
			return platform.NormalizeIOError(err, platform.CodeIntegrityError, false, "快照文件提前结束或读取失败")
		}
		_, _ = digest.Write(part)
		written, err := dst.Write(part)
		if err != nil {
			return snapshotIOError(err)
		}
		if written != len(part) {
			return snapshotIOError(io.ErrShortWrite)
		}
		remaining -= length
	}
	if hex.EncodeToString(digest.Sum(nil)) != expectedHash {
		return integrity("快照文件hash不一致")
	}
	return nil
}
func verifySnapshotFile(ctx context.Context, path string, file DomainSnapshotFile) error {
	if err := ValidatePrivatePath(path, "regular-file", false); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return snapshotIOError(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return snapshotIOError(err)
	}
	if info.Size() != file.ByteSize {
		return integrity("快照文件实际长度不一致")
	}
	if err = copySnapshotExact(ctx, io.Discard, f, file.ByteSize, file.SHA256); err != nil {
		return err
	}
	var extra [1]byte
	if n, readErr := f.Read(extra[:]); n != 0 || !errors.Is(readErr, io.EOF) {
		return integrity("快照文件有尾随数据")
	}
	return nil
}
