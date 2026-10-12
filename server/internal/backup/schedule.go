package backup

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Schedule(ctx context.Context, hour int, run func(context.Context) error, onError func(error)) {
	go func() {
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-timer.C:
				next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
				if !next.After(now) {
					next = next.Add(24 * time.Hour)
				}
				timer.Reset(time.Until(next))
				if now.Hour() == hour {
					if err := run(ctx); err != nil && onError != nil {
						onError(err)
					}
				}
			}
		}
	}()
}
func Retain(directory string, keep int) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tar.gz") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for len(names) > keep {
		if err := os.Remove(filepath.Join(directory, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}
