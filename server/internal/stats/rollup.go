package stats

import (
	"context"
	"time"
)
import (
	"database/sql"
	"nice-wiki/internal/store/sqlc"
)

type Daily struct {
	Day, Dimension, Value     string
	Language, Path            string
	Views, Visitors, AICrawls int64
}
type Store interface {
	ReadBeacons(context.Context, time.Time, time.Time) ([]Beacon, error)
	ReplaceDaily(context.Context, string, []Daily) error
	DeleteBeaconsBefore(context.Context, time.Time) error
}
type SQLStore struct{ DB *sql.DB }

func (s SQLStore) ReadBeacons(ctx context.Context, start, end time.Time) ([]Beacon, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT document_id,language,country,referrer_domain,visitor_hash,bot,ai_bot,created_at FROM page_views WHERE created_at>=? AND created_at<? ORDER BY created_at", start.Unix(), end.Unix())
	if err != nil {
		return nil, err
	}
	result := []Beacon{}
	for rows.Next() {
		var value Beacon
		var bot, ai int64
		var created int64
		if err := rows.Scan(&value.Path, &value.Language, &value.Country, &value.Referrer, &value.Visitor, &bot, &ai, &created); err != nil {
			return nil, err
		}
		value.Bot = bot != 0
		value.AI = ai != 0
		value.Created = time.Unix(created, 0)
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return result, nil
}
func (s SQLStore) ReplaceDaily(ctx context.Context, day string, values []Daily) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM daily_stats WHERE day=?", day); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, v := range values {
		dimension, value := v.Dimension, v.Value
		if dimension == "" {
			dimension, value = "path", v.Path
		}
		if dimension == "path" && v.Path == "" && v.AICrawls > 0 {
			dimension, value = "ai", "crawler"
			v.Views = v.AICrawls
		}
		if dimension == "ai" {
			v.Views = v.AICrawls
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO daily_stats(day,dimension,value,views,visitors) VALUES(?,?,?,?,?)", day, dimension, value, v.Views, v.Visitors); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
func (s SQLStore) DeleteBeaconsBefore(ctx context.Context, before time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM page_views WHERE created_at<?", before.Unix()); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM daily_salts WHERE day<?", before.UTC().Format("2006-01-02")); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

var _ = sqlc.DailyStat{}

func Rollup(ctx context.Context, store Store, day time.Time, retention int) error {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	values, err := store.ReadBeacons(ctx, start, start.Add(24*time.Hour))
	if err != nil {
		return err
	}
	type key struct{ dimension, value string }
	rows := map[key]*Daily{}
	visitors := map[key]map[string]bool{}
	add := func(dimension, value string, beacon Beacon) {
		if value == "" {
			return
		}
		k := key{dimension, value}
		if rows[k] == nil {
			rows[k] = &Daily{Day: start.Format("2006-01-02"), Dimension: dimension, Value: value}
			visitors[k] = map[string]bool{}
		}
		rows[k].Views++
		visitors[k][beacon.Visitor] = true
	}
	for _, v := range values {
		if v.AI {
			k := key{"ai", "crawler"}
			if rows[k] == nil {
				rows[k] = &Daily{Day: start.Format("2006-01-02"), Dimension: "ai", Value: "crawler"}
				visitors[k] = map[string]bool{}
			}
			rows[k].AICrawls++
			continue
		}
		if v.Bot {
			continue
		}
		add("total", "all", v)
		add("path", v.Path, v)
		add("language", v.Language, v)
		add("country", v.Country, v)
		add("referrer", v.Referrer, v)
	}
	result := []Daily{}
	for k, row := range rows {
		row.Visitors = int64(len(visitors[k]))
		result = append(result, *row)
	}
	if err := store.ReplaceDaily(ctx, start.Format("2006-01-02"), result); err != nil {
		return err
	}
	return store.DeleteBeaconsBefore(ctx, start.AddDate(0, 0, -retention))
}
