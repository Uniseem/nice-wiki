package assistant

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type CancelAssistantRunInput = gen.CancelAssistantRunInput
type CancelAssistantRunResult = gen.CancelAssistantRunResult

func CancelAssistantRun(ctx context.Context, op OperationContext, input CancelAssistantRunInput) (CancelAssistantRunResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return CancelAssistantRunResult{}, err
	}
	var result CancelAssistantRunResult
	err := memoryruntime.Write(ctx, op, "memory.CancelAssistantRun", input.IdempotencyKey, input, &result, func(_ *memoryruntime.Store, tx *sql.Tx) error {
		run, err := CancelRunTransaction(ctx, tx, op, input.RunId)
		result.Run = run
		return err
	})
	if err != nil {
		return result, err
	}
	// 取消意图已持久化；远端不可用时后台继续投递，绝不撤销本地取消或重新发起模型。
	if err = memoryruntime.DispatchAssistantCancellations(ctx, 20); err != nil {
		return result, err
	}
	return result, nil
}
func CancelRunTransaction(ctx context.Context, tx *sql.Tx, op OperationContext, runID string) (gen.AssistantRun, error) {
	run := gen.AssistantRun{RunId: runID, ProposalIds: []string{}}
	var ids []byte
	var invocation sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT source_generation,state,actor_id,invocation_id,proposal_ids_json FROM assistant_runs WHERE site_id=? AND run_id=?`, op.Actor.SiteID, runID).Scan(&run.SourceGeneration, &run.State, &run.ActorId, &invocation, &ids)
	if errors.Is(err, sql.ErrNoRows) {
		return run, platform.NewError(platform.CodeNotFound, false, "助手任务不存在")
	}
	if err != nil {
		return run, memoryruntime.DBError(err)
	}
	if json.Unmarshal(ids, &run.ProposalIds) != nil {
		return run, platform.NewError(platform.CodeIntegrityError, false, "助手提议索引损坏")
	}
	if invocation.Valid {
		run.InvocationId = &invocation.String
	}
	if run.State == "completed" || run.State == "superseded" {
		return run, nil
	}
	run.State = "cancelled"
	if _, err = tx.ExecContext(ctx, `UPDATE assistant_runs SET state='cancelled' WHERE site_id=? AND run_id=?`, op.Actor.SiteID, runID); err != nil {
		return run, memoryruntime.DBError(err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE assistant_inputs SET dispatch_state='cancelled' WHERE site_id=? AND run_id=?`, op.Actor.SiteID, runID); err != nil {
		return run, memoryruntime.DBError(err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT proposal_id,payload_json FROM proposals WHERE site_id=? AND run_id=? AND state='pending'`, op.Actor.SiteID, runID)
	if err != nil {
		return run, memoryruntime.DBError(err)
	}
	pending := []gen.MemoryProposal{}
	for rows.Next() {
		var id string
		var raw []byte
		if err = rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return run, memoryruntime.DBError(err)
		}
		var p gen.MemoryProposal
		if json.Unmarshal(raw, &p) != nil {
			rows.Close()
			return run, platform.NewError(platform.CodeIntegrityError, false, "助手提议正文损坏")
		}
		pending = append(pending, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return run, memoryruntime.DBError(err)
	}
	for _, p := range pending {
		p.State = "rejected"
		p.ApprovalRevision++
		p.ApprovedBy = &op.Actor.SubjectID
		raw, _ := platform.CanonicalJSON(p)
		if _, err = tx.ExecContext(ctx, `UPDATE proposals SET state='rejected',approved_by=?,approval_revision=?,payload_json=?,rejection_reason='取消助手任务' WHERE site_id=? AND proposal_id=? AND state='pending'`, op.Actor.SubjectID, p.ApprovalRevision, raw, op.Actor.SiteID, p.ProposalId); err != nil {
			return run, memoryruntime.DBError(err)
		}
	}
	return run, nil
}
