package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
)

// ProgressTransaction 是HeartbeatJob已经核对owner、epoch与fence的业务事务。
type ProgressTransaction = sql.Tx

// ProgressCheckpoint 只允许安全阶段代码和计数，不能携带正文或模型输出。
type ProgressCheckpoint struct {
	CheckpointKey  platform.Identifier `json:"checkpointKey"`
	Phase          string              `json:"phase"`
	CompletedUnits int64               `json:"completedUnits"`
	TotalUnits     int64               `json:"totalUnits"`
}

type AppendProgressInput struct {
	JobID platform.Identifier
	ProgressCheckpoint
}

type AppendProgressResult struct{ Seq platform.Revision }

// AppendProgress 在调用方已验租约的同一事务内分配seq并保存安全进度与outbox。
func AppendProgress(ctx context.Context, op OperationContext, tx *ProgressTransaction, in AppendProgressInput) (AppendProgressResult, error) {
	if in.CompletedUnits > in.TotalUnits {
		return AppendProgressResult{}, platform.NewError(platform.CodeValidation, false, "进度完成数量不能超过总数量")
	}
	request, _ := json.Marshal(in.ProgressCheckpoint)
	hash := platform.HashBytes(request)
	var existingHash string
	var seq int64
	err := tx.QueryRowContext(ctx, `SELECT input_hash,seq FROM progress WHERE job_id=? AND checkpoint_key=?`, in.JobID, in.CheckpointKey).Scan(&existingHash, &seq)
	if err == nil {
		if existingHash != hash {
			return AppendProgressResult{}, platform.NewError(platform.CodeConflict, false, "进度检查点已绑定不同输入")
		}
		return AppendProgressResult{Seq: seq}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return AppendProgressResult{}, mapQueueDBError(err)
	}
	var completed int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0),COALESCE(MAX(CASE WHEN phase=? THEN completed_units END),0)
		FROM progress WHERE job_id=?`, in.Phase, in.JobID).Scan(&seq, &completed)
	if err != nil {
		return AppendProgressResult{}, mapQueueDBError(err)
	}
	if in.CompletedUnits < completed {
		return AppendProgressResult{}, platform.NewError(platform.CodeConflict, false, "同阶段进度不能倒退")
	}
	seq++
	at := platform.Now()
	_, err = tx.ExecContext(ctx, `INSERT INTO progress(job_id,seq,phase,completed_units,total_units,safe_message_code,checkpoint_key,input_hash,created_at)
		VALUES(?,?,?,?,?,?,?,?,?)`, in.JobID, seq, in.Phase, in.CompletedUnits, in.TotalUnits, in.Phase, in.CheckpointKey, hash, at)
	if err != nil {
		return AppendProgressResult{}, mapQueueDBError(err)
	}
	payload, err := json.Marshal(struct {
		JobID          platform.Identifier `json:"jobId"`
		Seq            int64               `json:"seq"`
		Phase          string              `json:"phase"`
		CompletedUnits int64               `json:"completedUnits"`
		TotalUnits     int64               `json:"totalUnits"`
	}{in.JobID, seq, in.Phase, in.CompletedUnits, in.TotalUnits})
	if err != nil {
		return AppendProgressResult{}, platform.NewError(platform.CodeInternal, false, "任务进度事件编码失败")
	}
	if err := outbox.EnqueueEvent(tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: "jobs.job-progress.v1", EventVersion: 1,
		Producer: "jobs", SubjectID: in.JobID, Revision: seq, OccurredAt: at, DataEpoch: op.DataEpoch, Payload: payload}); err != nil {
		return AppendProgressResult{}, err
	}
	return AppendProgressResult{Seq: seq}, nil
}
