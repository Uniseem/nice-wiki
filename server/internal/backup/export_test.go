package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"nice-wiki/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func archiveEntries(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := gz.Close(); err != nil {
			t.Error(err)
		}
	}()
	reader := tar.NewReader(gz)
	entries := map[string]string{}
	for {
		h, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		entries[h.Name] = string(data)
	}
	return entries
}
func TestExportSnapshotAndFailure(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(root, "wiki.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := db.Exec("CREATE TABLE proof(value TEXT);INSERT INTO proof VALUES('snapshot')"); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"docs/document.json": "正文", ".nice-wiki.lock": "lock", "config/secrets.json": "secret", "cache/models.json": "models", "cache/runtime.tmp": "cache", "docs/.nw-write-test": "partial"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(t.TempDir(), "backup.tar.gz")
	if err := Export(ctx, root, target, SQLSnapshotter{DB: db}, false); err != nil {
		t.Fatal(err)
	}
	entries := archiveEntries(t, target)
	for _, name := range []string{"wiki.db-wal", "wiki.db-shm", ".nice-wiki.lock", "config/secrets.json", "cache/runtime.tmp", "docs/.nw-write-test"} {
		if _, ok := entries[name]; ok {
			t.Fatalf("泄漏运行文件：%s", name)
		}
	}
	if entries["docs/document.json"] != "正文" || entries["cache/models.json"] != "models" {
		t.Fatal(entries)
	}
	restored := filepath.Join(t.TempDir(), "restored")
	if err := Restore(ctx, target, restored); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Open(ctx, filepath.Join(restored, "wiki.db"))
	if err != nil {
		t.Fatal(err)
	}
	var value string
	queryErr := snapshot.QueryRow("SELECT value FROM proof").Scan(&value)
	if err := errors.Join(queryErr, snapshot.Close()); err != nil {
		t.Fatal(err)
	}
	if value != "snapshot" {
		t.Fatal(value)
	}
	original, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := Export(cancelled, root, target, SQLSnapshotter{DB: db}, true); err == nil {
		t.Fatal("取消任务成功")
	}
	current, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != string(current) {
		t.Fatal("失败覆盖原备份")
	}
	if err := Export(ctx, root, target, SQLSnapshotter{DB: db}, true); err != nil {
		t.Fatal(err)
	}
	if archiveEntries(t, target)["config/secrets.json"] != "secret" {
		t.Fatal("密钥选项失效")
	}
}
func TestRestoreRejectsUnsafeArchive(t *testing.T) {
	for _, header := range []tar.Header{{Name: "..", Typeflag: tar.TypeDir}, {Name: "../escape", Typeflag: tar.TypeReg}, {Name: "link", Typeflag: tar.TypeSymlink, Linkname: "outside"}, {Name: "wiki.db-wal", Typeflag: tar.TypeReg}} {
		path := filepath.Join(t.TempDir(), "bad.tar.gz")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		gz := gzip.NewWriter(file)
		writer := tar.NewWriter(gz)
		err = writer.WriteHeader(&header)
		if err := errors.Join(err, writer.Close(), gz.Close(), file.Close()); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "target")
		if err := Restore(context.Background(), path, target); err == nil {
			t.Fatalf("接受非法文件：%s", header.Name)
		}
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("非法备份留下目标")
		}
	}
}
