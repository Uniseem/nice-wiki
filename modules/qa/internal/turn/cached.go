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

// planQuestion 每轮都运行前置（fast 档位）：把问题改写成基准语言的独立问题、上报风险，并判断能否共享缓存。
// 风险准入类错误（封禁/限流/需验证）照常终止本轮；其他失败（未分配档位、模型不可用、结构不合格）只是没有改写结果。
func planQuestion(ctx context.Context, op OperationContext, turnID string, record qaruntime.TurnRecord, resolved ResolveTurnContextResult, history []gen.Message) (*questionPlan, context.Context, error) {
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
	planned, err := questioncontext.PlanQuestion(ctx, op, questioncontext.PlanQuestionInput{Question: record.Question, VisitorLanguage: record.Language, History: history, GuardSignals: []string{}})
	if err != nil {
		if failure, ok := err.(*platform.Error); ok && (failure.Code == platform.CodeForbidden || failure.Code == platform.CodeRateLimited || failure.Code == platform.CodeChallengeRequired) {
			return nil, ctx, err
		}
		return nil, ctx, nil
	}
	if err := savePlan(ctx, op, turnID, planned.Plan, resolved.Memory.VersionHash); err != nil {
		return nil, ctx, nil
	}
	eligibility, _ := cache.EvaluateCacheEligibility(ctx, op, cache.EvaluateCacheEligibilityInput{Plan: planned.Plan, FirstTurn: len(history) == 0, CurrentPageDependent: planned.CurrentPageDependent, PrivateContextDependent: planned.PrivateContextDependent})
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

// answerFromCache 命中精确缓存就直接用缓存答案完成本轮（不调用回答模型）。
// 只复用与访客同语言的条目；引用在 ValidateCachedAnswer 里已按当前发布状态重新核验。
func answerFromCache(ctx context.Context, op OperationContext, turnID, lease string, record qaruntime.TurnRecord, resolved ResolveTurnContextResult, cp *questionPlan, firstTurn bool) (bool, error) {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return false, err
	}
	normalized, err := normalize.NormalizeQuestion(ctx, op, normalize.NormalizeQuestionInput{Question: cp.plan.CanonicalQuestion, Language: runtime.BaseLanguage})
	if err != nil {
		return false, nil
	}
	hit, err := cache.LookupExactCache(ctx, op, cache.LookupExactCacheInput{QuestionHash: normalized.QuestionHash, Language: record.Language, ScopeHash: cache.ScopeHash(op, resolved.Policy, runtime.BaseLanguage), Complete: true, FirstTurn: firstTurn})
	if os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "精确缓存查询 turn=%s hit=%v err=%v scope=%s q=%s\n", turnID, hit.Hit, err, cache.ScopeHash(op, resolved.Policy, runtime.BaseLanguage)[:8], normalized.QuestionHash[:8])
	}
	if err != nil || !hit.Hit || hit.Entry == nil || hit.Entry.Language != record.Language {
		return false, nil
	}
	answer := hit.Entry.Answer
	if _, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("delta"), Delta: &answer}}); err != nil {
		return false, err
	}
	for _, citation := range hit.Entry.Citations {
		c := citation
		if _, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("citation"), Citation: &c}}); err != nil {
			return false, err
		}
	}
	kind := "answered"
	if len(hit.Entry.Citations) == 0 {
		kind = "no_evidence"
	}
	_, err = CompleteTurn(ctx, op, CompleteTurnInput{TurnID: turnID, Answer: answer, Citations: hit.Entry.Citations, LeaseID: lease, AnswerKind: kind, TokenCount: len(answer)/4 + 1, CacheHit: "exact"})
	return err == nil, err
}

// writeCache 在一轮正常回答完成后把可共享的答案写入自动缓存；失败不影响本轮结果。
func writeCache(ctx context.Context, op OperationContext, turnID string, cp *questionPlan, answer string, memoryHash string) {
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
	if os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "写入自动缓存 turn=%s written=%v err=%v\n", turnID, result.Written, err)
	}
}
