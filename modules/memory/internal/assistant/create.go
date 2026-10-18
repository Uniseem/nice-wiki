package assistant

import (
	"context"
	"database/sql"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type CreateAssistantRunInput = gen.CreateAssistantRunInput
type CreateAssistantRunResult = gen.CreateAssistantRunResult

func CreateAssistantRun(ctx context.Context, op OperationContext, input CreateAssistantRunInput) (CreateAssistantRunResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return CreateAssistantRunResult{}, err
	}
	if len(input.EntryIds) > 32 || len([]byte(input.Instruction)) > 16384 {
		return CreateAssistantRunResult{}, platform.NewError(platform.CodeValidation, false, "助手整理输入超过范围")
	}
	var result CreateAssistantRunResult
	if found, err := memoryruntime.ReplayReceipt(ctx, op, "memory.CreateAssistantRun", input.IdempotencyKey, input, &result); found || err != nil {
		return result, err
	}
	profile, err := memoryruntime.SelectAssistantProfile(ctx, op)
	if err != nil {
		return result, err
	}
	err = memoryruntime.Write(ctx, op, "memory.CreateAssistantRun", input.IdempotencyKey, input, &result, func(s *memoryruntime.Store, tx *sql.Tx) error {
		runID := platform.NewIdentifier()
		var generation int
		if err := tx.QueryRowContext(ctx, `SELECT memory_generation FROM runtime_state WHERE id=1`).Scan(&generation); err != nil {
			return memoryruntime.DBError(err)
		}
		entries := []gen.MemoryEntry{}
		total := len([]byte(input.Instruction))
		for _, id := range input.EntryIds {
			e, err := s.ReadEntryObject(ctx, tx, op.Actor.SiteID, id, nil)
			if err != nil {
				return err
			}
			total += len([]byte(e.Body)) + len([]byte(e.Title)) + len([]byte(e.Description))
			if total > 16384 {
				return platform.NewError(platform.CodeValidation, false, "助手固定正文输入超过16384字节")
			}
			entries = append(entries, e)
		}
		prompt, err := BuildAssistantPrompt(ctx, op, BuildAssistantPromptInput{RunId: runID, Instruction: input.Instruction, Entries: entries})
		if err != nil {
			return err
		}
		result.Run = gen.AssistantRun{RunId: runID, SourceGeneration: generation, State: "queued", ActorId: op.Actor.SubjectID, ProposalIds: []string{}}
		_, err = tx.ExecContext(ctx, `INSERT INTO assistant_runs(site_id,run_id,source_generation,state,actor_id,proposal_ids_json) VALUES(?,?,?,'queued',?,'[]')`, op.Actor.SiteID, runID, generation, op.Actor.SubjectID)
		if err != nil {
			return memoryruntime.DBError(err)
		}
		taskOp := op
		taskOp.CostClass = platform.CostClassAdminProbe
		taskOp.RootOperationID = taskOp.OperationID
		request := platform.CreateInvocationInput{TaskID: "memory.assistant", ProfileID: profile, Capability: "chat", Messages: prompt.Messages, Texts: []string{}, CandidateIDs: []string{}, MaximumOutputTokens: 1500, IDempotencyKey: taskOp.OperationID + ":assistant"}
		return memoryruntime.SaveAssistantInput(ctx, tx, taskOp, memoryruntime.AssistantSnapshot{RunID: runID, SourceGeneration: generation, Entries: entries, Request: request})
	})
	return result, err
}
