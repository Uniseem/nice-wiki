package setup

import (
	"context"
	"errors"
	"nice-wiki/internal/backup"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreVolumeRootAndRollback(t *testing.T) {
	ctx := context.Background()
	source, _ := datadir.New(t.TempDir())
	if err := source.Initialize(); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(ctx, source.Database())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE proof(value TEXT);INSERT INTO proof VALUES('new')"); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "backup.tar.gz")
	exportErr := backup.Export(ctx, source.Root, archive, backup.SQLSnapshotter{DB: db}, false)
	if err := errors.Join(exportErr, db.Close()); err != nil {
		t.Fatal(err)
	}
	target, _ := datadir.New(t.TempDir())
	if err := target.Initialize(); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(target.Root, "old.txt")
	if err := os.WriteFile(old, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	lock, err := target.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lock.Close(); err != nil {
			t.Error(err)
		}
	}()
	rootBefore, err := os.Stat(target.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareRestore(ctx, target, archive); err != nil {
		t.Fatal(err)
	}
	if err := ApplyPending(target); err != nil {
		t.Fatal(err)
	}
	rootAfter, err := os.Stat(target.Root)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(rootBefore, rootAfter) {
		t.Fatal("重命名了挂载根目录")
	}
	if _, err := os.Stat(filepath.Join(target.RestorePrevious(), "old.txt")); err != nil {
		t.Fatal(err)
	}
	if second, err := target.Acquire(); err == nil {
		if err := second.Close(); err != nil {
			t.Fatal(err)
		}
		t.Fatal("恢复丢失进程锁")
	}
	if err := RollbackPending(target); err != nil {
		t.Fatal(err)
	}
	if value, err := os.ReadFile(old); err != nil || string(value) != "old" {
		t.Fatalf("回滚原数据失败：%v", err)
	}
	if err := ApplyPending(target); err != nil {
		t.Fatal(err)
	}
	if err := CompleteRestore(target); err != nil {
		t.Fatal(err)
	}
	if err := RollbackPending(target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("已提交恢复又被回滚")
	}
}
func TestRestoreRejectsInvalidBackup(t *testing.T) {
	target, _ := datadir.New(t.TempDir())
	if err := target.Initialize(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "broken.tar.gz")
	if err := os.WriteFile(archive, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := PrepareRestore(context.Background(), target, archive); err == nil {
		t.Fatal("接受损坏备份")
	}
	if _, err := os.Stat(target.PendingRestore()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("留下恢复标记")
	}
}
