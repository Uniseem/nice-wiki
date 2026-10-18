package assistant

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type CommitAssistantProposalsInput struct {
	RunId                    string
	Proposals                []gen.MemoryProposal
	ExpectedSourceGeneration int
	LeaseId                  string
	Job                      platform.JobExecutionInput
	InvocationId             string
}
type CommitAssistantProposalsResult struct {
	Run      gen.AssistantRun `json:"run"`
	Accepted int              `json:"accepted"`
}

func CommitAssistantProposals(ctx context.Context, op OperationContext, input CommitAssistantProposalsInput) (CommitAssistantProposalsResult, error) {
	if _, err := CheckJobBinding(ctx, op, input.RunId, input.Job); err != nil {
		return CommitAssistantProposalsResult{}, err
	}
	var result CommitAssistantProposalsResult
	err := memoryruntime.Write(ctx, op, "memory.CommitAssistantProposals", input.RunId, input.RunId, &result, func(s *memoryruntime.Store, tx *sql.Tx) error {
		run := gen.AssistantRun{RunId: input.RunId, ProposalIds: []string{}}
		var invocation sql.NullString
		var ids []byte
		if err := tx.QueryRowContext(ctx, `SELECT source_generation,state,actor_id,invocation_id,proposal_ids_json FROM assistant_runs WHERE site_id=? AND run_id=?`, op.Actor.SiteID, input.RunId).Scan(&run.SourceGeneration, &run.State, &run.ActorId, &invocation, &ids); err != nil {
			return memoryruntime.DBError(err)
		}
		if json.Unmarshal(ids, &run.ProposalIds) != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "助手提议索引损坏")
		}
		if invocation.Valid {
			run.InvocationId = &invocation.String
		}
		if run.State == "cancelled" || run.State == "completed" || run.State == "superseded" || run.State == "review" {
			result.Run = run
			return nil
		}
		var generation int
		if err := tx.QueryRowContext(ctx, `SELECT memory_generation FROM runtime_state WHERE id=1`).Scan(&generation); err != nil {
			return memoryruntime.DBError(err)
		}
		superseded := generation != input.ExpectedSourceGeneration || run.SourceGeneration != input.ExpectedSourceGeneration
		for _, p := range input.Proposals {
			if p.EntryId != nil {
				e, err := s.ReadEntryObject(ctx, tx, op.Actor.SiteID, *p.EntryId, nil)
				if err != nil {
					return err
				}
				if p.ExpectedEntryVersion == nil || e.Version != *p.ExpectedEntryVersion || e.Readonly {
					superseded = true
				}
			}
		}
		if superseded {
			run.State = "superseded"
		} else {
			for _, p := range input.Proposals {
				raw, _ := platform.CanonicalJSON(p)
				_, err := tx.ExecContext(ctx, `INSERT INTO proposals(site_id,proposal_id,run_id,kind,state,payload_hash,expected_entry_version,approval_revision,payload_json) VALUES(?,?,?,?,'pending',?,?,0,?)`, op.Actor.SiteID, p.ProposalId, input.RunId, p.Kind, p.PayloadHash, p.ExpectedEntryVersion, raw)
				if err != nil {
					return memoryruntime.DBError(err)
				}
				run.ProposalIds = append(run.ProposalIds, p.ProposalId)
			}
			run.State = "review"
			result.Accepted = len(input.Proposals)
		}
		run.InvocationId = &input.InvocationId
		ids, _ = json.Marshal(run.ProposalIds)
		_, err := tx.ExecContext(ctx, `UPDATE assistant_runs SET state=?,invocation_id=?,proposal_ids_json=? WHERE site_id=? AND run_id=?`, run.State, input.InvocationId, ids, op.Actor.SiteID, input.RunId)
		result.Run = run
		return memoryruntime.DBError(err)
	})
	return result, err
}
