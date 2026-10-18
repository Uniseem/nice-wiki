package invocation

import (
	"context"
	"database/sql"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

type LookupInvocationInput = gen.CreateInvocationInput
type LookupInvocationResult = GetInvocationResult

// RequireInvocationLookupTracking只读既有可信初始化来源；绝不从空库创建“未接受”的证明。
func RequireInvocationLookupTracking(tx *gatewayruntime.ExecutionTransaction, op OperationContext) error {
	var version int
	var epoch, kind, proof string
	err := tx.Tx.QueryRowContext(tx.Context, `SELECT format_version,data_epoch,proof_kind,proof_hash FROM invocation_lookup_tracking WHERE singleton=1`).Scan(&version, &epoch, &kind, &proof)
	if err == sql.ErrNoRows {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "原调用完整tracking来源尚未建立，不能证明从未接受")
	}
	if err != nil {
		return gatewayruntime.ExecutionStorageError(err)
	}
	if version != 1 || epoch != op.DataEpoch || (kind != "fresh_activation" && kind != "verified_restore") || !platform.ValidHash(proof) {
		return platform.NewError(platform.CodeIntegrityError, false, "原调用tracking来源或代际证据不合法")
	}
	var unknown bool
	err = tx.Tx.QueryRowContext(tx.Context, `SELECT EXISTS(SELECT 1 FROM invocations WHERE owner_service IS NULL OR owner_service='' OR site_id IS NULL OR site_id='' OR data_epoch='' OR (owner_service=? AND site_id=? AND ((data_epoch<>? AND NOT EXISTS(SELECT 1 FROM restored_epoch_sources s WHERE s.source_epoch=invocations.data_epoch AND s.target_epoch=?)) OR create_command_key IS NULL OR create_command_key='' OR execution_hash IS NULL OR execution_hash='')))`, op.CallerService, op.Actor.SiteID, op.DataEpoch,op.DataEpoch).Scan(&unknown)
	if err != nil {
		return gatewayruntime.ExecutionStorageError(err)
	}
	if unknown {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "旧调用缺完整创建键/归属或恢复投影证据，不能返回未找到")
	}
	return nil
}

func LookupInvocation(ctx context.Context, op OperationContext, input LookupInvocationInput) (LookupInvocationResult, error) {
	task, exists := gen.AITaskCatalog[input.TaskId]
	if !exists || task.Owner != op.CallerService || op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService {
		return LookupInvocationResult{}, platform.NewError(platform.CodeForbidden, false, "原调用查找仅AI任务owner纯服务")
	}
	if op.CostClass != platform.CostClassQA && op.CostClass != platform.CostClassBackground && !(input.TaskId=="memory.assistant"&&op.CallerService=="memory"&&op.CostClass==platform.CostClassAdminProbe) {
		return LookupInvocationResult{}, platform.NewError(platform.CodeForbidden, false, "原调用查找须保留原费用类别")
	}
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return LookupInvocationResult{}, err
	}
	defer tx.Close()
	canonical := CanonicalInvocationInput(op, input)
	var receipt CreateInvocationResult
	found, err := gatewayruntime.ReplayExecutionReceipt(tx, op, "model-gateway.CreateInvocation", input.IdempotencyKey, canonical, &receipt)
	if err != nil {
		return LookupInvocationResult{}, err
	}
	if !found {
		var orphan bool
		err := tx.Tx.QueryRowContext(tx.Context, `SELECT EXISTS(SELECT 1 FROM invocations WHERE owner_service=? AND site_id=? AND data_epoch=? AND create_command_key=?)`, op.CallerService, op.Actor.SiteID, op.DataEpoch, input.IdempotencyKey).Scan(&orphan)
		if err != nil {
			return LookupInvocationResult{}, gatewayruntime.ExecutionStorageError(err)
		}
		if orphan {
			return LookupInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "创建键已有原调用但完整回执缺失，禁止重新收费")
		}
 var previous bool
  if err:=tx.Tx.QueryRowContext(tx.Context,`SELECT EXISTS(SELECT 1 FROM invocations i JOIN restored_epoch_sources s ON s.source_epoch=i.data_epoch WHERE i.owner_service=? AND i.site_id=? AND i.create_command_key=? AND s.target_epoch=?)`,op.CallerService,op.Actor.SiteID,input.IdempotencyKey,op.DataEpoch).Scan(&previous);err!=nil{return LookupInvocationResult{},gatewayruntime.ExecutionStorageError(err)};if previous{return LookupInvocationResult{},platform.NewError(platform.CodeStaleResult,false,"原创建键存在已接受旧代调用，须读取原付费引用，不得改代重付")}
		if err := RequireInvocationLookupTracking(tx, op); err != nil {
			return LookupInvocationResult{}, err
		}
		return LookupInvocationResult{}, platform.NewError(platform.CodeInvocationNotAccepted, false, "完整tracking确认原键尚无已接受调用")
	}
	record, err := gatewayruntime.ReadInvocationRecord(tx, op, receipt.Invocation.InvocationId)
	if err != nil {
		if ErrorCode(err) == platform.CodeNotFound {
			return LookupInvocationResult{}, platform.NewError(platform.CodeIntegrityError, false, "原创建回执引用的调用事实缺失")
		}
		return LookupInvocationResult{}, err
	}
	original, _ := platform.CanonicalJSON(CanonicalInvocationInput(record.Operation, record.Request))
	expected, _ := platform.CanonicalJSON(canonical)
	if record.Request.IdempotencyKey != input.IdempotencyKey || platform.HashBytes(original) != platform.HashBytes(expected) {
		return LookupInvocationResult{}, platform.NewError(platform.CodeConflict, false, "原键对应的完整语义输入不一致")
	}
	return ReadInvocationSnapshot(tx, record)
}
