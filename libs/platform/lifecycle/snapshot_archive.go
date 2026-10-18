package lifecycle

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"nice-wiki/libs/platform"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SnapshotArchiveResult struct {
	Manifest     DomainSnapshotManifest
	ManifestHash string
	SHA256       string
	ByteSize     int64
}
type snapshotCountedReader struct {
	Reader io.Reader
	Count  int64
	Limit  int64
	Digest io.Writer
}

func (r *snapshotCountedReader) Read(p []byte) (int, error) {
	if r.Count > r.Limit {
		return 0, snapshotBudget("归档超过20GiB")
	}
	if int64(len(p)) > r.Limit-r.Count+1 {
		p = p[:r.Limit-r.Count+1]
	}
	n, err := r.Reader.Read(p)
	r.Count += int64(n)
	if n > 0 {
		_, _ = r.Digest.Write(p[:n])
	}
	if r.Count > r.Limit {
		return n, snapshotBudget("归档超过20GiB")
	}
	return n, err
}

type snapshotCountedWriter struct {
	Writer io.Writer
	Count  int64
	Limit  int64
	Digest io.Writer
}

func (w *snapshotCountedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.Limit-w.Count {
		return 0, snapshotBudget("归档超过20GiB")
	}
	n, err := w.Writer.Write(p)
	if n > 0 {
		_, _ = w.Digest.Write(p[:n])
		w.Count += int64(n)
	}
	return n, err
}

func validateSnapshotInventory(manifest DomainSnapshotManifest, validate func([]byte) error) ([]byte, error) {
	raw, err := platform.CanonicalJSON(manifest)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > MaxManifestBytes {
		return nil, snapshotBudget("域清单超过16MiB")
	}
	if validate == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "域清单严格校验未装配")
	}
	if err = validate(raw); err != nil {
		return nil, integrity("域清单不符合公共V1 schema")
	}
	if manifest.Format != "nice-wiki.domain-snapshot" || manifest.SchemaVersion != 1 {
		return nil, snapshotUnsupported("域清单版本不支持")
	}
	if len(manifest.Files)+1 > MaxArchiveFiles {
		return nil, snapshotBudget("归档总文件数超过100000")
	}
	files := map[string]DomainSnapshotFile{}
	for _, file := range manifest.Files {
		if !safeSnapshotRelative(file.Path) || !platform.ValidHash(file.SHA256) || file.ByteSize < 0 || file.ByteSize > MaxArchiveBytes {
			return nil, integrity("域文件清单不合法")
		}
		if _, ok := files[file.Path]; ok {
			return nil, integrity("域文件路径重复")
		}
		files[file.Path] = file
		header := &tar.Header{Name: "files/" + file.Path, Typeflag: tar.TypeReg, Size: file.ByteSize, Mode: 0600, Format: tar.FormatUSTAR}
		if err = tar.NewWriter(io.Discard).WriteHeader(header); err != nil {
			return nil, snapshotUnsupported("文件路径或长度不能由标准USTAR编码")
		}
	}
	objects := map[string]string{}
	for _, object := range manifest.Objects {
		file, ok := files[object.Path]
		if !ok || object.Reference.Owner != manifest.Owner || object.Reference.SHA256 != file.SHA256 || object.Reference.ByteSize != file.ByteSize {
			return nil, integrity("权威对象引用不对应真实文件")
		}
		identity, _ := platform.CanonicalJSON(struct {
			ID      string
			Version int64
		}{object.Reference.ObjectID, object.Reference.Version})
		if _, ok = objects[string(identity)]; ok {
			return nil, integrity("权威对象身份重复")
		}
		objects[string(identity)] = object.Path
	}
	if len(manifest.Publications) > 0 && manifest.Owner != "content" {
		return nil, integrity("非content域不能声明发布修订权威")
	}
	publications := map[string]bool{}
	for _, publication := range manifest.Publications {
		raw, _ := platform.CanonicalJSON([]string{publication.DocumentID, publication.PublishedRevisionID})
		if publications[string(raw)] {
			return nil, integrity("发布修订权威重复")
		}
		publications[string(raw)] = true
	}
	for _, ref := range manifest.References {
		if ref.Kind == "object" {
			if ref.Object == nil || ref.Publication != nil {
				return nil, integrity("对象引用union不合法")
			}
		} else if ref.Kind == "publication" {
			if ref.Publication == nil || ref.Object != nil {
				return nil, integrity("发布引用union不合法")
			}
		} else {
			return nil, integrity("引用种类未知")
		}
	}
	return raw, nil
}
func verifySnapshotInventoryTree(ctx context.Context, root string, files []DomainSnapshotFile, checkHash bool) error {
	if err := ValidatePrivatePath(root, "directory", false); err != nil {
		return err
	}
	known := map[string]DomainSnapshotFile{}
	for _, file := range files {
		known[file.Path] = file
	}
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return snapshotIOError(walkErr)
		}
		if err := ctx.Err(); err != nil {
			return snapshotIOError(err)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return integrity("快照文件树含符号链接")
		}
		if entry.IsDir() {
			return ValidatePrivatePath(path, "directory", false)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return snapshotIOError(err)
		}
		relative = filepath.ToSlash(relative)
		file, ok := known[relative]
		if !ok {
			return integrity("快照文件树含未声明文件")
		}
		if checkHash {
			if err = verifySnapshotFile(ctx, path, file); err != nil {
				return err
			}
		} else {
			if err = ValidatePrivatePath(path, "regular-file", false); err != nil {
				return err
			}
			info, statErr := os.Stat(path)
			if statErr != nil {
				return snapshotIOError(statErr)
			}
			if info.Size() != file.ByteSize {
				return integrity("快照捕获文件长度不符")
			}
		}
		seen[relative] = true
		return nil
	})
	if err != nil {
		return err
	}
	if len(seen) != len(known) {
		return integrity("快照文件树缺少已声明文件")
	}
	return nil
}
func snapshotArchiveByteSize(manifestBytes int64, files []DomainSnapshotFile) (int64, error) {
	size := int64(512) + (manifestBytes+511)/512*512 + 1024
	for _, file := range files {
		add := int64(512) + (file.ByteSize+511)/512*512
		if add > MaxArchiveBytes-size {
			return 0, snapshotBudget("完整tar超过20GiB")
		}
		size += add
	}
	return size, nil
}

// BuildArchive只读取owner已捕获的完整workspace/files，schema/路径/普通文件/hash逐项核完后才安装真实tar。
func BuildSnapshotArchive(ctx context.Context, workspace, output string, manifest DomainSnapshotManifest, validate func([]byte) error) (SnapshotArchiveResult, error) {
	raw, err := validateSnapshotInventory(manifest, validate)
	if err != nil {
		return SnapshotArchiveResult{}, err
	}
	root := filepath.Join(workspace, "files")
	if err = verifySnapshotInventoryTree(ctx, root, manifest.Files, false); err != nil {
		return SnapshotArchiveResult{}, err
	}
	expectedSize, err := snapshotArchiveByteSize(int64(len(raw)), manifest.Files)
	if err != nil {
		return SnapshotArchiveResult{}, err
	}
	if err = CheckSnapshotDiskBudget(ctx, workspace, expectedSize); err != nil {
		return SnapshotArchiveResult{}, err
	}
	if err = ValidatePrivatePath(output, "regular-file", true); err != nil {
		return SnapshotArchiveResult{}, err
	}
	f, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return SnapshotArchiveResult{}, snapshotIOError(err)
	}
	success := false
	defer func() {
		_ = f.Close()
		if !success {
			_ = os.Remove(output)
		}
	}()
	digest := sha256.New()
	writer := &snapshotCountedWriter{Writer: f, Limit: MaxArchiveBytes, Digest: digest}
	archive := tar.NewWriter(writer)
	if err = archive.WriteHeader(&tar.Header{Name: "manifest.json", Size: int64(len(raw)), Mode: 0600, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}); err != nil {
		return SnapshotArchiveResult{}, snapshotUnsupported("清单不能由标准USTAR编码")
	}
	if _, err = archive.Write(raw); err != nil {
		return SnapshotArchiveResult{}, snapshotIOError(err)
	}
	files := append([]DomainSnapshotFile(nil), manifest.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	for _, file := range files {
		if err = ctx.Err(); err != nil {
			return SnapshotArchiveResult{}, snapshotIOError(err)
		}
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err = ValidatePrivatePath(path, "regular-file", false); err != nil {
			return SnapshotArchiveResult{}, err
		}
		source, openErr := os.Open(path)
		if openErr != nil {
			return SnapshotArchiveResult{}, snapshotIOError(openErr)
		}
		if err = archive.WriteHeader(&tar.Header{Name: "files/" + file.Path, Size: file.ByteSize, Mode: 0600, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}); err != nil {
			_ = source.Close()
			return SnapshotArchiveResult{}, snapshotUnsupported("文件不能由标准USTAR编码")
		}
		err = copySnapshotExact(ctx, archive, source, file.ByteSize, file.SHA256)
		var extra [1]byte
		if err == nil {
			if n, readErr := source.Read(extra[:]); n != 0 || !errors.Is(readErr, io.EOF) {
				err = integrity("快照源出现尾随数据")
			}
		}
		_ = source.Close()
		if err != nil {
			return SnapshotArchiveResult{}, err
		}
	}
	if err = archive.Close(); err != nil {
		return SnapshotArchiveResult{}, snapshotIOError(err)
	}
	if writer.Count != expectedSize {
		return SnapshotArchiveResult{}, integrity("tar实际长度与计划不符")
	}
	if err = f.Sync(); err != nil {
		return SnapshotArchiveResult{}, snapshotIOError(err)
	}
	if err = f.Close(); err != nil {
		return SnapshotArchiveResult{}, snapshotIOError(err)
	}
	success = true
	return SnapshotArchiveResult{Manifest: manifest, ManifestHash: platform.HashBytes(raw), SHA256: hex.EncodeToString(digest.Sum(nil)), ByteSize: writer.Count}, nil
}

// ExtractArchive完整认证前仅写唯一隔离目录。允许的tar版本仅USTAR，禁PAX/GNU扩展、链接及未声明项。
func ExtractSnapshotArchive(ctx context.Context, source io.Reader, root string, expected platform.ObjectRef, validate func([]byte) error) (SnapshotArchiveResult, error) {
	if expected.ByteSize < 1024 || expected.ByteSize > MaxArchiveBytes || !platform.ValidHash(expected.SHA256) {
		return SnapshotArchiveResult{}, integrity("快照归档引用长度或hash不合法")
	}
	if err := ValidatePrivatePath(root, "directory", false); err != nil {
		return SnapshotArchiveResult{}, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return SnapshotArchiveResult{}, snapshotIOError(err)
	}
	if len(entries) != 0 {
		return SnapshotArchiveResult{}, integrity("解包隔离目录必须为空")
	}
	if err = CheckSnapshotDiskBudget(ctx, root, expected.ByteSize); err != nil {
		return SnapshotArchiveResult{}, err
	}
	digest := sha256.New()
	reader := &snapshotCountedReader{Reader: source, Limit: expected.ByteSize, Digest: digest}
	archive := tar.NewReader(reader)
	header, err := archive.Next()
	if err != nil {
		return SnapshotArchiveResult{}, integrity("域归档缺首清单")
	}
	if header.Name != "manifest.json" || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA) || header.Format != tar.FormatUSTAR || len(header.PAXRecords) != 0 || header.Linkname != "" || header.Size <= 0 || header.Size > MaxManifestBytes {
		return SnapshotArchiveResult{}, integrity("域归档首项不是V1普通清单")
	}
	raw, err := io.ReadAll(io.LimitReader(archive, MaxManifestBytes+1))
	if err != nil {
		return SnapshotArchiveResult{}, snapshotIOError(err)
	}
	if int64(len(raw)) != header.Size {
		return SnapshotArchiveResult{}, integrity("域清单长度不一致")
	}
	var manifest DomainSnapshotManifest
	if err = decodeSnapshotCanonical(raw, &manifest); err != nil {
		return SnapshotArchiveResult{}, err
	}
	if _, err = validateSnapshotInventory(manifest, validate); err != nil {
		return SnapshotArchiveResult{}, err
	}
	plannedBytes, planErr := snapshotArchiveByteSize(int64(len(raw)), manifest.Files)
	if planErr != nil {
		return SnapshotArchiveResult{}, planErr
	}
	if plannedBytes != expected.ByteSize {
		return SnapshotArchiveResult{}, integrity("USTAR长度缺少规范终止块或含额外内容")
	}
	declared := map[string]DomainSnapshotFile{}
	for _, file := range manifest.Files {
		declared["files/"+file.Path] = file
	}
	seen := map[string]bool{}
	for {
		header, err = archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return SnapshotArchiveResult{}, platform.NormalizeIOError(err, platform.CodeIntegrityError, false, "域归档header损坏或读取失败")
		}
		if (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA) || header.Format != tar.FormatUSTAR || len(header.PAXRecords) != 0 || header.Linkname != "" || !strings.HasPrefix(header.Name, "files/") {
			return SnapshotArchiveResult{}, integrity("域归档含链接、扩展或未知entry")
		}
		file, ok := declared[header.Name]
		if !ok || seen[header.Name] || header.Size != file.ByteSize {
			return SnapshotArchiveResult{}, integrity("域归档entry未声明、重复或长度不符")
		}
		seen[header.Name] = true
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err = snapshotDirectory(filepath.Dir(path), root); err != nil {
			return SnapshotArchiveResult{}, err
		}
		if err = ValidatePrivatePath(path, "regular-file", true); err != nil {
			return SnapshotArchiveResult{}, err
		}
		output, openErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if openErr != nil {
			return SnapshotArchiveResult{}, snapshotIOError(openErr)
		}
		err = copySnapshotExact(ctx, output, archive, file.ByteSize, file.SHA256)
		if err == nil {
			err = snapshotIOError(output.Sync())
		}
		_ = output.Close()
		if err != nil {
			return SnapshotArchiveResult{}, err
		}
		if err = syncSnapshotDirectories(filepath.Dir(path), root); err != nil {
			return SnapshotArchiveResult{}, err
		}
	}
	if len(seen) != len(declared) {
		return SnapshotArchiveResult{}, integrity("域归档缺少声明文件")
	}
	var extra [1]byte
	n, endErr := reader.Read(extra[:])
	if n != 0 || !errors.Is(endErr, io.EOF) {
		return SnapshotArchiveResult{}, integrity("tar终止块后存在尾随数据或未结束")
	}
	if reader.Count != expected.ByteSize || hex.EncodeToString(digest.Sum(nil)) != expected.SHA256 {
		return SnapshotArchiveResult{}, integrity("完整归档hash或长度与引用不符")
	}
	return SnapshotArchiveResult{Manifest: manifest, ManifestHash: platform.HashBytes(raw), SHA256: expected.SHA256, ByteSize: reader.Count}, nil
}
