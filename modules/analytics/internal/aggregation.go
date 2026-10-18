package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"time"
)

// AggregateHourly 在同一事务固定明细快照，替换已封闭的小时桶和水位。
func AggregateHourly(ctx context.Context, op OperationContext, in AggregateHourlyInput) (AggregateHourlyResult, error) {
	hour, err := time.Parse(time.RFC3339Nano, in.Hour)
	if err != nil || !hour.Equal(hour.UTC().Truncate(time.Hour)) || hour.Add(time.Hour).After(time.Now().UTC()) {
		return AggregateHourlyResult{}, platform.NewError(platform.CodeValidation, false, "聚合小时必须为已封闭UTC整点")
	}
	if err = checkAggregateLease(ctx, in.Execution); err != nil {
		return AggregateHourlyResult{}, err
	}
	work, _, tx, cancel, err := beginWrite(ctx, op)
	if err != nil {
		return AggregateHourlyResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	if err = verifyAggregateLease(work, tx, op, in.Execution); err != nil {
		return AggregateHourlyResult{}, err
	}
	var previous AggregateHourlyResult
	found, source, err := readAggregate(work, tx, "hourly", hour.UTC().Format(analyticsTimeLayout), in.SourceWatermark, &previous)
	if err != nil {
		return AggregateHourlyResult{}, err
	}
	if found {
		return previous, nil
	}
	if hour.Before(time.Now().UTC().AddDate(0, 0, -30)) {
		return AggregateHourlyResult{}, platform.NewError(platform.CodeStaleResult, false, "原始明细已超过保留期，不能重建缺失小时")
	}
	start, end := hour.UTC().Format(analyticsTimeLayout), hour.Add(time.Hour).UTC().Format(analyticsTimeLayout)
	rows, err := tx.QueryContext(work, "SELECT document_id,locale,count(*),count(DISTINCT daily_visitor_hash) FROM beacon_inbox WHERE occurred_at>=? AND occurred_at<? AND event_type='page-view' AND data_epoch=? GROUP BY document_id,locale ORDER BY document_id,locale LIMIT 10001", start, end, op.DataEpoch)
	if err != nil {
		return AggregateHourlyResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "读取匿名小时明细失败")
	}
	result := AggregateHourlyResult{Rows: []StatisticRow{}}
	for rows.Next() {
		row := StatisticRow{PeriodStart: start}
		if err = rows.Scan(&row.DocumentID, &row.Locale, &row.Views, &row.Visits); err != nil {
			break
		}
		result.Rows = append(result.Rows, row)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return AggregateHourlyResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "读取匿名小时分组失败")
	}
	if len(result.Rows) > 10000 {
		return AggregateHourlyResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "小时聚合超过结果预算")
	}
	if _, err = tx.ExecContext(work, "DELETE FROM hourly_page_counts WHERE hour=?", start); err != nil {
		return AggregateHourlyResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "替换小时聚合失败")
	}
	for _, row := range result.Rows {
		bucketRows, e := tx.QueryContext(work, "SELECT elapsed_bucket,count(*) FROM beacon_inbox WHERE occurred_at>=? AND occurred_at<? AND event_type='page-view' AND document_id=? AND locale=? AND data_epoch=? GROUP BY elapsed_bucket", start, end, row.DocumentID, row.Locale, op.DataEpoch)
		if e != nil {
			return AggregateHourlyResult{}, platform.NormalizeIOError(e, platform.CodeInternal, true, "读取时延分桶失败")
		}
		buckets := map[string]int64{}
		for bucketRows.Next() {
			var bucket string
			var count int64
			if e = bucketRows.Scan(&bucket, &count); e != nil {
				break
			}
			buckets[bucket] = count
		}
		if e == nil {
			e = bucketRows.Err()
		}
		_ = bucketRows.Close()
		if e != nil {
			return AggregateHourlyResult{}, platform.NormalizeIOError(e, platform.CodeInternal, true, "读取时延分桶失败")
		}
		raw, e := platform.CanonicalJSON(buckets)
		if e != nil {
			return AggregateHourlyResult{}, e
		}
		if _, err = tx.ExecContext(work, "INSERT INTO hourly_page_counts VALUES(?,?,?,?,?,?,?)", start, row.DocumentID, row.Locale, row.Views, row.Visits, string(raw), op.DataEpoch); err != nil {
			return AggregateHourlyResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "保存小时聚合失败")
		}
	}
	result.Revision, err = advanceWatermark(work, tx, "pages")
	if err == nil {
		err = enqueueAggregate(tx, op, result.Revision, hour.Format("2006-01-02"), "hourly")
	}
	if err == nil {
		err = saveAggregate(work, tx, "hourly", hour.UTC().Format(analyticsTimeLayout), source, result)
	}
	if err == nil {
		err = verifyAggregateLease(work, tx, op, in.Execution)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		return AggregateHourlyResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "提交小时聚合失败")
	}
	return result, nil
}

// AggregateDaily 采用IANA日界线，夏令时日不假定为24小时。
func AggregateDaily(ctx context.Context, op OperationContext, in AggregateDailyInput) (AggregateDailyResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return AggregateDailyResult{}, err
	}
	loc := rt.CountingLocation
	if loc == nil {
		return AggregateDailyResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "可信报表时区未配置")
	}
	if in.Timezone != loc.String() {
		return AggregateDailyResult{}, platform.NewError(platform.CodeValidation, false, "任务日界线与可信部署时区不同")
	}
	day, err := time.ParseInLocation("2006-01-02", in.Day, loc)
	if err != nil || day.AddDate(0, 0, 1).After(time.Now()) {
		return AggregateDailyResult{}, platform.NewError(platform.CodeValidation, false, "日聚合尚未封闭")
	}
	if err = checkAggregateLease(ctx, in.Execution); err != nil {
		return AggregateDailyResult{}, err
	}
	work, _, tx, cancel, err := beginWrite(ctx, op)
	if err != nil {
		return AggregateDailyResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	if err = verifyAggregateLease(work, tx, op, in.Execution); err != nil {
		return AggregateDailyResult{}, err
	}
	var previous AggregateDailyResult
	key := in.Day + "/" + in.Timezone
	found, source, err := readAggregate(work, tx, "daily", key, in.SourceWatermark, &previous)
	if err != nil {
		return AggregateDailyResult{}, err
	}
	if found {
		return previous, nil
	}
	if day.Before(time.Now().In(loc).AddDate(0, 0, -30)) {
		return AggregateDailyResult{}, platform.NewError(platform.CodeStaleResult, false, "原始明细已超过保留期，不能重建缺失日报")
	}
	if _, err = tx.ExecContext(work, "DELETE FROM daily_page_counts WHERE day=? AND timezone=?", in.Day, in.Timezone); err != nil {
		return AggregateDailyResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "替换日聚合失败")
	}
	_, err = tx.ExecContext(work, "INSERT INTO daily_page_counts SELECT ?,?,document_id,locale,count(*),count(DISTINCT daily_visitor_hash),? FROM beacon_inbox WHERE occurred_at>=? AND occurred_at<? AND event_type='page-view' AND data_epoch=? GROUP BY document_id,locale", in.Day, in.Timezone, op.DataEpoch, day.UTC().Format(analyticsTimeLayout), day.AddDate(0, 0, 1).UTC().Format(analyticsTimeLayout), op.DataEpoch)
	if err != nil {
		return AggregateDailyResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "保存日聚合失败")
	}
	if _, err = tx.ExecContext(work, `DELETE FROM daily_page_dimensions WHERE day=? AND timezone=?`, in.Day, in.Timezone); err != nil {
		return AggregateDailyResult{}, storageError(err)
	}
	if _, err = tx.ExecContext(work, `INSERT INTO daily_page_dimensions SELECT ?,?,document_id,locale,source_domain,country_code,count(*),count(DISTINCT daily_visitor_hash),? FROM beacon_inbox WHERE occurred_at>=? AND occurred_at<? AND event_type='page-view' AND data_epoch=? GROUP BY document_id,locale,source_domain,country_code`, in.Day, in.Timezone, op.DataEpoch, day.UTC().Format(analyticsTimeLayout), day.AddDate(0, 0, 1).UTC().Format(analyticsTimeLayout), op.DataEpoch); err != nil {
		return AggregateDailyResult{}, storageError(err)
	}
	var count int64
	if err = tx.QueryRowContext(work, `SELECT count(*) FROM daily_page_dimensions WHERE day=? AND timezone=?`, in.Day, in.Timezone).Scan(&count); err != nil {
		return AggregateDailyResult{}, storageError(err)
	}
	if count > 100000 {
		return AggregateDailyResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "日报维度超过预算")
	}
	revision, err := advanceWatermark(work, tx, "pages")
	if err == nil {
		err = enqueueAggregate(tx, op, revision, in.Day, "daily")
	}
	if err == nil {
		err = saveAggregate(work, tx, "daily", key, source, AggregateDailyResult{Revision: revision})
	}
	if err == nil {
		err = verifyAggregateLease(work, tx, op, in.Execution)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		return AggregateDailyResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "提交日聚合失败")
	}
	return AggregateDailyResult{Revision: revision}, nil
}

func enqueueAggregate(tx *sql.Tx, op OperationContext, revision int64, day, kind string) error {
	raw, err := json.Marshal(struct {
		Revision int64  `json:"aggregateRevision"`
		Day      string `json:"day"`
		Kind     string `json:"kind"`
	}{revision, day, kind})
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "聚合事件编码失败")
	}
	return outbox.EnqueueEvent(tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: "analytics.aggregate-updated.v1", EventVersion: 1, Producer: "analytics", SubjectID: "analytics." + kind, Revision: revision, OccurredAt: platform.Now(), DataEpoch: op.DataEpoch, Payload: raw})
}

func readAggregate(ctx context.Context, tx *sql.Tx, kind, key string, expected int64, out any) (bool, int64, error) {
	var source int64
	err := tx.QueryRowContext(ctx, "SELECT revision FROM beacon_period_watermarks WHERE kind=? AND period_key=?", kind, key).Scan(&source)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if err != nil {
		return false, 0, platform.NormalizeIOError(err, platform.CodeInternal, true, "读取明细来源水位失败")
	}
	if source != expected {
		return false, source, platform.NewError(platform.CodeStaleResult, false, "任务来源水位已有真实修订，须新不可变输入")
	}
	var stored int64
	var raw string
	err = tx.QueryRowContext(ctx, "SELECT source_watermark,result_json FROM aggregate_runs WHERE kind=? AND period_key=?", kind, key).Scan(&stored, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, source, nil
	}
	if err != nil {
		return false, 0, platform.NormalizeIOError(err, platform.CodeInternal, true, "读取聚合原结果失败")
	}
	if source != stored {
		return false, source, nil
	}
	if json.Unmarshal([]byte(raw), out) != nil {
		return false, 0, platform.NewError(platform.CodeIntegrityError, false, "聚合原结果损坏")
	}
	return true, source, nil
}

func saveAggregate(ctx context.Context, tx *sql.Tx, kind, key string, source int64, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "聚合结果编码失败")
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO aggregate_runs VALUES(?,?,?,?) ON CONFLICT(kind,period_key) DO UPDATE SET source_watermark=excluded.source_watermark,result_json=excluded.result_json", kind, key, source, string(raw))
	return platform.NormalizeIOError(err, platform.CodeInternal, true, "保存聚合固定来源结果失败")
}
