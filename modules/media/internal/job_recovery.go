package media

import (
	"context"
	"database/sql"

	"nice-wiki/libs/platform"
)

type ReadJobRecoveryInput = platform.OwnerRecoveryRequest
type ReadJobRecoveryResult = platform.OwnerRecoveryResult

// ReadJobRecovery 只给 jobs 证明原清理任务的状态：已有删除结果即完成；否则执行可从原决定续做或尚未开始，安全重排。
func ReadJobRecovery(ctx context.Context, op OperationContext, input ReadJobRecoveryInput) (ReadJobRecoveryResult, error) {
	result := ReadJobRecoveryResult{Status: "unknown", PaidResultRefs: []string{}}
	if op.CallerService != "jobs" || op.Actor.Kind != "service" || input.Owner != "media" || input.Type != "media.cleanup" {
		return result, platform.NewError(platform.CodeForbidden, false, "恢复证明只允许jobs读取本域原任务")
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return result, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体清理存储未装配")
	}
	if input.DataEpoch != op.DataEpoch || op.DataEpoch != s.openedEpoch {
		return result, platform.NewError(platform.CodeStaleResult, false, "恢复请求不是当前数据代次")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, mapMediaDBError(err, "开始恢复证明只读快照失败")
	}
	defer tx.Rollback()
	approval, found, err := s.loadCleanupApproval(ctx, tx, input.InputRef.ObjectID)
	if err != nil {
		return result, err
	}
	if !found {
		return result, platform.NewError(platform.CodeIntegrityError, false, "恢复请求的原清理批准不存在")
	}
	binding := OperationContext{DataEpoch: op.DataEpoch, OperationID: approval.Approval.OperationId, RootOperationID: input.RootOperationID, Actor: platform.AuthContext{SiteID: op.Actor.SiteID}}
	job := platform.JobExecutionInput{JobID: input.JobID, JobType: input.Type, InputRef: input.InputRef, DataEpoch: input.DataEpoch}
	if err := checkCleanupJobBinding(binding, approval, job); err != nil {
		return result, err
	}
	record, decided, err := s.readCleanupExecution(ctx, tx, approval)
	if err != nil {
		return result, err
	}
	if decided && record.Result != nil {
		result.Status = "completed"
		return result, nil
	}
	result.Status = "safeToRetry"
	return result, nil
}
