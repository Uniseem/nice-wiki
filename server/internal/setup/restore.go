package setup

import (
	"context"
	"errors"
	"nice-wiki/internal/backup"
	"nice-wiki/internal/datadir"
	"nice-wiki/internal/fsutil"
	"os"
	"path/filepath"
	"time"
)

type restoreMarker struct {
	Directory string `json:"directory"`
}

func PrepareRestore(ctx context.Context, layout datadir.Layout, archive string) error {
	parent := filepath.Dir(layout.Root)
	name := filepath.Base(layout.Root) + ".restore-" + time.Now().UTC().Format("20060102T150405")
	target := filepath.Join(parent, name)
	if err := backup.Restore(ctx, archive, target); err != nil {
		return err
	}
	return fsutil.WriteJSON(layout.PendingRestore(), restoreMarker{Directory: name})
}
func ApplyPending(layout datadir.Layout) error {
	var marker restoreMarker
	if err := fsutil.ReadJSON(layout.PendingRestore(), &marker); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if filepath.Base(marker.Directory) != marker.Directory {
		return errors.New("恢复目录标记无效")
	}
	parent := filepath.Dir(layout.Root)
	staged := filepath.Join(parent, marker.Directory)
	if info, err := os.Stat(staged); err != nil || !info.IsDir() {
		return errors.New("待恢复目录不存在")
	}
	old := layout.Root + ".pre-restore"
	if _, err := os.Stat(old); err == nil {
		return errors.New("上次恢复保留目录仍存在，请先人工处理")
	}
	if err := os.Rename(layout.Root, old); err != nil {
		return err
	}
	if err := os.Rename(staged, layout.Root); err != nil {
		return errors.Join(err, os.Rename(old, layout.Root))
	}
	return nil
}
