package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/memory/internal/gen"
	"nice-wiki/modules/memory/internal/proposal"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type RunAssistantInput struct {
	RunId    string                     `json:"runId"`
	Messages []platform.GatewayMessage  `json:"messages"`
	LeaseId  string                     `json:"leaseId"`
	Job      platform.JobExecutionInput `json:"-"`
}
type RunAssistantResult struct {
	Run       gen.AssistantRun     `json:"run"`
	Proposals []gen.MemoryProposal `json:"proposals"`
}

func CheckJobBinding(ctx context.Context, op OperationContext, runID string, job platform.JobExecutionInput) (memoryruntime.AssistantInputRecord, error) {
	record, err := memoryruntime.AssistantInput(ctx, runID)
	if err != nil {
		return record, err
	}
	if record.DispatchState == "cancelled" {
		return record, platform.NewError(platform.CodeCancelled, false, "助手任务已取消")
	}
	if op.CallerService != "memory" || op.Actor.Kind != "service" || op.Actor.SubjectID != "memory" || op.Actor.SiteID != record.SiteID || op.RootOperationID != record.Operation.RootOperationID || op.OperationID != record.Operation.OperationID || op.DataEpoch != record.Operation.DataEpoch || op.CostClass != platform.CostClassAdminProbe || job.JobType != "memory.assistant" || job.InputRef != record.Ref || job.DataEpoch != op.DataEpoch || (record.JobID != "" && record.JobID != job.JobID) || job.CheckLease == nil {
		return record, platform.NewError(platform.CodeForbidden, false, "助手任务输入或租约绑定不匹配")
	}
	if err = job.CheckLease(ctx); err != nil {
		return record, err
	}
	return record, nil
}
func RunAssistant(ctx context.Context, op OperationContext, input RunAssistantInput) (RunAssistantResult, error) {
	record, err := CheckJobBinding(ctx, op, input.RunId, input.Job)
	if err != nil {
		return RunAssistantResult{}, err
	}
	state, generation, err := memoryruntime.CurrentAssistantState(ctx, op.Actor.SiteID, input.RunId)
	if err != nil {
		return RunAssistantResult{}, err
	}
	if state == "review" || state == "completed" || state == "superseded" {
		return RunAssistantResult{Run: gen.AssistantRun{RunId: input.RunId, SourceGeneration: record.Snapshot.SourceGeneration, ActorId: record.Operation.Actor.SubjectID, State: gen.AssistantRunState(state), ProposalIds: []string{}}, Proposals: []gen.MemoryProposal{}}, nil
	}
	if generation != record.Snapshot.SourceGeneration {
		committed, err := CommitAssistantProposals(ctx, op, CommitAssistantProposalsInput{RunId: input.RunId, ExpectedSourceGeneration: record.Snapshot.SourceGeneration, Job: input.Job})
		return RunAssistantResult{Run: committed.Run, Proposals: []gen.MemoryProposal{}}, err
	}
	// 原始不可变输入为唯一来源；任务参数中的messages不能改写批准时的正文。
	outcome, err := memoryruntime.InvokeModel(ctx, op, platform.InvokeModelRequest{Request: record.Snapshot.Request, IDempotencyKey: record.Operation.OperationID + ":assistant", Stream: false})
	if err != nil {
		return RunAssistantResult{}, err
	}
	if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil || outcome.CurrentSnapshot.Output.Text == nil {
		return RunAssistantResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "助手完整输出尚未持久化，保留原调用待查询")
	}
	text := *outcome.CurrentSnapshot.Output.Text
	if err = memoryruntime.ValidateForeign("AssistantModelOutput", []byte(text)); err != nil {
		return RunAssistantResult{}, err
	}
	d := json.NewDecoder(bytes.NewBufferString(text))
	d.DisallowUnknownFields()
	var decoded struct {
		Proposals []gen.MemoryProposal `json:"proposals"`
	}
	if d.Decode(&decoded) != nil || d.Decode(new(any)) != io.EOF || decoded.Proposals == nil || len(decoded.Proposals) > 256 {
		return RunAssistantResult{}, platform.NewError(platform.CodeIntegrityError, false, "助手候选不符合封闭JSON格式")
	}
	byID := map[string]gen.MemoryEntry{}
	for _, e := range record.Snapshot.Entries {
		byID[e.EntryId] = e
	}
	for i := range decoded.Proposals {
		p := &decoded.Proposals[i]
		p.ProposalId = platform.NewIdentifier()
		p.RunId = input.RunId
		p.State = "pending"
		p.ApprovedBy = nil
		p.ApprovalRevision = 0
		if p.EntryId != nil {
			e, ok := byID[*p.EntryId]
			if !ok || e.Readonly || p.ExpectedEntryVersion == nil || *p.ExpectedEntryVersion != e.Version {
				return RunAssistantResult{}, platform.NewError(platform.CodeIntegrityError, false, "助手修改了未选中、只读或已变化的记忆")
			}
		}
		checked, err := proposal.ValidateProposal(ctx, op, proposal.ValidateProposalInput{Proposal: *p, SourceGeneration: record.Snapshot.SourceGeneration})
		if err != nil {
			return RunAssistantResult{}, err
		}
		p.PayloadHash = checked.ProposalHash
	}
	committed, err := CommitAssistantProposals(ctx, op, CommitAssistantProposalsInput{RunId: input.RunId, Proposals: decoded.Proposals, ExpectedSourceGeneration: record.Snapshot.SourceGeneration, LeaseId: input.LeaseId, Job: input.Job, InvocationId: outcome.InvocationID})
	if err != nil {
		return RunAssistantResult{}, err
	}
	return RunAssistantResult{Run: committed.Run, Proposals: decoded.Proposals}, nil
}
