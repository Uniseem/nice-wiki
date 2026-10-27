package internal

import (
	"context"
	"database/sql"
	"time"

	"nice-wiki/libs/platform"
)

const (
	progressRetention  = 7 * 24 * time.Hour
	terminalRetention  = 30 * 24 * time.Hour
	eventRetention     = 30 * 24 * time.Hour
	maxRetentionBatch  = 500
	terminalStateQuery = "('succeeded','cancelled','dead-letter')"
)

type PurgeExpiredHistoryInput struct {
	Now       time.Time
	BatchSize int
}

type PurgeExpiredHistoryResult struct {
	DeletedCount int64
}

// PurgeExpiredHistory 在一个短事务内清理过期进度、已全确认事件和无结果引用的
// 终局任务。批量上限按每类记录计算；任何付费结果引用、活动租约、orphan 或
// 未知活动依赖都保留任务，宁可多留历史也不丢弃无法证明安全的引用。
func PurgeExpiredHistory(ctx context.Context, op OperationContext, in PurgeExpiredHistoryInput) (PurgeExpiredHistoryResult, error) {
	if op.CallerService != "jobs" {
		return PurgeExpiredHistoryResult{}, platform.NewError(platform.CodeForbidden, false, "仅jobs后台可以清理历史")
	}
	if in.Now.IsZero() || in.BatchSize < 1 || in.BatchSize > maxRetentionBatch {
		return PurgeExpiredHistoryResult{}, platform.NewError(platform.CodeValidation, false, "历史清理时间或批量不合法")
	}
	work, err := beginLeaseOperation(ctx, op, op.DataEpoch)
	if err != nil {
		return PurgeExpiredHistoryResult{}, err
	}
	defer work.close()
	now := in.Now.UTC()
	progressCutoff := now.Add(-progressRetention)
	eventCutoff := now.Add(-eventRetention)
	terminalCutoff := now.Add(-terminalRetention)

	progressCount, err := purgeProgress(ctx, work.tx, op.DataEpoch, progressCutoff, in.BatchSize)
	if err != nil {
		return PurgeExpiredHistoryResult{}, err
	}
	var eventCount int64
	if q, err := currentEventQueue(); err == nil {
		if eventCount, err = q.PurgeEvents(ctx, eventCutoff); err != nil {
			return PurgeExpiredHistoryResult{}, err
		}
	}
	jobCount, err := purgeTerminalJobs(ctx, work.tx, op.DataEpoch, terminalCutoff, in.BatchSize)
	if err != nil {
		return PurgeExpiredHistoryResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return PurgeExpiredHistoryResult{}, mapQueueDBError(err)
	}
	return PurgeExpiredHistoryResult{DeletedCount: progressCount + eventCount + jobCount}, nil
}

func purgeProgress(ctx context.Context, tx *sql.Tx, epoch platform.Identifier, cutoff time.Time, limit int) (int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT p.job_id,p.seq,p.created_at
		FROM progress p JOIN jobs j ON j.job_id=p.job_id
		WHERE j.data_epoch=? AND p.created_at IS NOT NULL AND p.created_at<=?
		  AND j.state IN `+terminalStateQuery+`
		ORDER BY p.created_at,p.job_id,p.seq
		LIMIT ?`, epoch, cutoff.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return 0, mapQueueDBError(err)
	}
	type candidate struct {
		jobID  string
		seq    int64
		atText string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.jobID, &item.seq, &item.atText); err != nil {
			_ = rows.Close()
			return 0, mapQueueDBError(err)
		}
		expired, err := retentionTimeReached(item.atText, cutoff)
		if err != nil {
			_ = rows.Close()
			return 0, err
		}
		if expired {
			candidates = append(candidates, item)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, mapQueueDBError(err)
	}
	_ = rows.Close()
	var deleted int64
	for _, item := range candidates {
		result, err := tx.ExecContext(ctx, `DELETE FROM progress WHERE job_id=? AND seq=? AND job_id IN (SELECT job_id FROM jobs WHERE data_epoch=?)`, item.jobID, item.seq, epoch)
		if err != nil {
			return deleted, mapQueueDBError(err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return deleted, mapQueueDBError(err)
		}
		deleted += count
	}
	return deleted, nil
}

func purgeTerminalJobs(ctx context.Context, tx *sql.Tx, epoch platform.Identifier, cutoff time.Time, limit int) (int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT j.job_id,j.updated_at
		FROM jobs j
		JOIN job_results r ON r.job_id=j.job_id
		WHERE j.data_epoch=? AND j.state IN `+terminalStateQuery+`
		  AND j.updated_at<=?
		  AND NOT EXISTS(SELECT 1 FROM leases l WHERE l.job_id=j.job_id)
		  AND NOT EXISTS(SELECT 1 FROM job_orphans o WHERE o.job_id=j.job_id)
		  AND EXISTS(SELECT 1 FROM job_outcomes outcome
		    WHERE outcome.job_id=j.job_id AND outcome.operation_kind IN ('complete','fail')
		      AND json_valid(CAST(outcome.result_json AS TEXT))=1)
		  AND r.result_ref=''
		  AND json_type(CAST(r.paid_result_refs_json AS TEXT))='array'
		  AND json_array_length(CAST(r.paid_result_refs_json AS TEXT))=0
		ORDER BY j.updated_at,j.job_id
		LIMIT ?`, epoch, cutoff.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return 0, mapQueueDBError(err)
	}
	type candidate struct {
		id     string
		atText string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.atText); err != nil {
			_ = rows.Close()
			return 0, mapQueueDBError(err)
		}
		expired, err := retentionTimeReached(item.atText, cutoff)
		if err != nil {
			_ = rows.Close()
			return 0, err
		}
		if expired {
			candidates = append(candidates, item)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, mapQueueDBError(err)
	}
	_ = rows.Close()

	var deleted int64
	for _, candidate := range candidates {
		for _, statement := range []string{
			`DELETE FROM progress WHERE job_id=? AND job_id IN (SELECT job_id FROM jobs WHERE data_epoch=?)`,
			`DELETE FROM job_results WHERE job_id=? AND job_id IN (SELECT job_id FROM jobs WHERE data_epoch=?)`,
			`DELETE FROM job_outcomes WHERE job_id=? AND job_id IN (SELECT job_id FROM jobs WHERE data_epoch=?)`,
		} {
			if _, err := tx.ExecContext(ctx, statement, candidate.id, epoch); err != nil {
				return deleted, mapQueueDBError(err)
			}
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM jobs WHERE job_id=? AND data_epoch=?`, candidate.id, epoch)
		if err != nil {
			return deleted, mapQueueDBError(err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return deleted, mapQueueDBError(err)
		}
		deleted += count
	}
	return deleted, nil
}

func retentionTimeReached(value string, cutoff time.Time) (bool, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "历史时间不符合RFC3339")
	}
	return !parsed.After(cutoff), nil
}
