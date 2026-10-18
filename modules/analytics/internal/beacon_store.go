package analytics

import (
	"context"
	"nice-wiki/libs/platform"
	"time"
)

// FlushBeacons只报告真实新增行；漏数无后续访问时也独立持久化。
func FlushBeacons(ctx context.Context, op OperationContext, in FlushBeaconsInput) (FlushBeaconsResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return FlushBeaconsResult{}, err
	}
	rt.flushMu.Lock()
	defer rt.flushMu.Unlock()
	rt.mu.Lock()
	n := int(in.BatchSize)
	if n <= 0 || n > 500 {
		n = 500
	}
	if n > len(rt.queue) {
		n = len(rt.queue)
	}
	batch := append([]beaconRow(nil), rt.queue[:n]...)
	lost := rt.dropped
	rt.mu.Unlock()
	if n == 0 && lost == 0 {
		return FlushBeaconsResult{}, nil
	}
	work, _, tx, cancel, err := beginWrite(ctx, op)
	var persisted int64
	if err == nil {
		defer cancel()
		defer tx.Rollback()
		periods := map[string]string{}
		for _, b := range batch {
			var resultErr error
			result, resultErr := tx.ExecContext(work, `INSERT OR IGNORE INTO beacon_inbox VALUES(?,?,?,?,?,?,?,?,?,?,?)`, b.ID, b.DocumentID, b.Locale, b.EventType, b.Status/100, b.Bucket, b.DayHash, b.Source, b.Country, b.At, b.Epoch)
			if resultErr != nil {
				err = resultErr
				break
			}
			added, e := result.RowsAffected()
			if e != nil {
				err = e
				break
			}
			persisted += added
			if added != 0 {
				at, _ := time.Parse(analyticsTimeLayout, b.At)
				periods["hourly/"+at.Truncate(time.Hour).Format(analyticsTimeLayout)] = "hourly"
				if rt.CountingLocation != nil {
					periods["daily/"+at.In(rt.CountingLocation).Format("2006-01-02")+"/"+rt.CountingLocation.String()] = "daily"
				}
			}
		}
		var revision int64
		if err == nil && (persisted != 0 || lost != 0) {
			source := "beacons"
			if persisted == 0 {
				source = "beacon-loss"
			}
			revision, err = advanceWatermark(work, tx, source)
		}
		if err == nil {
			for compound, kind := range periods {
				key := compound[len(kind)+1:]
				_, err = tx.ExecContext(work, `INSERT INTO beacon_period_watermarks(kind,period_key,revision) VALUES(?,?,?) ON CONFLICT(kind,period_key) DO UPDATE SET revision=excluded.revision`, kind, key, revision)
				if err != nil {
					break
				}
			}
		}
		if err == nil && lost != 0 {
			result, e := tx.ExecContext(work, `UPDATE analytics_meta SET dropped=dropped+? WHERE singleton=1 AND dropped<=9223372036854775807-?`, lost, lost)
			err = e
			if err == nil {
				affected, e := result.RowsAffected()
				err = e
				if err == nil && affected != 1 {
					err = platform.NewError(platform.CodeBudgetExceeded, false, "漏数计数超过精确整数预算")
				}
			}
		}
		if err == nil {
			err = tx.Commit()
		}
	}
	if err != nil {
		rt.mu.Lock()
		keep := 0
		now := time.Now()
		for _, b := range rt.queue {
			if now.Sub(b.QueuedAt) < 5*time.Second {
				rt.queue[keep] = b
				keep++
			} else {
				rt.dropped++
			}
		}
		rt.queue = rt.queue[:keep]
		rt.mu.Unlock()
		return FlushBeaconsResult{}, platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, true, "匿名访问持久化失败")
	}
	rt.mu.Lock()
	copy(rt.queue, rt.queue[n:])
	rt.queue = rt.queue[:len(rt.queue)-n]
	rt.dropped -= lost
	rt.mu.Unlock()
	return FlushBeaconsResult{Persisted: persisted, Dropped: lost}, nil
}
