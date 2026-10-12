package datadir

import (
	"errors"
	"nice-wiki/internal/fsutil"
	"os"
	"path/filepath"
)

func (l Layout) Initialize() error {
	dirs := []string{l.Root, l.Docs(), l.Drafts(), l.Trash(), l.Config(), l.Settings(), l.I18n(), l.Memory(), l.Backups(), l.Cache()}
	for _, kind := range []string{"images", "files", "videos", "drawings"} {
		dirs = append(dirs, filepath.Join(l.Assets(), kind))
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	if _, err := os.Stat(l.GitIgnore()); errors.Is(err, os.ErrNotExist) {
		return fsutil.WriteFile(l.GitIgnore(), []byte("*\n!.gitignore\n!docs/\n!docs/**\n!i18n/\n!i18n/**\n!settings/\n!settings/**\n!memory/\n!memory/**\n!redirects.json\n"), 0600)
	} else {
		return err
	}
}
