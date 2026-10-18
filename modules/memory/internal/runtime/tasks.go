package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/memory/internal/gen"
	"time"
)

// AssistantSnapshot 为已人工发起任务固定输入和免费选择的实际模型档位。
type AssistantSnapshot struct {
	RunID            string                         `json:"runId"`
	SourceGeneration int                            `json:"sourceGeneration"`
	Entries          []gen.MemoryEntry              `json:"entries"`
	Request          platform.CreateInvocationInput `json:"request"`
}
type AssistantInputRecord struct {
	SiteID        string
	Operation     platform.OperationContext
	Snapshot      AssistantSnapshot
	Ref           platform.ObjectRef
	JobID         string
	DispatchState string
}

func ConfigureAssistantClients(jobs *gen.JobsClient, gateway *gen.ModelGatewayClient) error {
	s := CurrentStore()
	if s == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "记忆存储未打开")
	}
	s.jobs = jobs
	s.gatewayRaw = gateway
	return nil
}

// SelectAssistantProfile 免费读取已有配置，不修改任务分配或模型。
func SelectAssistantProfile(ctx context.Context, op OperationContext) (string, error) {
	s := CurrentStore()
	if s == nil || s.gatewayRaw == nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "记忆模型配置客户端未装配")
	}
	service := op
	service.CallerService = "memory"
	service.CostClass = platform.CostClassAdminProbe
	service.Actor = platform.AuthContext{Kind: "service", SubjectID: "memory", SiteID: op.Actor.SiteID, ExpiresAt: op.Deadline, Grants: []string{}}
	work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service)
	raw, err := s.gatewayRaw.ReadTaskAssignments(work, url.Values{"taskId": {"memory.assistant"}})
	if err != nil {
		return "", err
	}
	var assignments struct {
		Assignments []struct {
			TaskID string `json:"taskId"`
			Tier   string `json:"tier"`
		} `json:"assignments"`
	}
	if ValidateForeign("ReadTaskAssignmentsResult", raw) != nil || json.Unmarshal(raw, &assignments) != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "模型任务目录响应损坏")
	}
	tier := ""
	for _, a := range assignments.Assignments {
		if a.TaskID == "memory.assistant" {
			tier = a.Tier
		}
	}
	if tier != "accurate" {
		return "", platform.NewError(platform.CodeUnsupportedCapability, false, "记忆助手尚未配置精确模型")
	}
	raw, err = s.gatewayRaw.ReadProfiles(work)
	if err != nil {
		return "", err
	}
	var profiles struct {
		Profiles []struct {
			ProfileID string   `json:"profileId"`
			Tier      string   `json:"tier"`
			CostClass string   `json:"costClass"`
			Models    []string `json:"orderedModelIds"`
		} `json:"profiles"`
	}
	if ValidateForeign("ReadProfilesResult", raw) != nil || json.Unmarshal(raw, &profiles) != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "模型档位目录响应损坏")
	}
	id := ""
	for _, p := range profiles.Profiles {
		if p.Tier == tier && p.CostClass == service.CostClass && len(p.Models) > 0 {
			if id != "" {
				return "", platform.NewError(platform.CodeIntegrityError, false, "模型档位声明不唯一")
			}
			id = p.ProfileID
		}
	}
	if id == "" {
		return "", platform.NewError(platform.CodeUnsupportedCapability, false, "记忆助手尚未配置可用档位")
	}
	return id, nil
}
func SaveAssistantInput(ctx context.Context, tx *sql.Tx, op OperationContext, snapshot AssistantSnapshot) error {
	raw, err := platform.CanonicalJSON(snapshot)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "助手固定输入编码失败")
	}
	operation, _ := platform.CanonicalJSON(op)
	_, err = tx.ExecContext(ctx, `INSERT INTO assistant_inputs(run_id,site_id,operation_json,snapshot_json,input_hash,data_epoch,dispatch_state) VALUES(?,?,?,?,?,?,'pending')`, snapshot.RunID, op.Actor.SiteID, operation, raw, platform.HashBytes(raw), op.DataEpoch)
	return DBError(err)
}
func ReadAssistantInput(ctx context.Context, q Queryer, runID string) (AssistantInputRecord, error) {
	var r AssistantInputRecord
	var opRaw, raw []byte
	var digest, epoch string
	var job sql.NullString
	err := q.QueryRowContext(ctx, `SELECT site_id,operation_json,snapshot_json,input_hash,data_epoch,job_id,dispatch_state FROM assistant_inputs WHERE run_id=?`, runID).Scan(&r.SiteID, &opRaw, &raw, &digest, &epoch, &job, &r.DispatchState)
	if errors.Is(err, sql.ErrNoRows) {
		return r, platform.NewError(platform.CodeNotFound, false, "助手固定输入不存在")
	}
	if err != nil {
		return r, DBError(err)
	}
	if platform.HashBytes(raw) != digest || json.Unmarshal(opRaw, &r.Operation) != nil || json.Unmarshal(raw, &r.Snapshot) != nil || r.Snapshot.RunID != runID || r.Operation.Actor.SiteID != r.SiteID || r.Operation.DataEpoch != epoch {
		return r, platform.NewError(platform.CodeIntegrityError, false, "助手固定输入绑定损坏")
	}
	r.JobID = job.String
	r.Ref = platform.ObjectRef{Owner: "memory", ObjectID: runID, Version: 1, SHA256: digest, ByteSize: int64(len(raw)), MediaType: "application/json"}
	return r, nil
}

// DispatchAssistantInputs 固定原始任务/root，远程成功后才把本地待投递行置submitted。
func DispatchAssistantInputs(ctx context.Context, limit int) (int, error) {
	s := CurrentStore()
	if s == nil || s.jobs == nil {
		return 0, platform.NewError(platform.CodeDependencyUnavailable, true, "助手任务客户端未装配")
	}
	ids := []string{}
	rows, err := s.db.QueryContext(ctx, `SELECT run_id FROM assistant_inputs WHERE dispatch_state='pending' ORDER BY run_id LIMIT ?`, limit)
	if err != nil {
		return 0, DBError(err)
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, DBError(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, DBError(err)
	}
	count := 0
	for _, id := range ids {
		r, err := ReadAssistantInput(ctx, s.db, id)
		if err != nil {
			return count, err
		}
		op := r.Operation
		op.CallerService = "memory"
		op.Actor = platform.AuthContext{Kind: "service", SubjectID: "memory", SiteID: r.SiteID, ExpiresAt: time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano), Grants: []string{}}
		op.Deadline = op.Actor.ExpiresAt
		work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
		raw, err := s.jobs.CreateJob(work, struct {
			Owner string             `json:"owner"`
			Type  string             `json:"type"`
			Ref   platform.ObjectRef `json:"inputRef"`
			Key   string             `json:"dedupeKey"`
			Epoch string             `json:"dataEpoch"`
		}{"memory", "memory.assistant", r.Ref, id, op.DataEpoch}, id)
		if err != nil {
			return count, err
		}
		var receipt struct {
			Job struct {
				JobID string `json:"jobId"`
			} `json:"job"`
		}
		if ValidateForeign("JobsCreateJobResult", raw) != nil || json.Unmarshal(raw, &receipt) != nil || !platform.ValidIdentifier(receipt.Job.JobID) {
			return count, platform.NewError(platform.CodeIntegrityError, false, "助手任务接收回执损坏")
		}
		var result struct{ Saved bool }
		err = Write(work, op, "memory.DispatchAssistantInput", id, map[string]any{"runId": id, "jobId": receipt.Job.JobID}, &result, func(_ *Store, tx *sql.Tx) error {
			_, err := tx.ExecContext(work, `UPDATE assistant_inputs SET job_id=?,dispatch_state='submitted' WHERE run_id=? AND data_epoch=? AND dispatch_state='pending'`, receipt.Job.JobID, id, op.DataEpoch)
			result.Saved = err == nil
			return DBError(err)
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// AssistantInput 读取本域不可变任务输入，供平台执行器装配可信上下文。
func AssistantInput(ctx context.Context, id string) (AssistantInputRecord, error) {
	s := CurrentStore()
	if s == nil {
		return AssistantInputRecord{}, platform.NewError(platform.CodeDependencyUnavailable, true, "记忆存储未打开")
	}
	return ReadAssistantInput(ctx, s.db, id)
}

// ReplayReceipt 让收费任务创建的原结果重放不依赖再次查询模型配置。
func ReplayReceipt(ctx context.Context, op OperationContext, functionID, key string, input, result any) (bool, error) {
	s := CurrentStore()
	if s == nil {
		return false, platform.NewError(platform.CodeDependencyUnavailable, true, "记忆存储未打开")
	}
	var raw []byte
	var digest, expiry string
	err := s.db.QueryRowContext(ctx, `SELECT input_hash,result_json,expires_at FROM memory_receipts WHERE site_id=? AND actor_id=? AND function_id=? AND key_hash=? AND data_epoch=?`, op.Actor.SiteID, op.Actor.SubjectID, functionID, platform.HashBytes([]byte(key)), op.DataEpoch).Scan(&digest, &raw, &expiry)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, DBError(err)
	}
	expires, err := platform.ParseTimestamp(expiry)
	if err != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "记忆原回执时间损坏")
	}
	if !expires.After(time.Now()) {
		return false, nil
	}
	encoded, err := platform.CanonicalJSON(input)
	if err != nil {
		return false, platform.NewError(platform.CodeInternal, false, "记忆请求不可编码")
	}
	if digest != platform.HashBytes(encoded) {
		return false, platform.NewError(platform.CodeConflict, false, "同幂等键的记忆请求不同")
	}
	if json.Unmarshal(raw, result) != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "记忆原结果损坏")
	}
	return true, nil
}

// ValidateForeign 强制验证生产方成功响应；生成器保留生产方 schema 引用闭包。
func ValidateForeign(name string, raw json.RawMessage) error {
	s := CurrentStore()
	if s == nil || s.validators[name] == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "记忆响应校验器未装配")
	}
	if err := s.validators[name](raw); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "记忆依赖响应不符合合同")
	}
	return nil
}
func ConfigureValidators(validators map[string]func(json.RawMessage) error) error {
	s := CurrentStore()
	if s == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "记忆存储未打开")
	}
	s.validators = validators
	return nil
}
func CurrentAssistantState(ctx context.Context, site, run string) (string, int, error) {
	s := CurrentStore()
	if s == nil {
		return "", 0, platform.NewError(platform.CodeDependencyUnavailable, true, "记忆存储未打开")
	}
	var state string
	var generation int
	err := s.db.QueryRowContext(ctx, `SELECT r.state,s.memory_generation FROM assistant_runs r CROSS JOIN runtime_state s WHERE r.site_id=? AND r.run_id=? AND s.id=1`, site, run).Scan(&state, &generation)
	return state, generation, DBError(err)
}

// DispatchAssistantCancellations 恢复原任务/root取消，不推算费用或创建新模型调用。
func DispatchAssistantCancellations(ctx context.Context, limit int) error {
	s := CurrentStore()
	if s == nil || s.jobs == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "助手任务客户端未装配")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT run_id FROM assistant_inputs WHERE dispatch_state='cancelled' AND cancel_delivered=0 ORDER BY run_id LIMIT ?`, limit)
	if err != nil {
		return DBError(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return DBError(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return DBError(err)
	}
	for _, id := range ids {
		r, err := ReadAssistantInput(ctx, s.db, id)
		if err != nil {
			return err
		}
		op := r.Operation
		op.CallerService = "memory"
		op.Deadline = time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano)
		op.Actor = platform.AuthContext{Kind: "service", SubjectID: "memory", SiteID: r.SiteID, ExpiresAt: op.Deadline, Grants: []string{}}
		work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
		// pending投递可能已远端提交但本地未收到jobId；用原dedupe请求取回同一任务再取消。
		jobID := r.JobID
		if jobID == "" {
			raw, err := s.jobs.CreateJob(work, struct {
				Owner string             `json:"owner"`
				Type  string             `json:"type"`
				Ref   platform.ObjectRef `json:"inputRef"`
				Key   string             `json:"dedupeKey"`
				Epoch string             `json:"dataEpoch"`
			}{"memory", "memory.assistant", r.Ref, id, op.DataEpoch}, id)
			if err != nil {
				return err
			}
			var result struct {
				Job struct {
					ID string `json:"jobId"`
				} `json:"job"`
			}
			if ValidateForeign("JobsCreateJobResult", raw) != nil || json.Unmarshal(raw, &result) != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "助手任务接收回执损坏")
			}
			jobID = result.Job.ID
		}
		raw, err := s.jobs.CancelJob(work, jobID, struct{}{}, id+":cancel")
		if err != nil {
			return err
		}
		if ValidateForeign("JobsRequestCancelResult", raw) != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "助手任务取消回执损坏")
		}
		var invocation sql.NullString
		if err = s.db.QueryRowContext(work, `SELECT invocation_id FROM assistant_runs WHERE site_id=? AND run_id=?`, r.SiteID, id).Scan(&invocation); err != nil {
			return DBError(err)
		}
		if invocation.Valid && s.gateway != nil {
			if err = s.gateway.CancelInference(work, id+":cancel", invocation.String, "记忆助手已取消"); err != nil {
				return err
			}
		}
		var result struct{ Saved bool }
		if err = Write(work, op, "memory.DispatchAssistantCancellation", id, id, &result, func(_ *Store, tx *sql.Tx) error {
			_, err := tx.ExecContext(work, `UPDATE assistant_inputs SET job_id=?,cancel_delivered=1 WHERE run_id=? AND dispatch_state='cancelled'`, jobID, id)
			result.Saved = err == nil
			return DBError(err)
		}); err != nil {
			return err
		}
	}
	return nil
}
