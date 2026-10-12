package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		data, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		checksum := hex.EncodeToString(hash[:])
		err = Transaction(ctx, db, func(tx *sql.Tx) error {
			var old string
			err := tx.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE name = ?", entry.Name()).Scan(&old)
			if err == nil {
				if old != checksum {
					return fmt.Errorf("已执行迁移被修改: %s", entry.Name())
				}
				return nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if _, err := tx.ExecContext(ctx, string(data)); err != nil {
				return fmt.Errorf("执行迁移 %s: %w", entry.Name(), err)
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(name,checksum) VALUES(?,?)", entry.Name(), checksum)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}
