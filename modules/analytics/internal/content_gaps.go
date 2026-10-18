package analytics

import (
	"context"
	"database/sql"
	"nice-wiki/libs/platform"
	"sort"
	"time"
)

type QaTotals struct {
	Completed         int64 `json:"completed"`
	Failed            int64 `json:"failed"`
	ExactCacheHits    int64 `json:"exactCacheHits"`
	SemanticCacheHits int64 `json:"semanticCacheHits"`
	Unanswered        int64 `json:"unanswered"`
}

// QueryContentGaps使用完整可信日历日，正文仍由qa自己的当前权限页面读取。
func QueryContentGaps(ctx context.Context, op OperationContext, in QueryContentGapsInput) (QueryContentGapsResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return QueryContentGapsResult{}, err
	}
	if rt.Authorize == nil {
		return QueryContentGapsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "问答资源权限客户端未装配")
	}
	if err = rt.Authorize(ctx, op, "qa.manage", ""); err != nil {
		return QueryContentGapsResult{}, err
	}
	pageInput := QueryPageStatisticsInput{From: in.From, To: in.To, Granularity: "day", Actor: op.Actor, DataEpoch: op.DataEpoch}
	begin, end, _, err := pageInterval(ctx, op, rt, pageInput)
	if err != nil {
		return QueryContentGapsResult{}, err
	}
	tx, err := rt.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return QueryContentGapsResult{}, storageError(err)
	}
	defer tx.Rollback()
	pages, err := readPageSnapshot(ctx, tx, op, pageInput, begin, end, rt.CountingLocation)
	if err != nil {
		return QueryContentGapsResult{}, err
	}
	totals := map[string]StatisticRow{}
	for _, row := range pages.Rows {
		key := row.DocumentID + "\x00" + row.Locale
		sum := totals[key]
		sum.DocumentID = row.DocumentID
		sum.Locale = row.Locale
		sum.PeriodStart = begin.Format(time.RFC3339Nano)
		if sum.Views > 9223372036854775807-row.Views || sum.Visits > 9223372036854775807-row.Visits {
			return QueryContentGapsResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "内容缺口计数超过精确整数预算")
		}
		sum.Views += row.Views
		sum.Visits += row.Visits
		totals[key] = sum
	}
	result := QueryContentGapsResult{HotDocuments: []StatisticRow{}, UnansweredRecordIDs: []string{}, PageWatermark: pages.Watermark, FromUTC: begin.Format(time.RFC3339Nano), ToUTC: end.Format(time.RFC3339Nano)}
	for _, row := range totals {
		result.HotDocuments = append(result.HotDocuments, row)
	}
	sort.Slice(result.HotDocuments, func(i, j int) bool {
		a, b := result.HotDocuments[i], result.HotDocuments[j]
		if a.Views != b.Views {
			return a.Views > b.Views
		}
		if a.DocumentID != b.DocumentID {
			return a.DocumentID < b.DocumentID
		}
		return a.Locale < b.Locale
	})
	rows, err := tx.QueryContext(ctx, `SELECT record_id FROM unanswered_records WHERE occurred_at>=? AND occurred_at<? AND data_epoch=? ORDER BY occurred_at,record_id LIMIT 10001`, begin.Format(analyticsTimeLayout), end.Format(analyticsTimeLayout), rt.Epoch)
	if err != nil {
		return QueryContentGapsResult{}, storageError(err)
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		result.UnansweredRecordIDs = append(result.UnansweredRecordIDs, id)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return QueryContentGapsResult{}, storageError(err)
	}
	if len(result.UnansweredRecordIDs) > 10000 {
		return QueryContentGapsResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "无答案引用超过响应预算")
	}
	dayBegin, dayEnd := begin.In(rt.CountingLocation).Format("2006-01-02"), end.In(rt.CountingLocation).Format("2006-01-02")
	rows, err = tx.QueryContext(ctx, `SELECT kind,count FROM qa_turn_counts WHERE day>=? AND day<? AND data_epoch=?`, dayBegin, dayEnd, rt.Epoch)
	if err != nil {
		return QueryContentGapsResult{}, storageError(err)
	}
	for rows.Next() {
		var kind string
		var count int64
		if err = rows.Scan(&kind, &count); err != nil {
			break
		}
		target := (*int64)(nil)
		switch kind {
		case "completed":
			target = &result.QaTotals.Completed
		case "cache.exact":
			target = &result.QaTotals.ExactCacheHits
		case "cache.semantic":
			target = &result.QaTotals.SemanticCacheHits
		case "answer.no_evidence", "answer.out_of_scope":
			target = &result.QaTotals.Unanswered
		default:
			if len(kind) >= 7 && kind[:7] == "failed." {
				target = &result.QaTotals.Failed
			}
		}
		if target != nil {
			if count < 0 || *target > 9223372036854775807-count {
				err = platform.NewError(platform.CodeIntegrityError, false, "问答计数超过精确整数预算")
				break
			}
			*target += count
		}
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return QueryContentGapsResult{}, storageError(err)
	}
	if err = tx.QueryRowContext(ctx, `SELECT coalesce((SELECT revision FROM aggregation_watermarks WHERE source='qa'),0),coalesce((SELECT updated_at FROM aggregation_watermarks WHERE source='qa'),'')`).Scan(&result.QaWatermark, &result.UpdatedAt); err != nil {
		return QueryContentGapsResult{}, storageError(err)
	}
	return result, nil
}
