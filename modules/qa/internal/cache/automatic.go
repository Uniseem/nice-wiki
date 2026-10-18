package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/normalize"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"time"
)

type WriteAutomaticCacheInput struct {
	TurnID            string         `json:"turnId"`
	Plan              QuestionPlan   `json:"plan"`
	Answer            string         `json:"answer"`
	Citations         []gen.Citation `json:"citations"`
	MemoryVersionHash string         `json:"memoryVersionHash"`
}
type WriteAutomaticCacheResult struct {
	Entry   *gen.CacheEntry `json:"entry,omitempty"`
	Written bool            `json:"written"`
}

func WriteAutomaticCache(ctx context.Context, op OperationContext, input WriteAutomaticCacheInput) (WriteAutomaticCacheResult, error) {
	eligibility, _ := EvaluateCacheEligibility(ctx, op, EvaluateCacheEligibilityInput{Plan: input.Plan})
	if !eligibility.Eligible {
		return WriteAutomaticCacheResult{}, nil
	}
	read, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return WriteAutomaticCacheResult{}, err
	}
	policy, err := qaruntime.ReadPolicy(read)
	if err != nil {
		read.Close()
		return WriteAutomaticCacheResult{}, err
	}
	if !policy.CacheEnabled {
		read.Close()
		return WriteAutomaticCacheResult{}, nil
	}
	record, err := qaruntime.ReadTurn(read, op, input.TurnID, false)
	if err != nil {
		read.Close()
		return WriteAutomaticCacheResult{}, err
	}
	conversation, err := qaruntime.ReadConversation(read, op, record.Turn.ConversationId, false)
	baseLanguage := read.Runtime.BaseLanguage
	read.Close()
	if err != nil {
		return WriteAutomaticCacheResult{}, err
	}
	if record.State != "completed" || record.Plan == nil || !record.Plan.Complete || !record.Plan.CacheEligible || record.Plan.ContextFingerprint != input.Plan.ContextFingerprint || record.CanonicalQuestion != input.Plan.CanonicalQuestion || record.MemoryHash != input.MemoryVersionHash || record.Answer != input.Answer {
		return WriteAutomaticCacheResult{}, platform.NewError(platform.CodeStaleResult, false, "自动缓存输入不是该轮最终可信答案")
	}
	sourceCitations, _ := platform.CanonicalJSON(record.Citations)
	inputCitations, _ := platform.CanonicalJSON(input.Citations)
	if string(sourceCitations) != string(inputCitations) {
		return WriteAutomaticCacheResult{}, platform.NewError(platform.CodeIntegrityError, false, "自动缓存引用不是该轮最终白名单")
	}
	normalized, err := normalize.NormalizeQuestion(ctx, op, normalize.NormalizeQuestionInput{Question: input.Plan.CanonicalQuestion, Language: baseLanguage})
	if err != nil {
		return WriteAutomaticCacheResult{}, err
	}
	citations, err := qaruntime.ValidateCitations(ctx, op, input.Citations, conversation.Language)
	if err != nil {
		return WriteAutomaticCacheResult{}, err
	}
	publicationHash := ""
	if len(citations) == 0 {
		publicationHash, err = qaruntime.PublicationFingerprint(ctx, op)
		if err != nil {
			return WriteAutomaticCacheResult{}, err
		}
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return WriteAutomaticCacheResult{}, err
	}
	defer tx.Close()
	current, err := qaruntime.ReadTurn(tx, op, input.TurnID, false)
	if err != nil {
		return WriteAutomaticCacheResult{}, err
	}
	activePolicy, err := qaruntime.ReadPolicy(tx)
	if err != nil {
		return WriteAutomaticCacheResult{}, err
	}
	if !activePolicy.CacheEnabled {
		return WriteAutomaticCacheResult{}, nil
	}
	if activePolicy.Version != policy.Version || current.State != "completed" || current.Turn.AnswerRevision != record.Turn.AnswerRevision {
		return WriteAutomaticCacheResult{}, platform.NewError(platform.CodeStaleResult, false, "缓存提交前来源或策略已变化")
	}
	var feedback string
	err = tx.Tx.QueryRowContext(tx.Context, `SELECT value FROM feedback WHERE turn_id=? AND visitor_subject=?`, input.TurnID, current.SubjectID).Scan(&feedback)
	if err != nil && err != sql.ErrNoRows {
		return WriteAutomaticCacheResult{}, qaruntime.StorageError(err)
	}
	if feedback == "unhelpful" {
		return WriteAutomaticCacheResult{}, nil
	}
	identity, _ := json.Marshal(struct {
		TurnID   string
		Revision int
	}{input.TurnID, record.Turn.AnswerRevision})
	id := "auto." + platform.HashBytes(identity)
	var exists string
	err = tx.Tx.QueryRowContext(tx.Context, `SELECT cache_id FROM answer_cache WHERE cache_id=? AND site_id=? AND data_epoch=?`, id, op.Actor.SiteID, op.DataEpoch).Scan(&exists)
	if err == nil {
		existing, err := ReadCache(tx, op, id)
		if err != nil {
			return WriteAutomaticCacheResult{}, err
		}
		if string(existing.Entry.State) == "active" {
			return WriteAutomaticCacheResult{Entry: &existing.Entry, Written: true}, nil
		}
		return WriteAutomaticCacheResult{}, nil
	}
	if err != sql.ErrNoRows {
		return WriteAutomaticCacheResult{}, qaruntime.StorageError(err)
	}
	expires := time.Now().Add(time.Duration(policy.CacheTtlDays) * 24 * time.Hour).UTC()
	entry := gen.CacheEntry{CacheId: id, Version: 1, Language: conversation.Language, Question: input.Plan.OriginalQuestion, Answer: input.Answer, State: gen.CacheEntryState("active"), QuestionHash: normalized.QuestionHash, ScopeHash: ScopeHash(op, policy, baseLanguage), MemoryVersionHash: input.MemoryVersionHash, Citations: citations, AnswerRevision: record.Turn.AnswerRevision, Curated: false, ExpiresAt: &expires, CanonicalQuestion: normalized.Normalized, BaseLanguage: baseLanguage, QuestionComplete: true, LocalizedAnswers: make([]gen.LocalizedAnswer, 0)}
	if err := InsertCache(tx, op, CacheRecord{Entry: entry, SourceTurnID: input.TurnID, PublicationHash: publicationHash}); err != nil {
		return WriteAutomaticCacheResult{}, err
	}
	if err := EmitCacheChanged(tx, op, entry); err != nil {
		return WriteAutomaticCacheResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return WriteAutomaticCacheResult{}, err
	}
	return WriteAutomaticCacheResult{Entry: &entry, Written: true}, nil
}
