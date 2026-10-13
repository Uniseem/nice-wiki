package setup

import (
	"context"
	"errors"
	"nice-wiki/internal/backup"
	"nice-wiki/internal/content"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/fsutil"
	"nice-wiki/internal/store"
	"os"
	"path/filepath"
	"strings"
)

type restoreMarker struct {
	Directory string `json:"directory"`
}
type restoreJournal struct {
	Directory string   `json:"directory"`
	Old       []string `json:"old"`
	New       []string `json:"new"`
}

func PrepareRestore(ctx context.Context, layout datadir.Layout, archive string) (result error) {
	if _, err := os.Stat(layout.PendingRestore()); err == nil {
		return errors.New("已有待恢复备份")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent, err := os.MkdirTemp(layout.Root, ".restore-stage-")
	if err != nil {
		return err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, os.RemoveAll(parent))
		}
	}()
	target := filepath.Join(parent, "data")
	if err := backup.Restore(ctx, archive, target); err != nil {
		return err
	}
	if err := validateRestore(ctx, target); err != nil {
		return err
	}
	return fsutil.WriteJSON(layout.PendingRestore(), restoreMarker{Directory: filepath.Base(parent)})
}
func validateRestore(ctx context.Context, root string) error {
	layout, err := datadir.New(root)
	if err != nil {
		return err
	}
	info, err := os.Stat(layout.Database())
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("备份缺少数据库")
	}
	db, err := store.Open(ctx, layout.Database())
	if err != nil {
		return err
	}
	var integrity string
	checkErr := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&integrity)
	if err := errors.Join(checkErr, db.Close()); err != nil {
		return err
	}
	if integrity != "ok" {
		return errors.New("备份数据库损坏")
	}
	return filepath.WalkDir(layout.Docs(), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		var file content.File
		if !strings.HasSuffix(entry.Name(), ".json") {
			return errors.New("备份正文包含未知文件")
		}
		if err := fsutil.ReadJSON(path, &file); err != nil {
			return err
		}
		return file.Validate()
	})
}
func ApplyPending(layout datadir.Layout) (result error) {
	// 上次启动中断后优先恢复原数据，再重新处理完整的待恢复包。
	if err := RollbackPending(layout); err != nil {
		return err
	}
	var marker restoreMarker
	if err := fsutil.ReadJSON(layout.PendingRestore(), &marker); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if !strings.HasPrefix(marker.Directory, ".restore-stage-") || filepath.Base(marker.Directory) != marker.Directory {
		return errors.New("恢复目录标记无效")
	}
	stage := filepath.Join(layout.Root, marker.Directory, "data")
	if err := validateRestore(context.Background(), stage); err != nil {
		return err
	}
	if _, err := os.Stat(layout.RestorePrevious()); err == nil {
		return errors.New("上次恢复保留目录仍存在，请先人工处理")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	oldEntries, err := os.ReadDir(layout.Root)
	if err != nil {
		return err
	}
	newEntries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	journal := restoreJournal{Directory: marker.Directory}
	for _, entry := range oldEntries {
		name := entry.Name()
		if name == ".nice-wiki.lock" || strings.HasPrefix(name, ".restore-") {
			continue
		}
		journal.Old = append(journal.Old, name)
	}
	for _, entry := range newEntries {
		if strings.HasPrefix(entry.Name(), ".restore-") || entry.Name() == ".nice-wiki.lock" {
			return errors.New("备份含保留路径")
		}
		journal.New = append(journal.New, entry.Name())
	}
	if err := os.Mkdir(layout.RestorePrevious(), 0700); err != nil {
		return err
	}
	if err := fsutil.WriteJSON(layout.RestoreJournal(), journal); err != nil {
		return errors.Join(err, os.Remove(layout.RestorePrevious()))
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, RollbackPending(layout))
		}
	}()
	for _, name := range journal.Old {
		if err := os.Rename(filepath.Join(layout.Root, name), filepath.Join(layout.RestorePrevious(), name)); err != nil {
			return err
		}
	}
	for _, name := range journal.New {
		if err := os.Rename(filepath.Join(stage, name), filepath.Join(layout.Root, name)); err != nil {
			return err
		}
	}
	return nil
}
func RollbackPending(layout datadir.Layout) error {
	var journal restoreJournal
	if err := fsutil.ReadJSON(layout.RestoreJournal(), &journal); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if filepath.Base(journal.Directory) != journal.Directory || !strings.HasPrefix(journal.Directory, ".restore-stage-") {
		return errors.New("恢复日志无效")
	}
	stage := filepath.Join(layout.Root, journal.Directory, "data")
	for _, name := range append(append([]string{}, journal.Old...), journal.New...) {
		if name == "." || name == ".." || filepath.Base(name) != name || strings.HasPrefix(name, ".restore-") {
			return errors.New("恢复日志路径无效")
		}
	}
	for _, name := range journal.New {
		staged := filepath.Join(stage, name)
		if _, err := os.Stat(staged); errors.Is(err, os.ErrNotExist) {
			if _, err := os.Stat(filepath.Join(layout.Root, name)); err == nil {
				if err := os.Rename(filepath.Join(layout.Root, name), staged); err != nil {
					return err
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	for _, name := range journal.Old {
		old := filepath.Join(layout.RestorePrevious(), name)
		if _, err := os.Stat(old); err == nil {
			if err := os.Rename(old, filepath.Join(layout.Root, name)); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return errors.Join(os.Remove(layout.RestoreJournal()), os.Remove(layout.RestorePrevious()))
}

// CompleteRestore 仅在应用完整初始化后提交切换；原数据保留供人工回退。
func CompleteRestore(layout datadir.Layout) error {
	var journal restoreJournal
	if err := fsutil.ReadJSON(layout.RestoreJournal(), &journal); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := os.Remove(layout.RestoreJournal()); err != nil {
		return err
	}
	return nil
}
