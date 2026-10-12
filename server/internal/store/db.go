package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"time"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := (&url.URL{Scheme: "file", Path: path}).String() + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(FULL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)
	ping, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(ping); err != nil {
		return nil, errors.Join(fmt.Errorf("打开数据库: %w", err), db.Close())
	}
	return db, nil
}
