package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

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

// reuseConfidence 是 Jev 选中某条缓存答案时要达到的把握；拿不准就正常回答，宁可多花一次调用也不答非所问。
const reuseConfidence = 0.8

// reuseCandidateLimit 是交给 Jev 挑选的缓存问答条数上限（精选和较新的优先）。
const reuseCandidateLimit = 60

// reuseAnswer 查找能原样复用的已有回答：先按完整问题精确匹配，再把同语言的缓存问答（英文问题）交给决策模型（Jev），
// 由它判断有没有一条与本轮问的是同一件事。命中就把缓存答案作为本轮回答（不调用回答模型）；没命中或 Jev 不可用时正常回答。
// 引用按当前发布状态重新核验，不再有效的条目不用。
func reuseAnswer(ctx context.Context, op OperationContext, turnID, lease string, record qaruntime.TurnRecord, resolved ResolveTurnContextResult, cp *questionPlan, firstTurn bool, step stepFunc) (bool, error) {
	step(gen.Cache, gen.Running, "查找已有回答", "", nil)
	entry, kind := exactCached(ctx, op, record, resolved, cp, firstTurn), "exact"
	if entry == nil {
		entry, kind = jevCached(ctx, op, record, resolved, cp), "semantic"
	}
	if entry == nil {
		step(gen.Cache, gen.Skipped, "查找已有回答", "", nil)
		return false, nil
	}
	step(gen.Cache, gen.Done, "命中缓存", "", []string{clip(entry.Question, 200)})
	answer := entry.Answer
	if _, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("delta"), Delta: &answer}}); err != nil {
		return false, err
	}
	for _, citation := range entry.Citations {
		c := citation
		if _, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("citation"), Citation: &c}}); err != nil {
			return false, err
		}
	}
	answerKind := "answered"
	if len(entry.Citations) == 0 {
		answerKind = "no_evidence"
	}
	_, err := CompleteTurn(ctx, op, CompleteTurnInput{TurnID: turnID, Answer: answer, Citations: entry.Citations, LeaseID: lease, AnswerKind: answerKind, TokenCount: len(answer)/4 + 1, CacheHit: kind})
	return err == nil, err
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

// jevCached 把本轮的英文问题和缓存里的英文问题交给 Jev 单选：哪一条问的是同一件事、答案可以原样给本轮访客。
func jevCached(ctx context.Context, op OperationContext, record qaruntime.TurnRecord, resolved ResolveTurnContextResult, cp *questionPlan) *gen.CacheEntry {
	if cp.plan.EnglishQuestion == "" {
		return nil
	}
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return nil
	}
	candidates, err := cache.ListReuseCandidates(ctx, op, cache.ScopeHash(op, resolved.Policy, runtime.BaseLanguage), record.Language, reuseCandidateLimit)
	if err != nil || len(candidates) == 0 {
		return nil
	}
	profile, err := qaruntime.ResolveTaskProfile(ctx, op, "qa.decide")
	if err != nil {
		return nil
	}
	criteria := map[string]string{"none": "None of these earlier questions asks for the same thing; answer the question from the documentation instead"}
	byKey := map[string]string{}
	for i, candidate := range candidates {
		key := "c" + strconv.Itoa(i+1)
		criteria[key] = candidate.EnglishQuestion
		byKey[key] = candidate.CacheID
	}
	question := platform.DecisionQuestion{Type: "choice", Criteria: criteria, Instructions: "A visitor of a documentation site asked `question`. Each option is an earlier visitor's question whose answer is cached. " +
		"Choose the option whose cached answer can be shown unchanged as a complete and correct answer to `question`: it must ask for the same thing about the same product, feature, document or step, with the same scope. " +
		"A related topic, a broader or narrower question, or a different step is not enough. Choose none if unsure."}
	answers, err := decide(ctx, op, profile, "cache-reuse", map[string]any{"question": cp.plan.EnglishQuestion}, map[string]platform.DecisionQuestion{"reuse": question})
	if err != nil {
		if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "缓存复用判断不可用 turn=%s err=%v\n", record.Turn.TurnId, err)
		}
		return nil
	}
	answer, ok := answers["reuse"]
	if !ok || answer.Type != "choice" || answer.Choice == "none" || answer.Probabilities[answer.Choice] < reuseConfidence || byKey[answer.Choice] == "" {
		return nil
	}
	entry, err := cache.LoadReusable(ctx, op, byKey[answer.Choice])
	if err != nil {
		return nil
	}
	return entry
}

// decide 调一次决策模型（Jev）回答类型化题目；6 秒内没结果按不可用处理，本轮照常回答。
func decide(ctx context.Context, op OperationContext, profile, key string, state any, questions map[string]platform.DecisionQuestion) (map[string]platform.DecisionAnswer, error) {
	work, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	outcome, err := qaruntime.InvokeModel(work, op, qaruntime.InvokeModelInput{Request: platform.CreateInvocationInput{
		TaskID: "qa.decide", ProfileID: platform.Identifier(profile), Capability: platform.CapabilityDecision, IDempotencyKey: key,
		Messages: []platform.GatewayMessage{}, Texts: []string{}, CandidateIDs: []platform.Identifier{},
		Decision: &platform.DecisionRequest{State: state, Questions: questions},
	}})
	if err != nil {
		return nil, err
	}
	if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "决策结果未完成")
	}
	return outcome.CurrentSnapshot.Output.Decision, nil
}

// writeCache 在一轮正常回答完成后把可共享的答案写入自动缓存，并记下英文问题供 Jev 判断复用；失败不影响本轮结果。
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
	if err == nil && result.Written && result.Entry != nil && cp.plan.EnglishQuestion != "" {
		err = cache.SaveEnglishQuestion(ctx, op, result.Entry.CacheId, cp.plan.EnglishQuestion)
	}
	if os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "写入自动缓存 turn=%s written=%v err=%v\n", turnID, result.Written, err)
	}
}
