package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"nice-wiki/libs/platform"
)

func libraryName(sha, original, mediaType string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(original), "."))
	valid := len(ext) > 0 && len(ext) <= 12
	for _, c := range ext {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			valid = false
		}
	}
	if !valid {
		ext = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "image/gif": "gif", "image/svg+xml": "svg", "image/avif": "avif", "video/mp4": "mp4", "video/webm": "webm", "application/pdf": "pdf", "application/json": "json", "text/plain": "txt"}[mediaType]
		if ext == "" {
			ext = "bin"
		}
	}
	return sha + "." + ext
}

func (s *localBlobStore) libraryFileName(site, sha string) string {
	if name, ok := s.blobNames.Load(site + "/" + sha); ok {
		return name.(string)
	}
	// 无元数据的PREPARED残留仍可能使用旧路径，由既有journal恢复隔离。
	return sha
}

// migrateLibraryNames 在接收请求前迁移旧文件名，不改资源ID、引用或字节。
// 同哈希共用一个文件，保留首次上传的安全后缀；重启时从元数据重建名字映射。
func (s *localBlobStore) migrateLibraryNames(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT site_id,sha256,json_extract(asset_json,'$.originalName'),media_type FROM (SELECT * FROM assets ORDER BY json_extract(asset_json,'$.createdAt'),asset_id) UNION ALL SELECT site_id,json_extract(session_json,'$.expectedSha256'),json_extract(session_json,'$.originalName'),COALESCE(json_extract(session_json,'$.mediaType'),'application/octet-stream') FROM upload_sessions WHERE state IN ('receiving','validating','completed')`)
	if err != nil {
		return mapMediaDBError(err, "读取资源文件名迁移失败")
	}
	type file struct{ site, sha, name, kind string }
	files := []file{}
	for rows.Next() {
		var f file
		if err = rows.Scan(&f.site, &f.sha, &f.name, &f.kind); err != nil {
			break
		}
		files = append(files, f)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return mapMediaDBError(err, "读取资源文件名失败")
	}
	for _, f := range files {
		if !platform.ValidIdentifier(f.site) || !platform.ValidHash(f.sha) {
			return platform.NewError(platform.CodeIntegrityError, false, "资源路径标识不合法")
		}
		name := libraryName(f.sha, f.name, f.kind)
		matches, e := filepath.Glob(filepath.Join(s.root, "blobs", f.site, f.sha[:2], f.sha+".*"))
		if e != nil {
			return mapMediaDBError(e, "读取已有文件名失败")
		}
		if len(matches) > 1 {
			return platform.NewError(platform.CodeIntegrityError, false, "同哈希存在多个物理文件，须先核对")
		}
		if len(matches) == 1 {
			name = filepath.Base(matches[0])
		}
		if _, loaded := s.blobNames.LoadOrStore(f.site+"/"+f.sha, name); loaded {
			continue
		}
		directory := filepath.Join(s.root, "blobs", f.site, f.sha[:2])
		old := filepath.Join(directory, f.sha)
		target := filepath.Join(directory, name)
		oldInfo, e := os.Lstat(old)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return mapMediaDBError(e, "读取旧资源文件失败")
		}
		if !oldInfo.Mode().IsRegular() {
			return platform.NewError(platform.CodeIntegrityError, false, "旧资源不是普通文件")
		}
		if _, e = os.Lstat(target); e == nil {
			return platform.NewError(platform.CodeIntegrityError, false, "资源新旧文件名同时存在，须先核对")
		} else if !errors.Is(e, os.ErrNotExist) {
			return mapMediaDBError(e, "读取目标资源文件失败")
		}
		if e = os.Rename(old, target); e != nil {
			return mapMediaDBError(e, "迁移资源文件名失败")
		}
		if e = syncDirectory(directory); e != nil {
			return e
		}
	}
	return nil
}
