package turn

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type OperationContext = platform.OperationContext
type ResolveTurnContextInput struct {
	TurnID   string `json:"turnId"`
	Language string `json:"language"`
	LeaseID  string `json:"leaseId"`
}
type ResolveTurnContextResult struct {
	Memory      qaruntime.MemoryContext `json:"memory"`
	Policy      gen.TypesQaPolicy       `json:"policy"`
	ContextHash string                  `json:"contextHash"`
}

func ResolveTurnContext(ctx context.Context, op OperationContext, input ResolveTurnContextInput) (ResolveTurnContextResult, error) {
	read, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return ResolveTurnContextResult{}, err
	}
	record, err := qaruntime.ReadTurn(read, op, input.TurnID, false)
	if err != nil {
		read.Close()
		return ResolveTurnContextResult{}, err
	}
	conversation, err := qaruntime.ReadConversation(read, op, record.Turn.ConversationId, false)
	if err != nil {
		read.Close()
		return ResolveTurnContextResult{}, err
	}
	policy, err := qaruntime.ReadPolicy(read)
	read.Close()
	if err != nil {
		return ResolveTurnContextResult{}, err
	}
	if record.Turn.RootOperationId != op.RootOperationID || input.Language != conversation.Language {
		return ResolveTurnContextResult{}, platform.NewError(platform.CodeForbidden, false, "轮次根任务或回答语言不匹配")
	}
	if err := qaruntime.CheckTurnLease(record, input.LeaseID); err != nil {
		return ResolveTurnContextResult{}, err
	}
	memory, err := qaruntime.ResolveTaskMemory(ctx, op, "qa.answer", input.Language)
	if err != nil {
		return ResolveTurnContextResult{}, err
	}
	raw, _ := platform.CanonicalJSON(struct {
		Epoch      string
		Policy     gen.TypesQaPolicy
		MemoryHash string
	}{op.DataEpoch, policy, memory.VersionHash})
	hash := platform.HashBytes(raw)
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return ResolveTurnContextResult{}, err
	}
	defer tx.Close()
	current, err := qaruntime.ReadTurn(tx, op, input.TurnID, false)
	if err != nil {
		return ResolveTurnContextResult{}, err
	}
	activePolicy, err := qaruntime.ReadPolicy(tx)
	if err != nil {
		return ResolveTurnContextResult{}, err
	}
	if activePolicy.Version != policy.Version {
		return ResolveTurnContextResult{}, platform.NewError(platform.CodeStaleResult, false, "问答上下文读取期间策略已变化")
	}
	if err := qaruntime.CheckTurnLease(current, input.LeaseID); err != nil {
		return ResolveTurnContextResult{}, err
	}
	if current.MemoryHash != "" && (current.MemoryHash != memory.VersionHash || current.Turn.ContextHash != hash) {
		return ResolveTurnContextResult{}, platform.NewError(platform.CodeStaleResult, false, "该轮已经固定了其他上下文版本")
	}
	current.MemoryHash = memory.VersionHash
	current.Turn.ContextHash = hash
	if err := qaruntime.SaveTurn(tx, current); err != nil {
		return ResolveTurnContextResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ResolveTurnContextResult{}, err
	}
	return ResolveTurnContextResult{Memory: memory, Policy: policy, ContextHash: hash}, nil
}
