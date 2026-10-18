package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"strings"
	"time"
)

type auditCursor struct {
	After   int64  `json:"after"`
	Upper   int64  `json:"upper"`
	Binding string `json:"binding"`
	Expires int64  `json:"expires"`
	Anchor  int64  `json:"anchor"`
}

// QueryPageStatistics 只读固定数据库快照，不把派生值当实时额度。
func QueryPageStatistics(ctx context.Context, op OperationContext, in QueryPageStatisticsInput) (QueryPageStatisticsResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return QueryPageStatisticsResult{}, err
	}
	begin, end, granularity, err := pageInterval(ctx, op, rt, in)
	if err != nil {
		return QueryPageStatisticsResult{}, err
	}
	in.Granularity = granularity
	tx, err := rt.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return QueryPageStatisticsResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "开始统计读取失败")
	}
	defer tx.Rollback()
	return readPageSnapshot(ctx, tx, op, in, begin, end, rt.CountingLocation)

}

// QueryAudits 的签名游标绑定身份/权限修订/站点/代次/筛选/页大小与固定上界。
func QueryAudits(ctx context.Context, op OperationContext, in QueryAuditsInput) (QueryAuditsResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return QueryAuditsResult{}, err
	}
	if in.Limit == 0 {
		in.Limit = 20
	}
	if in.Limit < 1 || in.Limit > 100 {
		return QueryAuditsResult{}, platform.NewError(platform.CodeValidation, false, "审计分页大小超出范围")
	}
	if (in.From == "") != (in.To == "") {
		return QueryAuditsResult{}, platform.NewError(platform.CodeValidation, false, "审计日期范围必须完整同现")
	}
	from, to := "", ""
	if in.From != "" {
		interval, e := ValidateDateRange(ctx, op, ValidateDateRangeInput{From: in.From, To: in.To, MaxDays: 366})
		if e != nil {
			return QueryAuditsResult{}, e
		}
		begin, _ := time.Parse(time.RFC3339Nano, interval.FromUTC)
		end, _ := time.Parse(time.RFC3339Nano, interval.ToUTC)
		from = begin.Format(analyticsTimeLayout)
		to = end.Format(analyticsTimeLayout)
	}
	bindingRaw, err := platform.CanonicalJSON([]any{op.Actor.SubjectID, op.Actor.SessionID, op.Actor.PermissionRevision, op.Actor.SessionRevision, op.Actor.SiteID, op.DataEpoch, in.Action, in.Limit, from, to})
	if err != nil {
		return QueryAuditsResult{}, err
	}
	binding := platform.HashBytes(bindingRaw)
	scope := "analytics.audits.v1:" + binding
	tx, err := rt.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return QueryAuditsResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "开始审计快照失败")
	}
	defer tx.Rollback()
	var anchor int64
	if err = tx.QueryRowContext(ctx, "SELECT coalesce(max(last_seq),0) FROM audit_anchors").Scan(&anchor); err != nil {
		return QueryAuditsResult{}, storageError(err)
	}
	cursor := auditCursor{Binding: binding, Anchor: anchor, After: anchor, Expires: time.Now().UTC().Add(15 * time.Minute).Unix()}
	if in.Cursor == "" {
		if err = tx.QueryRowContext(ctx, "SELECT coalesce(max(seq),0) FROM audit_records").Scan(&cursor.Upper); err != nil {
			return QueryAuditsResult{}, storageError(err)
		}
		if cursor.Upper < anchor {
			cursor.Upper = anchor
		}
	} else {
		raw, verifyErr := serviceauth.VerifyCursor(rt.Key, scope, in.Cursor)
		if verifyErr != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Binding != binding || cursor.Expires <= time.Now().UTC().Unix() || cursor.Anchor != anchor || cursor.After < anchor || cursor.After > cursor.Upper {
			return QueryAuditsResult{}, platform.NewError(platform.CodeStaleResult, false, "审计游标过期、保留锚或身份筛选已经变化")
		}
	}
	rows, err := tx.QueryContext(ctx, "SELECT seq,audit_id FROM audit_records WHERE seq>? AND seq<=? AND (?='' OR action=?) AND (?='' OR occurred_at>=?) AND (?='' OR occurred_at<?) AND data_epoch=? ORDER BY seq LIMIT ?", cursor.After, cursor.Upper, in.Action, in.Action, from, from, to, to, op.DataEpoch, in.Limit+1)
	if err != nil {
		return QueryAuditsResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "读取审计分页失败")
	}
	defer rows.Close()
	result := QueryAuditsResult{AuditIDs: []string{}}
	var last int64
	hasMore := false
	for rows.Next() {
		var seq int64
		var id string
		if err = rows.Scan(&seq, &id); err != nil {
			return QueryAuditsResult{}, platform.NormalizeIOError(err, platform.CodeIntegrityError, false, "审计分页损坏")
		}
		if int64(len(result.AuditIDs)) == in.Limit {
			hasMore = true
			break
		}
		last = seq
		result.AuditIDs = append(result.AuditIDs, id)
	}
	if err = rows.Err(); err != nil {
		return QueryAuditsResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "读取审计分页失败")
	}
	if hasMore {
		cursor.After = last
		raw, encodeErr := platform.CanonicalJSON(cursor)
		if encodeErr != nil {
			return QueryAuditsResult{}, encodeErr
		}
		result.NextCursor, err = serviceauth.SignCursor(rt.Key, scope, raw)
		if err != nil {
			return QueryAuditsResult{}, err
		}
	}
	return result, nil
}

// ReadAudit 同一快照核对本条内容hash与前驱链接，损坏只报告false。
func ReadAudit(ctx context.Context, op OperationContext, in ReadAuditInput) (ReadAuditResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return ReadAuditResult{}, err
	}
	tx, err := rt.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ReadAuditResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "开始审计读取失败")
	}
	defer tx.Rollback()
	var seq int64
	var record AuditRecord
	err = tx.QueryRowContext(ctx, "SELECT seq,audit_id,actor_id,action,resource_ref,result_code,occurred_at,hash,previous_hash,metadata,hash_schema_version,producer,actor_kind,source_operation_id,origin_epoch,origin_site_id FROM audit_records WHERE audit_id=? AND data_epoch=?", in.AuditID, op.DataEpoch).Scan(&seq, &record.AuditID, &record.ActorID, &record.Action, &record.ResourceID, &record.ResultCode, &record.OccurredAt, &record.Hash, &record.PreviousHash, &record.Metadata, &record.SchemaVersion, &record.Producer, &record.ActorKind, &record.SourceOperationID, &record.OriginEpoch, &record.SiteID)
	if errors.Is(err, sql.ErrNoRows) {
		return ReadAuditResult{}, platform.NewError(platform.CodeNotFound, false, "审计引用不存在")
	}
	if err != nil {
		return ReadAuditResult{}, platform.NormalizeIOError(err, platform.CodeIntegrityError, false, "审计记录损坏")
	}
	if !platform.ValidHash(record.Hash) || !platform.ValidHash(record.PreviousHash) {
		return ReadAuditResult{}, platform.NewError(platform.CodeIntegrityError, false, "审计hash编码损坏")
	}
	previous := strings.Repeat("0", 64)
	err = tx.QueryRowContext(ctx, "SELECT hash FROM audit_records WHERE seq<? ORDER BY seq DESC LIMIT 1", seq).Scan(&previous)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, "SELECT hash FROM audit_anchors WHERE last_seq<? ORDER BY last_seq DESC LIMIT 1", seq).Scan(&previous)
		if errors.Is(err, sql.ErrNoRows) {
			err = nil
		}
	}
	if err != nil {
		return ReadAuditResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "读取审计链前驱失败")
	}
	hash, err := ComputeAuditHash(ctx, op, ComputeAuditHashInput{PreviousHash: record.PreviousHash, AuditJSON: record})
	if err != nil {
		return ReadAuditResult{}, err
	}
	valid := hash.Hash == record.Hash && previous == record.PreviousHash
	safe, redactErr := RedactAuditMetadata(ctx, op, RedactAuditMetadataInput{Action: record.Action, Metadata: record.Metadata})
	if redactErr != nil {
		record.Metadata = ""
		valid = false
	} else {
		record.Metadata = safe.SafeMetadata
	}
	return ReadAuditResult{AuditJSON: record, ChainValid: valid}, nil
}

func readPageSnapshot(ctx context.Context, tx *sql.Tx, op OperationContext, in QueryPageStatisticsInput, begin, end time.Time, location *time.Location) (QueryPageStatisticsResult, error) {
	result := QueryPageStatisticsResult{Rows: []StatisticRow{}, Granularity: in.Granularity, FromUTC: begin.UTC().Format(time.RFC3339Nano), ToUTC: end.UTC().Format(time.RFC3339Nano)}
	if err := tx.QueryRowContext(ctx, "SELECT coalesce((SELECT revision FROM aggregation_watermarks WHERE source='pages'),0),dropped FROM analytics_meta WHERE singleton=1").Scan(&result.Watermark, &result.DropCount); err != nil {
		return QueryPageStatisticsResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "读取统计水位失败")
	}
	query := "SELECT hour,document_id,locale,views,visits FROM hourly_page_counts WHERE hour>=? AND hour<? AND (?='' OR locale=?) AND (?='' OR document_id=?) AND data_epoch=? ORDER BY hour,document_id,locale LIMIT 10001"
	args := []any{begin.UTC().Format(analyticsTimeLayout), end.UTC().Format(analyticsTimeLayout), in.Locale, in.Locale, in.DocumentID, in.DocumentID, op.DataEpoch}
	if in.Granularity == "day" {
		query = "SELECT day,document_id,locale,views,visits FROM daily_page_counts WHERE day>=? AND day<? AND timezone=? AND (?='' OR locale=?) AND (?='' OR document_id=?) AND data_epoch=? ORDER BY day,document_id,locale LIMIT 10001"
		args = []any{begin.In(location).Format("2006-01-02"), end.In(location).Format("2006-01-02"), location.String(), in.Locale, in.Locale, in.DocumentID, in.DocumentID, op.DataEpoch}
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return QueryPageStatisticsResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "查询访问汇总失败")
	}
	defer rows.Close()
	for rows.Next() {
		var row StatisticRow
		if err = rows.Scan(&row.PeriodStart, &row.DocumentID, &row.Locale, &row.Views, &row.Visits); err != nil {
			return QueryPageStatisticsResult{}, platform.NormalizeIOError(err, platform.CodeIntegrityError, false, "访问汇总损坏")
		}
		if in.Granularity == "day" {
			day, parseErr := time.ParseInLocation("2006-01-02", row.PeriodStart, location)
			if parseErr != nil {
				return QueryPageStatisticsResult{}, platform.NewError(platform.CodeIntegrityError, false, "报表日期损坏")
			}
			row.PeriodStart = day.UTC().Format(time.RFC3339Nano)
		}
		result.Rows = append(result.Rows, row)
	}
	if err = rows.Err(); err != nil {
		return QueryPageStatisticsResult{}, platform.NormalizeIOError(err, platform.CodeInternal, true, "读取访问汇总失败")
	}
	if len(result.Rows) > 10000 {
		return QueryPageStatisticsResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "统计范围超过响应预算")
	}

	return result, nil
}
