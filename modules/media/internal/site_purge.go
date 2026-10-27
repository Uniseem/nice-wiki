package media

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/pgdb"
)

// SiteLifecycle 返回媒体库和保留期满时的文件清理：站点的媒体文件，以及没传完的上传分片。
func SiteLifecycle() (*sql.DB, func(context.Context, string) error, error) {
	store, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return nil, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体存储尚未装配")
	}
	purge := func(ctx context.Context, site string) error {
		rows, err := store.db.QueryContext(pgdb.WithSite(ctx, site), `SELECT upload_id FROM upload_sessions`)
		if err != nil {
			return mapBlobStoreError(err, "读取站点上传记录失败")
		}
		var uploads []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return mapBlobStoreError(err, "读取站点上传记录失败")
			}
			uploads = append(uploads, id)
		}
		rows.Close()
		paths := []string{filepath.Join(store.root, "blobs", site)}
		for _, id := range uploads {
			paths = append(paths, store.uploadPath(id))
			staged, _ := filepath.Glob(filepath.Join(store.root, "staging", id+"-*.part"))
			paths = append(paths, staged...)
		}
		for _, path := range paths {
			if err := os.RemoveAll(path); err != nil {
				return mapBlobStoreError(err, "删除站点媒体文件失败")
			}
		}
		return purgeObjects(ctx, site)
	}
	return store.db, purge, nil
}
