package stats

import (
	"context"
	"errors"
	"nice-wiki/internal/store"
	"path/filepath"
	"testing"
	"time"
)

func TestRollupSeparatesCrawlersAndLanguages(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "wiki.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, v := range []struct {
		language, visitor string
		bot, ai           int
	}{{"en", "one", 0, 0}, {"zh-Hans", "one", 0, 0}, {"en", "two", 0, 0}, {"en", "crawler", 1, 1}, {"en", "search", 1, 0}} {
		if _, err := db.Exec("INSERT INTO page_views(document_id,language,country,referrer_domain,visitor_hash,bot,ai_bot,created_at) VALUES('page',?,'','',?,?,?,?)", v.language, v.visitor, v.bot, v.ai, day.Unix()); err != nil {
			t.Fatal(err)
		}
	}
	if err := Rollup(ctx, SQLStore{DB: db}, day, 30); err != nil {
		t.Fatal(err)
	}
	var views, visitors, ai int64
	err = db.QueryRow("SELECT views,visitors FROM daily_stats WHERE dimension='path' AND value='page'").Scan(&views, &visitors)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow("SELECT views FROM daily_stats WHERE dimension='ai'").Scan(&ai)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if views != 3 || visitors != 2 || ai != 1 {
		t.Fatalf("views=%d visitors=%d ai=%d", views, visitors, ai)
	}
	if err := Rollup(ctx, SQLStore{DB: db}, day, 30); err != nil {
		t.Fatal("重复汇总失败", err)
	}
}
