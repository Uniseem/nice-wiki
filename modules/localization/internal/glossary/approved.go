package glossary

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
)

type ApplyApprovedGlossaryProposalInput = gen.ApplyApprovedGlossaryProposalInput
type ApplyApprovedGlossaryProposalResult = gen.ApplyApprovedGlossaryProposalResult

// ApplyApprovedGlossaryProposal 只接受memory签名的持久人工批准；重试不重复创建术语。
func ApplyApprovedGlossaryProposal(ctx context.Context, op OperationContext, input ApplyApprovedGlossaryProposalInput) (ApplyApprovedGlossaryProposalResult, error) {
	p := input.Proposal
	if op.CallerService != "memory" || op.Actor.Kind != "service" || op.Actor.SubjectID != "memory" || p.ApprovalRevision < 1 || !platform.ValidIdentifier(p.ApprovedBy) || input.IdempotencyKey != p.ProposalId {
		return ApplyApprovedGlossaryProposalResult{}, platform.NewError(platform.CodeForbidden, false, "术语命令缺少可信memory人工批准")
	}
	term, err := NormalizeTerm(ctx, op, p.Term)
	if err != nil {
		return ApplyApprovedGlossaryProposalResult{}, err
	}
	if term.Source != "manual" {
		return ApplyApprovedGlossaryProposalResult{}, platform.NewError(platform.CodeForbidden, false, "人工批准命令不能声明自动术语")
	}
	var result ApplyApprovedGlossaryProposalResult
	err = language.Mutate(ctx, op, "localization.ApplyApprovedGlossaryProposal", input.IdempotencyKey, input, &result, func(tx *sql.Tx) error {
		rawTerm, _ := platform.CanonicalJSON(p.Term)
		var digest, state, by, epoch, site, root string
		var revision, expectedGeneration, resultGeneration int
		var oldTerm []byte
		var termID sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT proposal_hash,state,approved_by,approval_revision,term_json,data_epoch,site_id,root_operation_id,term_id,expected_glossary_generation,result_generation FROM approved_proposals WHERE proposal_id=?`, p.ProposalId).Scan(&digest, &state, &by, &revision, &oldTerm, &epoch, &site, &root, &termID, &expectedGeneration, &resultGeneration)
		generation, gerr := Generation(ctx, tx)
		if gerr != nil {
			return gerr
		}
		if err == nil {
			if digest != p.ProposalHash || by != p.ApprovedBy || revision != p.ApprovalRevision || string(oldTerm) != string(rawTerm) || epoch != op.DataEpoch || site != op.Actor.SiteID || root != op.RootOperationID || expectedGeneration != p.ExpectedGlossaryGeneration {
				return platform.NewError(platform.CodeConflict, false, "批准命令已绑定不同内容、根或代次")
			}
			result.Applied = state == "applied"
			result.Generation = resultGeneration
			if termID.Valid {
				result.TermId = &termID.String
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return language.StorageError(err)
		}
		state = "rejected"
		if generation == p.ExpectedGlossaryGeneration {
			saved, next, writeErr := StoreTerm(ctx, tx, op, term, 0)
			if writeErr == nil {
				result.Applied = true
				result.Generation = next
				result.TermId = &saved.TermId
				state = "applied"
			} else {
				var cause *platform.Error
				if !errors.As(writeErr, &cause) || (cause.Code != platform.CodeConflict && cause.Code != platform.CodeVersionMismatch) {
					return writeErr
				}
			}
		}
		if !result.Applied {
			result.Generation = generation
		}
		var storedID any
		if result.TermId != nil {
			storedID = *result.TermId
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO approved_proposals(proposal_id,proposal_hash,state,term_id,approved_by,approval_revision,expected_glossary_generation,term_json,data_epoch,created_at,updated_at,site_id,root_operation_id,result_generation) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, p.ProposalId, p.ProposalHash, state, storedID, p.ApprovedBy, p.ApprovalRevision, p.ExpectedGlossaryGeneration, string(rawTerm), op.DataEpoch, platform.Now(), platform.Now(), op.Actor.SiteID, op.RootOperationID, result.Generation)
		if err != nil {
			return language.StorageError(err)
		}
		payload := map[string]any{"proposalId": p.ProposalId, "proposalHash": p.ProposalHash, "approvalRevision": p.ApprovalRevision, "result": state, "glossaryGeneration": result.Generation, "dataEpoch": op.DataEpoch}
		if result.TermId != nil {
			payload["termId"] = *result.TermId
		}
		return language.EmitReference(tx, op, "glossary.proposal."+state, p.ProposalId, p.ApprovalRevision, payload)
	})
	return result, err
}
