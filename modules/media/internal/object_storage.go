package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/objectstore"
	"nice-wiki/libs/platform/pgdb"
)

// 媒体文件的 S3 兼容存储：站主在平台设置里开启后，之后上传的资源记成 pending，由后台搬到存储桶（键 <前缀>/<站点>/<sha256>），
// 搬完改成 s3 并删掉本机副本；开启之前的文件留在本机。读取按资源记录的位置走本机或存储桶，删除和站点清除两边都清。
// 连接信息（含访问密钥）由 identity 保管，本域每分钟最多读一次。

type mediaStorageReader interface {
	ReadMediaStorage(ctx context.Context) (json.RawMessage, error)
}

var storageState struct {
	sync.Mutex
	reader  mediaStorageReader
	checked time.Time
	client  *objectstore.Client
}

// InstallMediaStorageReader 装配读取媒体存储设置的 identity 客户端。
func InstallMediaStorageReader(reader mediaStorageReader) {
	storageState.Lock()
	storageState.reader = reader
	storageState.Unlock()
}

// objectStorage 返回开启的对象存储；没开或读不到设置时为 nil（读不到时沿用上一次的结果）。
func objectStorage(ctx context.Context) *objectstore.Client {
	storageState.Lock()
	defer storageState.Unlock()
	if storageState.reader == nil || time.Since(storageState.checked) < time.Minute {
		return storageState.client
	}
	storageState.checked = time.Now()
	raw, err := storageState.reader.ReadMediaStorage(platformServiceContext(ctx))
	if err != nil {
		return storageState.client
	}
	var cfg struct {
		Enabled         bool   `json:"enabled"`
		Endpoint        string `json:"endpoint"`
		Bucket          string `json:"bucket"`
		Region          string `json:"region"`
		Prefix          string `json:"prefix"`
		AccessKeyID     string `json:"accessKeyId"`
		SecretAccessKey string `json:"secretAccessKey"`
	}
	if json.Unmarshal(raw, &cfg) != nil || !cfg.Enabled {
		storageState.client = nil
		return nil
	}
	client, err := objectstore.New(&http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		objectstore.Config{Endpoint: cfg.Endpoint, Bucket: cfg.Bucket, Region: cfg.Region, Prefix: cfg.Prefix, AccessKey: cfg.AccessKeyID, SecretKey: cfg.SecretAccessKey})
	if err != nil {
		storageState.client = nil
		return nil
	}
	storageState.client = client
	return client
}

// platformServiceContext 以本服务身份调平台级接口（不属于某个站点）。
func platformServiceContext(ctx context.Context) context.Context {
	id := platform.NewIdentifier()
	deadline := platform.Timestamp(time.Now().UTC().Add(10 * time.Second).Format(time.RFC3339Nano))
	op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: "media", Deadline: deadline,
		Actor: platform.AuthContext{Kind: "service", SubjectID: "media", SiteID: "system", ExpiresAt: deadline}}
	if s, ok := currentBlobStore().(*localBlobStore); ok {
		op.DataEpoch = s.openedEpoch
	}
	return platform.WithOperationContext(platform.WithPortalProof(context.WithoutCancel(ctx), ""), op)
}

func objectKey(client *objectstore.Client, site, sha string) string {
	return client.Key(site + "/" + sha)
}

// newAssetStorage 是新资源的存放位置：开了对象存储就等后台搬走。
func newAssetStorage(ctx context.Context) string {
	if objectStorage(ctx) != nil {
		return "pending"
	}
	return "local"
}

// blobSource 是资源字节的来源（本机文件或存储桶对象），按范围读。
type blobSource interface {
	Range(ctx context.Context, start, length int64) (io.ReadCloser, error)
	Close() error
}

type fileSource struct{ file *os.File }

func (f fileSource) Range(_ context.Context, start, length int64) (io.ReadCloser, error) {
	return io.NopCloser(io.NewSectionReader(f.file, start, length)), nil
}
func (f fileSource) Close() error { return f.file.Close() }

type objectSource struct {
	client *objectstore.Client
	key    string
}

func (o objectSource) Range(ctx context.Context, start, length int64) (io.ReadCloser, error) {
	return o.client.Get(ctx, o.key, start, length)
}
func (o objectSource) Close() error { return nil }

// openBlob 按资源的存放位置打开字节；本机文件刚被后台搬走时改从存储桶读。
func (s *localBlobStore) openBlob(ctx context.Context, site, sha, storage string) (blobSource, error) {
	client := objectStorage(ctx)
	if storage != "s3" {
		if file, err := os.Open(s.blobPath(site, sha)); err == nil {
			return fileSource{file}, nil
		}
	}
	if client == nil {
		if storage == "s3" {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "资源在对象存储里，但对象存储没有开启")
		}
		return nil, platform.NewError(platform.CodeIntegrityError, false, "资源文件缺失")
	}
	return objectSource{client: client, key: objectKey(client, site, sha)}, nil
}

// RunObjectOffload 是把待搬的资源搬到存储桶的后台循环（每 30 秒一批）。
func RunObjectOffload(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := offloadBatch(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "媒体文件搬到对象存储失败 err=%v\n", err)
			}
		}
	}
}

func offloadBatch(ctx context.Context) error {
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return nil
	}
	client := objectStorage(ctx)
	if client == nil {
		return nil
	}
	system := pgdb.WithSite(ctx, "")
	rows, err := s.db.QueryContext(system, `SELECT DISTINCT site_id,sha256 FROM assets WHERE storage='pending' AND state='ready' LIMIT 20`)
	if err != nil {
		return err
	}
	type blob struct{ site, sha string }
	var batch []blob
	for rows.Next() {
		var b blob
		if err := rows.Scan(&b.site, &b.sha); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, b)
	}
	rows.Close()
	for _, b := range batch {
		file, err := os.Open(s.blobPath(b.site, b.sha))
		if err != nil {
			return err
		}
		info, err := file.Stat()
		if err == nil {
			err = client.Put(ctx, objectKey(client, b.site, b.sha), file, info.Size())
		}
		file.Close()
		if err != nil {
			return err
		}
		// 先改记录再删本机副本：改完以后读取都走存储桶；删除和清除也和提交在同一把锁下，避免同时改同一个文件。
		s.mu.Lock()
		_, err = s.db.ExecContext(system, `UPDATE assets SET storage='s3' WHERE site_id=? AND sha256=? AND storage='pending'`, b.site, b.sha)
		if err == nil {
			err = os.Remove(s.blobPath(b.site, b.sha))
		}
		s.mu.Unlock()
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// purgeObjects 删掉存储桶里某站点的全部对象（站点保留期满清除时）。
func purgeObjects(ctx context.Context, site string) error {
	client := objectStorage(ctx)
	if client == nil {
		return nil
	}
	keys, err := client.List(ctx, objectKey(client, site, ""))
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := client.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}
