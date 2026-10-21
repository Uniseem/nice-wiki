package cache

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/normalize"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type DisableCacheEntryInput = gen.DisableCacheEntryInput
type DisableCacheEntryResult = gen.DisableCacheEntryResult

func DisableCacheEntry(ctx context.Context, op OperationContext, input DisableCacheEntryInput) (DisableCacheEntryResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.manage"); err != nil {
		return DisableCacheEntryResult{}, err
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return DisableCacheEntryResult{}, err
	}
	defer tx.Close()
	var result DisableCacheEntryResult
	replay, err := qaruntime.ReplayReceipt(tx, op, "qa.DisableCacheEntry", input.IdempotencyKey, input, &result)
	if err != nil {
		return result, err
	}
	if replay {
		return result, nil
	}
	record, err := ReadCache(tx, op, input.CacheId)
	if err != nil {
		return result, err
	}
	if record.Entry.Version != input.ExpectedVersion {
		return result, platform.NewError(platform.CodeVersionMismatch, false, "缓存版本已变化")
	}
	record.Entry.State = gen.CacheEntryState("disabled")
	record.Entry.Version++
	if err := SaveCache(tx, op, record); err != nil {
		return result, err
	}
	if err := EmitCacheChanged(tx, op, record.Entry); err != nil {
		return result, err
	}
	result.Entry = record.Entry
	if err := qaruntime.SaveReceipt(tx, op, "qa.DisableCacheEntry", input.IdempotencyKey, input, result); err != nil {
		return DisableCacheEntryResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return DisableCacheEntryResult{}, err
	}
	return result, nil
}

type ProposeCacheEntryInput = gen.ProposeCacheEntryInput
type ProposeCacheEntryResult = gen.ProposeCacheEntryResult

// ProposeCacheEntry 只保存管理员明确清理后的公开基准语言问题和答案，pending不命中。
func ProposeCacheEntry(ctx context.Context, op OperationContext, input ProposeCacheEntryInput) (ProposeCacheEntryResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.manage"); err != nil {
		return ProposeCacheEntryResult{}, err
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return ProposeCacheEntryResult{}, err
	}
	var result ProposeCacheEntryResult
	replay, err := qaruntime.ReplayReceipt(tx, op, "qa.ProposeCacheEntry", input.IdempotencyKey, input, &result)
	if err != nil {
		tx.Close()
		return result, err
	}
	if replay {
		tx.Close()
		return result, nil
	}
	record, err := qaruntime.ReadTurn(tx, op, input.TurnId, false)
	if err != nil {
		tx.Close()
		return result, err
	}
	policy, err := qaruntime.ReadPolicy(tx)
	if err != nil {
		tx.Close()
		return result, err
	}
	baseLanguage := tx.Runtime.BaseLanguage
	tx.Close()
	if record.State != "completed" || record.MemoryHash == "" {
		return result, platform.NewError(platform.CodeConflict, false, "只有具备完整上下文来源的已完成问答可精选")
	}
	normalized, err := normalize.NormalizeQuestion(ctx, op, normalize.NormalizeQuestionInput{Question: input.PublicQuestion, Language: baseLanguage})
	if err != nil {
		return result, err
	}
	citations, err := qaruntime.ValidateCitations(ctx, op, record.Citations, baseLanguage)
	if err != nil {
		return result, err
	}
	publicationHash := ""
	if len(citations) == 0 {
		publicationHash, err = qaruntime.PublicationFingerprint(ctx, op)
		if err != nil {
			return result, err
		}
	}
	tx, err = qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return result, err
	}
	defer tx.Close()
	replay, err = qaruntime.ReplayReceipt(tx, op, "qa.ProposeCacheEntry", input.IdempotencyKey, input, &result)
	if err != nil {
		return result, err
	}
	if replay {
		return result, nil
	}
	current, err := qaruntime.ReadTurn(tx, op, input.TurnId, false)
	if err != nil {
		return result, err
	}
	currentPolicy, err := qaruntime.ReadPolicy(tx)
	if err != nil {
		return result, err
	}
	if current.State != "completed" || current.Turn.AnswerRevision != record.Turn.AnswerRevision || currentPolicy.Version != policy.Version {
		return result, platform.NewError(platform.CodeVersionMismatch, false, "精选来源或策略已变化")
	}
	entry := gen.CacheEntry{CacheId: platform.NewIdentifier(), Version: 1, Language: baseLanguage, Question: input.PublicQuestion, Answer: input.PublicAnswer, State: gen.CacheEntryState("pending"), QuestionHash: normalized.QuestionHash, ScopeHash: ScopeHash(op, policy, baseLanguage), MemoryVersionHash: record.MemoryHash, Citations: citations, AnswerRevision: 1, Curated: false, CanonicalQuestion: normalized.Normalized, BaseLanguage: baseLanguage, QuestionComplete: true, LocalizedAnswers: make([]gen.LocalizedAnswer, 0)}
	if err := InsertCache(tx, op, CacheRecord{Entry: entry, SourceTurnID: input.TurnId, PublicationHash: publicationHash}); err != nil {
		return result, err
	}
	if err := EmitCacheChanged(tx, op, entry); err != nil {
		return result, err
	}
	result.Entry = entry
	if err := qaruntime.SaveReceipt(tx, op, "qa.ProposeCacheEntry", input.IdempotencyKey, input, result); err != nil {
		return ProposeCacheEntryResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProposeCacheEntryResult{}, err
	}
	return result, nil
}

type ApproveCacheEntryInput = gen.ApproveCacheEntryInput
type ApproveCacheEntryResult = gen.ApproveCacheEntryResult

// ApproveCacheEntry 把待批准的精选候选激活：重新核验引用，成功后长期有效（不按 TTL 过期），仍随来源撤回/改动失效。
func ApproveCacheEntry(ctx context.Context, op OperationContext, input ApproveCacheEntryInput) (ApproveCacheEntryResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.manage"); err != nil {
		return ApproveCacheEntryResult{}, err
	}
	read, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return ApproveCacheEntryResult{}, err
	}
	record, err := ReadCache(read, op, input.CacheId)
	baseLanguage := read.Runtime.BaseLanguage
	read.Close()
	if err != nil {
		return ApproveCacheEntryResult{}, err
	}
	if string(record.Entry.State) != "pending" || record.Entry.Version != input.ExpectedVersion {
		return ApproveCacheEntryResult{}, platform.NewError(platform.CodeVersionMismatch, false, "只有版本一致的待批准条目可以批准")
	}
	citations, err := qaruntime.ValidateCitations(ctx, op, record.Entry.Citations, baseLanguage)
	if err != nil {
		return ApproveCacheEntryResult{}, err
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return ApproveCacheEntryResult{}, err
	}
	defer tx.Close()
	var result ApproveCacheEntryResult
	if replay, err := qaruntime.ReplayReceipt(tx, op, "qa.ApproveCacheEntry", input.IdempotencyKey, input, &result); err != nil || replay {
		return result, err
	}
	current, err := ReadCache(tx, op, input.CacheId)
	if err != nil {
		return ApproveCacheEntryResult{}, err
	}
	if string(current.Entry.State) != "pending" || current.Entry.Version != input.ExpectedVersion {
		return ApproveCacheEntryResult{}, platform.NewError(platform.CodeVersionMismatch, false, "条目在批准前已变化")
	}
	current.Entry.Citations = citations
	current.Entry.State = gen.CacheEntryState("active")
	current.Entry.Curated = true
	current.Entry.ExpiresAt = nil
	current.Entry.Version++
	if err := SaveCache(tx, op, current); err != nil {
		return ApproveCacheEntryResult{}, err
	}
	if err := EmitCacheChanged(tx, op, current.Entry); err != nil {
		return ApproveCacheEntryResult{}, err
	}
	result.Entry = current.Entry
	if err := qaruntime.SaveReceipt(tx, op, "qa.ApproveCacheEntry", input.IdempotencyKey, input, result); err != nil {
		return ApproveCacheEntryResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ApproveCacheEntryResult{}, err
	}
	return result, nil
}
