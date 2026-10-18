package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelclient"
	"time"
)

// ObserveOwnerModelSteps 只在SQL锁外有界观察本域原调用；不调用可Create的InvokeModel，不续付费或取消模型。
func ObserveOwnerModelSteps(ctx context.Context, op OperationContext, objectID string) error {
	runtime, ok := ctx.Value(modelRuntimeKey{}).(ModelRuntime)
	if !ok || runtime.Gateway == nil || runtime.DB == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "owner模型只读观察尚未装配")
	}
	rows, err := runtime.DB.QueryContext(ctx, `SELECT m.owner_id,m.step_key,m.operation_id,m.invocation_id,m.input_hash,m.request_json,m.request_hash,COALESCE((SELECT operation_json FROM build_dispatches WHERE build_id=m.owner_id),(SELECT operation_json FROM extraction_dispatches WHERE extraction_id=m.owner_id)) FROM model_steps m WHERE m.site_id=? AND m.data_epoch=? AND (m.owner_id=? OR m.owner_id IN (SELECT o.extraction_id FROM extraction_origins o WHERE o.build_id=? AND NOT EXISTS(SELECT 1 FROM execution_records f WHERE f.object_id=o.extraction_id AND f.job_type='knowledge.extract' AND f.state='completed' AND f.result_ref_json IS NOT NULL) AND NOT EXISTS(SELECT 1 FROM extraction_recovered_results r WHERE r.extraction_id=o.extraction_id))) AND m.dispatch_started=1 AND (m.state NOT IN ('completed','failed','cancelled') OR m.result_json IS NULL OR m.observation_json IS NULL) ORDER BY m.created_at,m.owner_id,m.step_key LIMIT 17`, op.Actor.SiteID, op.DataEpoch, objectID, objectID)
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原模型待观察事实不可读")
	}
	type pending struct {
		owner, key, operation, inputHash, requestHash string
		invocation                                    sql.NullString
		request, metadata                             []byte
	}
	pendingSteps := []pending{}
	for rows.Next() {
		var value pending
		if err = rows.Scan(&value.owner, &value.key, &value.operation, &value.invocation, &value.inputHash, &value.request, &value.requestHash, &value.metadata); err != nil {
			rows.Close()
			return platform.NormalizeIOError(err, platform.CodeInternal, false, "原模型待观察事实不可读")
		}
		pendingSteps = append(pendingSteps, value)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原模型待观察事实不可读")
	}
	if len(pendingSteps) > 16 {
		pendingSteps = pendingSteps[:16]
	} // 每次最多16项实际观察；完整SQL未决集合仍阻止错误safeToRetry。
	for _, step := range pendingSteps {
		var original OperationContext
		if json.Unmarshal(step.metadata, &original) != nil || original.CostClass != platform.CostClassBackground || original.Actor.SiteID != op.Actor.SiteID || original.DataEpoch != op.DataEpoch || original.OperationID != step.operation {
			return platform.NewError(platform.CodeIntegrityError, false, "原模型观察缺真实背景批准来源")
		}
		original.CallerService = "knowledge"
		original.Actor = platform.AuthContext{SubjectID: "knowledge", Kind: "service", SiteID: op.Actor.SiteID, ExpiresAt: original.Deadline, Grants: []string{}}
		var snapshot platform.GatewayInvocationSnapshot
		if step.invocation.Valid && step.invocation.String != "" {
			snapshot, err = modelclient.ObserveInvocation(ctx, runtime.Gateway, original, step.invocation.String)
		} else if len(step.request) > 0 {
			var request platform.CreateInvocationInput
			if len(step.request) > 16<<20 || platform.HashBytes(step.request) != step.requestHash || json.Unmarshal(step.request, &request) != nil || request.AdmissionPermit != nil || request.IDempotencyKey != step.key {
				return platform.NewError(platform.CodeIntegrityError, false, "原背景模型请求完整性损坏")
			}
			semantic := request
			semantic.IDempotencyKey = ""
			raw, hashErr := platform.CanonicalJSON(struct {
				Request         platform.CreateInvocationInput
				Root, CostClass string
			}{semantic, original.RootOperationID, original.CostClass})
			if hashErr != nil || platform.HashBytes(raw) != step.inputHash {
				return platform.NewError(platform.CodeIntegrityError, false, "原背景请求与稳定语义摘要不一致")
			}
			snapshot, err = modelclient.LookupModelInvocation(ctx, runtime.Gateway, original, platform.InvokeModelRequest{Request: request, IDempotencyKey: step.key})
		} else {
			continue
		} // 旧无ID/完整body不能猜，继续pending。
		if err != nil {
			var failure *platform.Error
			if errors.As(err, &failure) && failure.Code == platform.CodeInvocationNotAccepted && (!step.invocation.Valid || step.invocation.String == "") {
				local := runtime
				local.OwnerID = step.owner
				if err = RecordModelNonAcceptance(ctx, original, local, step.key, step.inputHash); err != nil {
					return err
				}
				continue
			}
			return err
		}
		if snapshot.Invocation.RootOperationID != original.RootOperationID || snapshot.Invocation.CostClass != original.CostClass || (step.invocation.Valid && step.invocation.String != "" && snapshot.Invocation.InvocationID != step.invocation.String) {
			return platform.NewError(platform.CodeIntegrityError, false, "原模型只读观察作用域不一致")
		}
		state := snapshot.Invocation.State
		switch state {
		case platform.InvocationSucceeded:
			state = "completed"
		case platform.InvocationChargeUnknown:
			state = "charge_unknown"
		case "reconciled":
			state = "failed"
		}
		if snapshot.Output != nil {
			state = "completed"
		}
		local := runtime
		local.OwnerID = step.owner
		outcome := InvokeModelResult{InvocationID: snapshot.Invocation.InvocationID, State: state, CurrentSnapshot: &snapshot}
		if err = RecordModelObservation(ctx, original, local, step.key, outcome); err != nil {
			return err
		}
	}
	return nil
}

// ModelNonAcceptance 仅证明某次真实只读查找时该原键未接受，不表示永久无费用或允许新owner。
type ModelNonAcceptance struct {
	FormatVersion   int    `json:"formatVersion"`
	Code            string `json:"code"`
	OwnerID         string `json:"ownerId"`
	Key             string `json:"key"`
	InputHash       string `json:"inputHash"`
	OperationID     string `json:"operationId"`
	RootOperationID string `json:"rootOperationId"`
	SiteID          string `json:"siteId"`
	DataEpoch       string `json:"dataEpoch"`
	ObservedAt      string `json:"observedAt"`
}

func RecordModelNonAcceptance(ctx context.Context, op OperationContext, runtime ModelRuntime, key, inputHash string) error {
	control, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	deadline, _ := control.Deadline()
	metadata := op
	metadata.Deadline = deadline.UTC().Format(time.RFC3339Nano)
	metadata.Actor.ExpiresAt = metadata.Deadline
	proof := ModelNonAcceptance{1, platform.CodeInvocationNotAccepted, runtime.OwnerID, key, inputHash, op.OperationID, op.RootOperationID, op.Actor.SiteID, op.DataEpoch, platform.Now()}
	raw, err := platform.CanonicalJSON(proof)
	if err != nil {
		return err
	}
	work, tx, closeWrite, err := beginModelWrite(platform.WithPortalProof(control, ""), metadata, runtime)
	if err != nil {
		return err
	}
	defer closeWrite()
	defer tx.Rollback()
	_, err = tx.ExecContext(work, `UPDATE model_steps SET state='not_accepted',not_accepted_json=?,not_accepted_hash=? WHERE site_id=? AND data_epoch=? AND owner_id=? AND step_key=? AND operation_id=? AND input_hash=? AND (invocation_id IS NULL OR invocation_id='') AND result_json IS NULL`, raw, platform.HashBytes(raw), op.Actor.SiteID, op.DataEpoch, runtime.OwnerID, key, op.OperationID, inputHash)
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原键未接受专用观察保存失败")
	}
	if err = tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原键未接受专用观察提交失败")
	}
	return nil
}

// ValidateModelNonAcceptance 只核真实保存的专用码/原scope，不从普通404、金额0或无ID推断。
func ValidateModelNonAcceptance(raw []byte, hash, site, epoch, owner, key, operation, root, inputHash string) error {
	var proof ModelNonAcceptance
	if platform.HashBytes(raw) != hash || json.Unmarshal(raw, &proof) != nil || proof.FormatVersion != 1 || proof.Code != platform.CodeInvocationNotAccepted || proof.SiteID != site || proof.DataEpoch != epoch || proof.OwnerID != owner || proof.Key != key || proof.OperationID != operation || proof.InputHash != inputHash || proof.RootOperationID != root || root == "" {
		return platform.NewError(platform.CodeIntegrityError, false, "原键未接受专用观察完整性损坏")
	}
	when, err := platform.ParseTimestamp(proof.ObservedAt)
	if err != nil || when.After(time.Now()) {
		return platform.NewError(platform.CodeIntegrityError, false, "原键未接受观察时刻损坏")
	}
	return nil
}
