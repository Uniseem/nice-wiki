package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"time"
)

// WithModelOwner 将本次已获准构建ID绑定到请求独立的付费结果记录；不改变root或费用类别。
func WithModelOwner(ctx context.Context, ownerID string) (context.Context, error) {
	runtime, ok := ctx.Value(modelRuntimeKey{}).(ModelRuntime)
	if !ok || ownerID == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "模型步骤来源未装配")
	}
	runtime.OwnerID = ownerID
	return WithModelRuntime(ctx, runtime), nil
}

func beginModelWrite(ctx context.Context, op OperationContext, runtime ModelRuntime) (context.Context, *sql.Tx, context.CancelFunc, error) {
	if runtime.DB == nil || runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return nil, nil, nil, platform.NewError(platform.CodeDependencyUnavailable, false, "模型步骤持久存储未装配")
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return nil, nil, nil, err
	}
	if view.DataEpoch != op.DataEpoch {
		cancel()
		return nil, nil, nil, platform.NewError(platform.CodeStaleResult, false, "模型步骤数据代次已变化")
	}
	tx, err := runtime.DB.BeginTx(work, nil)
	if err != nil {
		cancel()
		return nil, nil, nil, platform.NormalizeIOError(err, platform.CodeInternal, false, "模型步骤事务失败")
	}
	var site, epoch string
	if err = tx.QueryRowContext(work, `SELECT site_id,data_epoch FROM index_identity WHERE singleton=1`).Scan(&site, &epoch); err != nil || site != op.Actor.SiteID || epoch != op.DataEpoch {
		tx.Rollback()
		cancel()
		return nil, nil, nil, platform.NewError(platform.CodeStaleResult, false, "模型步骤私有存储身份不一致")
	}
	return work, tx, cancel, nil
}

// BeginModelStep 先保存不可变原请求摘要和原op/key；接受未知也只允许相同key找回原MG记录。
func BeginModelStep(ctx context.Context, op OperationContext, runtime ModelRuntime, key, semantic string, request platform.CreateInvocationInput) (string, InvokeModelResult, bool, error) {
	request.AdmissionPermit = nil
	legacyRaw, err := platform.CanonicalJSON(struct {
		Request         platform.CreateInvocationInput
		Root, CostClass string
	}{request, op.RootOperationID, op.CostClass})
	if err != nil {
		return "", InvokeModelResult{}, false, err
	}
	request.IDempotencyKey = "" // 实际RPC字段由公共网关适配改为持久原key，内部临时hash不属于请求语义。
	raw, err := platform.CanonicalJSON(struct {
		Request         platform.CreateInvocationInput
		Root, CostClass string
	}{request, op.RootOperationID, op.CostClass})
	if err != nil {
		return "", InvokeModelResult{}, false, err
	}
	work, tx, cancel, err := beginModelWrite(ctx, op, runtime)
	if err != nil {
		return "", InvokeModelResult{}, false, err
	}
	defer cancel()
	defer tx.Rollback()
	owner := runtime.OwnerID
	if owner == "" {
		owner = op.RootOperationID
	}
	var resolved string
	err = tx.QueryRowContext(work, `SELECT step_key FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND semantic_key=?`, op.Actor.SiteID, op.DataEpoch, owner, semantic).Scan(&resolved)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(work, `SELECT step_key FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND semantic_key='' AND operation_id=? AND input_hash IN (?,?)`, op.Actor.SiteID, op.DataEpoch, owner, op.OperationID, platform.HashBytes(raw), platform.HashBytes(legacyRaw)).Scan(&resolved)
		if err == nil {
			_, err = tx.ExecContext(work, `UPDATE model_steps SET semantic_key=?,input_hash=? WHERE site_id=? AND data_epoch=? AND owner_id=? AND step_key=? AND semantic_key=''`, semantic, platform.HashBytes(raw), op.Actor.SiteID, op.DataEpoch, owner, resolved)
		} else if err == sql.ErrNoRows {
			var legacy int
			err = tx.QueryRowContext(work, `SELECT COUNT(*) FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND semantic_key=''`, op.Actor.SiteID, op.DataEpoch, owner).Scan(&legacy)
			if err == nil && legacy > 0 {
				return "", InvokeModelResult{}, false, platform.NewError(platform.CodeConflict, false, "旧模型步骤语义无法证明，保留原结果不自动重建")
			}
			if err == nil {
				resolved = key
				_, err = tx.ExecContext(work, `INSERT INTO model_steps(site_id,data_epoch,owner_id,step_key,semantic_key,input_hash,operation_id,state,created_at,dispatch_started) VALUES(?,?,?,?,?,?,?,'prepared',?,0)`, op.Actor.SiteID, op.DataEpoch, owner, resolved, semantic, platform.HashBytes(raw), op.OperationID, platform.Now())
			}
		}
	}
	if err != nil {
		return "", InvokeModelResult{}, false, platform.NormalizeIOError(err, platform.CodeInternal, false, "模型稳定语义原意图读取或保存失败")
	}
	var hash, operation, resultHash, state, observationHash string
	var result, observation []byte
	err = tx.QueryRowContext(work, `SELECT input_hash,operation_id,result_json,result_hash,state,observation_json,observation_hash FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND step_key=?`, op.Actor.SiteID, op.DataEpoch, owner, resolved).Scan(&hash, &operation, &result, &resultHash, &state, &observation, &observationHash)
	if err != nil {
		return "", InvokeModelResult{}, false, platform.NormalizeIOError(err, platform.CodeInternal, false, "模型步骤原记录读取失败")
	}
	if hash != platform.HashBytes(raw) || operation != op.OperationID {
		return "", InvokeModelResult{}, false, platform.NewError(platform.CodeConflict, false, "同语义模型步骤原输入或配置已变化")
	}
	if op.CostClass == platform.CostClassBackground && runtime.OwnerID != "" {
		// 背景原文/记忆已在原任务冻结；实际key写入完整DTO，QA问题及permit始终不持久。
		durable := request
		durable.IDempotencyKey = resolved
		durable.AdmissionPermit = nil
		requestRaw, err := platform.CanonicalJSON(durable)
		if err != nil {
			return "", InvokeModelResult{}, false, err
		}
		if len(requestRaw) > 16<<20 {
			return "", InvokeModelResult{}, false, platform.NewError(platform.CodeBudgetExceeded, false, "原背景模型完整请求超过16MiB读取预算")
		}
		if _, err = tx.ExecContext(work, `UPDATE model_steps SET request_json=?,request_hash=? WHERE site_id=? AND data_epoch=? AND owner_id=? AND step_key=? AND operation_id=? AND dispatch_started=0 AND request_json IS NULL`, requestRaw, platform.HashBytes(requestRaw), op.Actor.SiteID, op.DataEpoch, owner, resolved, op.OperationID); err != nil {
			return "", InvokeModelResult{}, false, platform.NormalizeIOError(err, platform.CodeInternal, false, "原背景模型请求冻结失败")
		}
	}
	var original InvokeModelResult
	if len(result) > 0 && (resultHash != platform.HashBytes(result) || json.Unmarshal(result, &original) != nil || original.InvocationID == "") {
		return "", InvokeModelResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "模型付费原结果损坏")
	}
	if len(observation) > 0 {
		var observed InvokeModelResult
		if observationHash != platform.HashBytes(observation) || json.Unmarshal(observation, &observed) != nil || observed.InvocationID == "" || (original.InvocationID != "" && observed.InvocationID != original.InvocationID) {
			return "", InvokeModelResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "原模型最新真实观察完整性损坏")
		}
		if HasCompleteResult(observed) || !HasCompleteResult(original) {
			original = observed
		}
	}
	if err = tx.Commit(); err != nil {
		return "", InvokeModelResult{}, false, platform.NormalizeIOError(err, platform.CodeInternal, false, "模型步骤意图提交失败")
	}
	return resolved, original, len(result) > 0 && (state == "failed" || state == "cancelled" || (state == "completed" && HasCompleteResult(original))), nil
}

// SaveModelProgress 每个真实MG事件先保留调用引用，再转给UI回调；不保存prompt或许可。
func SaveModelProgress(ctx context.Context, op OperationContext, runtime ModelRuntime, key string, event platform.StreamEvent) error {
	if event.Invocation == nil {
		return nil
	}
	invocation := event.Invocation
	if invocation.RootOperationID != op.RootOperationID || invocation.CostClass != op.CostClass {
		return platform.NewError(platform.CodeIntegrityError, false, "模型调用原根或费用类别不一致")
	}
	work, tx, cancel, err := beginModelWrite(ctx, op, runtime)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	owner := runtime.OwnerID
	if owner == "" {
		owner = op.RootOperationID
	}
	var stored sql.NullString
	err = tx.QueryRowContext(work, `SELECT invocation_id FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND step_key=? AND operation_id=?`, op.Actor.SiteID, op.DataEpoch, owner, key, op.OperationID).Scan(&stored)
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "模型引用原意图不存在")
	}
	if stored.Valid && stored.String != "" && stored.String != invocation.InvocationID {
		return platform.NewError(platform.CodeIntegrityError, false, "模型步骤出现不同调用引用")
	}
	_, err = tx.ExecContext(work, `UPDATE model_steps SET invocation_id=?,state=CASE WHEN result_json IS NULL THEN 'running' ELSE state END WHERE site_id=? AND data_epoch=? AND owner_id=? AND step_key=? AND operation_id=?`, invocation.InvocationID, op.Actor.SiteID, op.DataEpoch, owner, key, op.OperationID)
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "模型调用引用保存失败")
	}
	if err = tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "模型调用引用提交失败")
	}
	return nil
}

// CompleteModelStep 所有终态原样保存，失败或费用未知重放也不新建调用。
func CompleteModelStep(ctx context.Context, op OperationContext, runtime ModelRuntime, key string, outcome InvokeModelResult) error {
	if outcome.InvocationID == "" {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "模型观察没有真实已接受调用ID")
	}
	if outcome.Events == nil {
		outcome.Events = []platform.StreamEvent{}
	}
	raw, err := platform.CanonicalJSON(outcome)
	if err != nil {
		return err
	}
	observation := raw
	if len(raw) > 16<<20 {
		return platform.NewError(platform.CodeBudgetExceeded, false, "模型原结果超出私有读取预算")
	}
	work, tx, cancel, err := beginModelWrite(ctx, op, runtime)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	owner := runtime.OwnerID
	if owner == "" {
		owner = op.RootOperationID
	}
	var invocation sql.NullString
	var storedHash string
	var storedResult []byte
	err = tx.QueryRowContext(work, `SELECT invocation_id,result_hash,result_json FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND step_key=? AND operation_id=?`, op.Actor.SiteID, op.DataEpoch, owner, key, op.OperationID).Scan(&invocation, &storedHash, &storedResult)
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "模型结果原意图不存在")
	}
	if invocation.Valid && invocation.String != "" && invocation.String != outcome.InvocationID {
		return platform.NewError(platform.CodeIntegrityError, false, "模型步骤调用引用冲突")
	}
	if storedHash != "" && storedHash != platform.HashBytes(raw) {
		var prior InvokeModelResult
		if storedHash != platform.HashBytes(storedResult) || json.Unmarshal(storedResult, &prior) != nil || prior.InvocationID != outcome.InvocationID {
			return platform.NewError(platform.CodeIntegrityError, false, "原模型观察结果损坏或调用身份变化")
		}
		if HasCompleteResult(prior) {
			if HasCompleteResult(outcome) {
				oldSnapshot, _ := WinningModelSnapshot(prior)
				newSnapshot, err := WinningModelSnapshot(outcome)
				if err != nil || oldSnapshot != newSnapshot {
					return platform.NewError(platform.CodeIntegrityError, false, "原调用对账改变实际结果来源")
				}
				if prior.CurrentSnapshot != nil && prior.CurrentSnapshot.Output != nil && outcome.CurrentSnapshot != nil && outcome.CurrentSnapshot.Output != nil {
					before, _ := platform.CanonicalJSON(prior.CurrentSnapshot.Output)
					after, _ := platform.CanonicalJSON(outcome.CurrentSnapshot.Output)
					if string(before) != string(after) {
						return platform.NewError(platform.CodeIntegrityError, false, "原调用完整付费输出不可被对账改写")
					}
				} else if outcome.CurrentSnapshot != nil && outcome.CurrentSnapshot.Output != nil {
					output := outcome.CurrentSnapshot.Output
					if output.Text != nil {
						oldText, err := CompletedText(prior, 65536)
						if err != nil || oldText != *output.Text {
							return platform.NewError(platform.CodeIntegrityError, false, "原完整文字与当前输出不一致")
						}
					} else {
						oldOutput, err := CompletedOutput(prior, output.Capability)
						if err != nil {
							return err
						}
						before, _ := platform.CanonicalJSON(oldOutput)
						after, _ := platform.CanonicalJSON(output)
						if string(before) != string(after) {
							return platform.NewError(platform.CodeIntegrityError, false, "原结构结果与当前输出不一致")
						}
					}
				}
			}
			raw = storedResult // 业务结果不可变；最新真实观察与费用状态另存。
		}
	}
	projected := ModelObservationState(outcome)
	_, err = tx.ExecContext(work, `UPDATE model_steps SET invocation_id=?,state=?,result_json=?,result_hash=?,observation_json=?,observation_hash=?,not_accepted_json=NULL,not_accepted_hash='' WHERE site_id=? AND data_epoch=? AND owner_id=? AND step_key=? AND operation_id=?`, outcome.InvocationID, projected, raw, platform.HashBytes(raw), observation, platform.HashBytes(observation), op.Actor.SiteID, op.DataEpoch, owner, key, op.OperationID)
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "模型原结果保存失败")
	}
	if err = tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "模型原结果提交失败")
	}
	return nil
}

// MarkModelDispatch 在真实租约门禁后、外部RPC前持久标记；崩溃只能保守unknown，不能靠无ID判零费用。
func MarkModelDispatch(ctx context.Context, op OperationContext, runtime ModelRuntime, key string) error {
	work, tx, cancel, err := beginModelWrite(ctx, op, runtime)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	owner := runtime.OwnerID
	if owner == "" {
		owner = op.RootOperationID
	}
	changed, err := tx.ExecContext(work, `UPDATE model_steps SET dispatch_started=1,state=CASE WHEN dispatch_started=0 OR state='not_accepted' THEN 'dispatching' ELSE state END,not_accepted_json=NULL,not_accepted_hash='' WHERE site_id=? AND data_epoch=? AND owner_id=? AND step_key=? AND operation_id=?`, op.Actor.SiteID, op.DataEpoch, owner, key, op.OperationID)
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原模型派发事实保存失败")
	}
	count, err := changed.RowsAffected()
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原模型派发事实不可读")
	}
	if count != 1 {
		return platform.NewError(platform.CodeStaleResult, false, "原模型派发意图已失效")
	}
	if err = tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原模型派发事实提交失败")
	}
	return nil
}

// RecordModelObservation 保存平台实际接受ID/当前对象，即使原请求已取消也只做短元数据写，不再调用模型。
func RecordModelObservation(ctx context.Context, op OperationContext, runtime ModelRuntime, key string, outcome InvokeModelResult) error {
	control, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	deadline, _ := control.Deadline()
	metadata := op
	metadata.Deadline = deadline.UTC().Format(time.RFC3339Nano)
	metadata.Actor.ExpiresAt = metadata.Deadline
	return CompleteModelStep(platform.WithPortalProof(control, ""), metadata, runtime, key, outcome)
}
