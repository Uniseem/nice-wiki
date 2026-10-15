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
	if err := authorizeJobManagement(ctx, op, in.JobID); err != nil {
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

func authorizeJobManagement(ctx context.Context, op OperationContext, jobID string) error {
	store := currentStore()
	if store == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	if store.closed || store.identity == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "任务管理授权尚未装配")
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
	}{op.Actor.SubjectID, sessionID, op.Actor.SiteID, "job.manage", jobID, nil}
	raw, err := store.identity.CheckAuthorization(platform.WithOperationContext(ctx, op), body)
	if err != nil {
		return err
	}
	var decision struct {
		Authorized *bool `json:"authorized"`
	}
	if err := json.Unmarshal(raw, &decision); err != nil || decision.Authorized == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "任务管理授权响应不可用")
	}
	if !*decision.Authorized {
		return platform.NewError(platform.CodeForbidden, false, "没有任务管理权限")
	}
	return nil
}
