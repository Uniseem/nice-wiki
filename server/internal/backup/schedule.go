package backup

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Schedule(ctx context.Context, hour func() int, run func(context.Context) error, onError func(error)) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		lastRun := ""
		check := func(now time.Time) {
			configured := 0
			if hour != nil {
				configured = hour()
			}
			day := now.Format("2006-01-02")
			if now.Hour() != configured || lastRun == day {
				return
			}
			if err := run(ctx); err != nil {
				if onError != nil {
					onError(err)
				}
				return
			}
			lastRun = day
		}
		check(time.Now())
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				check(now)
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
