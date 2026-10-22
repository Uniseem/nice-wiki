package analytics

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"nice-wiki/libs/platform"
)

type ReadJobRecoveryInput = platform.OwnerRecoveryRequest
type ReadJobRecoveryResult = platform.OwnerRecoveryResult

// ReadJobRecovery 只给 jobs 证明原聚合任务能否重排：聚合按水位重算整段、不调模型，本地租约镜像过期即可安全重排。
func ReadJobRecovery(ctx context.Context, op OperationContext, input ReadJobRecoveryInput) (ReadJobRecoveryResult, error) {
	result := ReadJobRecoveryResult{Status: "unknown", PaidResultRefs: []string{}}
	if op.CallerService != "jobs" || op.Actor.Kind != "service" || input.Owner != "analytics" {
		return result, platform.NewError(platform.CodeForbidden, false, "恢复证明只允许jobs读取本域原任务")
	}
	if input.DataEpoch != op.DataEpoch {
		return result, platform.NewError(platform.CodeStaleResult, false, "恢复请求不是当前数据代次")
	}
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return result, err
	}
	var ref, root, kind, jobID, expires string
	err = rt.DB.QueryRowContext(ctx, `SELECT input_ref,root_operation_id,job_type,job_id,lease_expires_at FROM aggregate_jobs WHERE input_id=? AND data_epoch=?`, input.InputRef.ObjectID, op.DataEpoch).Scan(&ref, &root, &kind, &jobID, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return result, platform.NewError(platform.CodeIntegrityError, false, "恢复请求的原聚合任务不存在")
	}
	if err != nil {
		return result, storageError(err)
	}
	expected, _ := platform.CanonicalJSON(input.InputRef)
	if ref != string(expected) || root != input.RootOperationID || kind != input.Type || (jobID != "" && jobID != input.JobID) {
		return result, platform.NewError(platform.CodeIntegrityError, false, "恢复请求与原聚合任务输入不一致")
	}
	if at, err := time.Parse(analyticsTimeLayout, expires); err == nil && at.After(time.Now()) {
		result.Status = "pending"
		return result, nil
	}
	result.Status = "safeToRetry"
	return result, nil
}
