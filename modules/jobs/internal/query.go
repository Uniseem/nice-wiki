package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/jobs/internal/gen"
)

type ReadJobInput struct {
	JobID     platform.Identifier
	Actor     platform.AuthContext
	DataEpoch platform.Identifier
}

type ReadJobResult = gen.ReadJobResult

// ReadJob 在最新功能授权下读取同一代次的安全状态及已有结果引用。
func ReadJob(ctx context.Context, op OperationContext, in ReadJobInput) (ReadJobResult, error) {
	authorized, err := authorizeJobManagement(ctx, op, in.JobID)
	if err != nil {
		return ReadJobResult{}, err
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return ReadJobResult{}, err
	}
	defer work.close()
	job, err := work.readJobRecord(ctx, in.JobID, in.DataEpoch)
	if err != nil {
		return ReadJobResult{}, err
	}
	if job.owner != authorized.owner || job.jobType != authorized.jobType || job.epoch != authorized.epoch {
		return ReadJobResult{}, platform.NewError(platform.CodeConflict, false, "任务授权作用域已变化")
	}
	result := ReadJobResult{State: job.state, Revision: int(job.revision), ResultRefs: []string{}}
	var primary string
	var refs []byte
	err = work.tx.QueryRowContext(ctx, `SELECT result_ref,paid_result_refs_json FROM job_results WHERE job_id=?`, in.JobID).Scan(&primary, &refs)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return ReadJobResult{}, mapQueueDBError(err)
	}
	if err := json.Unmarshal(refs, &result.ResultRefs); err != nil {
		return ReadJobResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务付费结果引用损坏")
	}
	if primary != "" {
		var ref ObjectRef
		if err := json.Unmarshal([]byte(primary), &ref); err != nil {
			return ReadJobResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务结果引用损坏")
		}
		result.ResultRefs = append(result.ResultRefs, ref.ObjectId)
	}
	return result, nil
}

func authorizeJobManagement(ctx context.Context, op OperationContext, jobID string) (jobRecord, error) {
	store := currentStore()
	if store == nil {
		return jobRecord{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	store.mu.RLock()
	if store.closed || store.identity == nil {
		store.mu.RUnlock()
		return jobRecord{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务管理授权尚未装配")
	}
	job := jobRecord{id: jobID}
	err := store.db.QueryRowContext(ctx, `SELECT owner,type,data_epoch FROM jobs WHERE job_id=?`, jobID).Scan(&job.owner, &job.jobType, &job.epoch)
	client := store.identity
	store.mu.RUnlock()
	if errors.Is(err, sql.ErrNoRows) {
		return jobRecord{}, platform.NewError(platform.CodeNotFound, false, "任务不存在")
	}
	if err != nil {
		return jobRecord{}, mapQueueDBError(err)
	}
	if job.epoch != op.DataEpoch {
		return jobRecord{}, epochMismatch()
	}
	permission := ""
	for _, entry := range gen.JobTypes {
		if entry.Owner == job.owner && entry.Type == job.jobType {
			permission = entry.ManagementPermission
			break
		}
	}
	if permission == "" {
		return jobRecord{}, platform.NewError(platform.CodeIntegrityError, false, "任务类型缺少具体管理权限")
	}
	var sessionID *string
	if op.Actor.SessionID != "" {
		sessionID = &op.Actor.SessionID
	}
	body := struct {
		SubjectID                  string  `json:"subjectId"`
		SessionID                  *string `json:"sessionId"`
		SiteID                     string  `json:"siteId"`
		Permission                 string  `json:"permission"`
		ResourceID                 string  `json:"resourceId"`
		ExpectedPermissionRevision *int64  `json:"expectedPermissionRevision"`
	}{op.Actor.SubjectID, sessionID, op.Actor.SiteID, permission, jobID, nil}
	raw, err := client.CheckAuthorization(platform.WithOperationContext(ctx, op), body)
	if err != nil {
		return jobRecord{}, err
	}
	var decision struct {
		Authorized *bool `json:"authorized"`
	}
	if err := json.Unmarshal(raw, &decision); err != nil || decision.Authorized == nil {
		return jobRecord{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务管理授权响应不可用")
	}
	if !*decision.Authorized {
		return jobRecord{}, platform.NewError(platform.CodeForbidden, false, "没有该任务所属功能的管理权限")
	}
	return job, nil
}
