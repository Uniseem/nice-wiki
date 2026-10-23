package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/cache"
	questioncontext "nice-wiki/modules/qa/internal/context"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/normalize"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"nice-wiki/modules/qa/internal/stream"
)

// questionPlan 是本轮前置改写的结果；nil 表示前置不可用，本轮按访客原话检索、不用缓存。
type questionPlan struct {
	plan      cache.QuestionPlan
	cacheable bool
}

// planQuestion 得到本轮的独立问题并判断能否共享缓存。每轮决策判定“不看上文也能懂”且访客用基准语言时，
// 直接用规范化后的原话，不调文本模型；否则运行前置改写（fast 档位）把问题改写成基准语言的独立问题。
// 风险准入类错误（封禁/限流/需验证）照常终止本轮；其他失败（未分配档位、模型不可用、结构不合格）只是没有改写结果。
func planQuestion(ctx context.Context, op OperationContext, turnID string, record qaruntime.TurnRecord, resolved ResolveTurnContextResult, summary string, history []gen.Message, pageTitle string) (*questionPlan, context.Context, error) {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return nil, ctx, err
	}
	profile, err := qaruntime.ResolveTaskProfile(ctx, op, "qa.preflight")
	if err != nil {
		return nil, ctx, nil
	}
	runtime.FastProfileID = profile
	runtime.AnswerMemoryHash = resolved.Memory.VersionHash // 缓存条目要与当前回答记忆版本一致才能复用
	ctx = qaruntime.WithRuntime(ctx, runtime)
	planned, err := questioncontext.PlanQuestion(ctx, op, questioncontext.PlanQuestionInput{Question: record.Question, VisitorLanguage: record.Language, History: history, PreviousSummary: summary, CurrentPageTitle: pageTitle, GuardSignals: []string{}, SkipRiskReport: false})
	if err != nil {
		if failure, ok := err.(*platform.Error); ok && (failure.Code == platform.CodeForbidden || failure.Code == platform.CodeRateLimited || failure.Code == platform.CodeChallengeRequired) {
			return nil, ctx, err
		}
		return nil, ctx, nil
	}
	if err := savePlan(ctx, op, turnID, planned.Plan, resolved.Memory.VersionHash); err != nil {
		return nil, ctx, nil
	}
	eligibility, _ := cache.EvaluateCacheEligibility(ctx, op, cache.EvaluateCacheEligibilityInput{Plan: planned.Plan, FirstTurn: summary == "" && len(history) == 0, CurrentPageDependent: planned.CurrentPageDependent, PrivateContextDependent: planned.PrivateContextDependent})
	return &questionPlan{plan: planned.Plan, cacheable: eligibility.Eligible}, ctx, nil
}

func savePlan(ctx context.Context, op OperationContext, turnID string, plan cache.QuestionPlan, memoryHash string) error {
	raw, err := platform.CanonicalJSON(plan)
	if err != nil {
		return err
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return err
	}
	defer tx.Close()
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE turns SET plan_json=?,canonical_question=?,memory_hash=? WHERE turn_id=?`, json.RawMessage(raw), plan.CanonicalQuestion, memoryHash, turnID); err != nil {
		return qaruntime.StorageError(err)
	}
	return tx.Commit()
}

// serveCached 把缓存答案作为本轮回答（不调用回答模型）：推送正文和引用后完成本轮。引用已按当前发布状态重新核验。
func serveCached(ctx context.Context, op OperationContext, turnID, lease string, entry *gen.CacheEntry, kind string) error {
	answer := entry.Answer
	if _, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("delta"), Delta: &answer}}); err != nil {
		return err
	}
	for _, citation := range entry.Citations {
		c := citation
		if _, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("citation"), Citation: &c}}); err != nil {
			return err
		}
	}
	answerKind := "answered"
	if len(entry.Citations) == 0 {
		answerKind = "no_evidence"
	}
	_, err := CompleteTurn(ctx, op, CompleteTurnInput{TurnID: turnID, Answer: answer, Citations: entry.Citations, LeaseID: lease, AnswerKind: answerKind, TokenCount: len(answer)/4 + 1, CacheHit: kind})
	return err
}

// exactCached 按完整问题的规范化哈希找同语言的缓存条目。
func exactCached(ctx context.Context, op OperationContext, record qaruntime.TurnRecord, resolved ResolveTurnContextResult, cp *questionPlan, firstTurn bool) *gen.CacheEntry {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return nil
	}
	normalized, err := normalize.NormalizeQuestion(ctx, op, normalize.NormalizeQuestionInput{Question: cp.plan.CanonicalQuestion, Language: runtime.BaseLanguage})
	if err != nil {
		return nil
	}
	hit, err := cache.LookupExactCache(ctx, op, cache.LookupExactCacheInput{QuestionHash: normalized.QuestionHash, Language: record.Language, ScopeHash: cache.ScopeHash(op, resolved.Policy, runtime.BaseLanguage), Complete: true, FirstTurn: firstTurn})
	if err != nil || !hit.Hit {
		return nil
	}
	return hit.Entry
}

// writeCache 在一轮正常回答完成后把可共享的答案写入自动缓存，并记下英文问题和提问所在的文档供 Jev 判断复用；失败不影响本轮结果。
func writeCache(ctx context.Context, op OperationContext, turnID string, cp *questionPlan, answer, memoryHash, currentDocument string) {
	read, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return
	}
	record, err := qaruntime.ReadTurn(read, op, turnID, false)
	read.Close()
	if err != nil {
		return
	}
	result, err := cache.WriteAutomaticCache(ctx, op, cache.WriteAutomaticCacheInput{TurnID: turnID, Plan: cp.plan, Answer: answer, Citations: record.Citations, MemoryVersionHash: memoryHash})
	if err == nil && result.Written && result.Entry != nil && cp.plan.EnglishQuestion != "" {
		err = cache.SaveReuseInfo(ctx, op, result.Entry.CacheId, cp.plan.EnglishQuestion, currentDocument)
	}
	if os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "写入自动缓存 turn=%s written=%v err=%v\n", turnID, result.Written, err)
	}
}
