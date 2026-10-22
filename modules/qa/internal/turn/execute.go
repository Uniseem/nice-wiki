package turn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/answer"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/history"
	"nice-wiki/modules/qa/internal/normalize"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"nice-wiki/modules/qa/internal/stream"
)

// ExecuteTurn 在本进程里跑完一轮回答：理解问题 → 读当前页 → 混合检索 → 流式生成 → 校验引用并落终态；
// 任何失败都落 failed 终态并释放准入。currentDocument 是访客提问时所在页面（可为空）。
func ExecuteTurn(ctx context.Context, op OperationContext, turnID, currentDocument string) {
	work, cancel := context.WithDeadline(ctx, mustDeadline(op))
	defer cancel()
	if err := runTurn(work, op, turnID, currentDocument); err != nil {
		if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "问答轮次失败 turn=%s err=%v\n", turnID, err)
		}
		failTurn(ctx, op, turnID, err)
	}
}

func mustDeadline(op OperationContext) time.Time {
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return time.Now().Add(time.Minute)
	}
	return deadline
}

// running 记着本进程里正在执行的轮次，取消接口据此直接中断在途步骤和模型调用。
var running sync.Map

var errTurnCancelled = platform.NewError(platform.CodeCancelled, false, "问答轮次已取消")

// stopRunningTurn 中断本进程里正在执行的轮次；没有在执行（已结束或在别处）时什么也不做。
func stopRunningTurn(turnID string, cause error) {
	if stop, ok := running.Load(turnID); ok {
		stop.(context.CancelCauseFunc)(cause)
	}
}

func runTurn(parent context.Context, op OperationContext, turnID, currentDocument string) (err error) {
	runtime, err := qaruntime.FromContext(parent)
	if err != nil {
		return err
	}
	turnCtx, stop := context.WithCancelCause(parent)
	running.Store(turnID, stop)
	defer func() {
		running.Delete(turnID)
		// 被取消或续租被拒而中断的，报告真正的原因，而不是某一步看到的“上下文已取消”。
		if err != nil && turnCtx.Err() != nil {
			if cause := context.Cause(turnCtx); cause != nil {
				err = cause
			}
		}
		stop(nil)
	}()
	ctx := turnCtx
	lease := platform.NewIdentifier()
	record, err := takeLease(ctx, op, turnID, lease, "retrieving")
	if err != nil {
		return err
	}
	runtime.SetPermit(record.Permit)
	stopRenew := make(chan struct{})
	defer close(stopRenew)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopRenew:
				return
			case <-ticker.C:
				_ = renewLease(ctx, op, turnID, lease)
				if err := renewPermit(ctx, runtime, record); err != nil {
					stop(err)
					return
				}
			}
		}
	}()
	runtime.OnModelAccepted = func(id platform.Identifier) error {
		return recordInvocation(ctx, op, turnID, id)
	}
	runtime.OnModelEvent = func(event platform.StreamEvent) error {
		if event.Event == platform.StreamDelta && event.Delta != "" {
			delta := event.Delta
			_, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("delta"), Delta: &delta}})
			return err
		}
		return nil
	}
	ctx = qaruntime.WithRuntime(ctx, runtime)

	resolved, err := ResolveTurnContext(ctx, op, ResolveTurnContextInput{TurnID: turnID, Language: record.Language, LeaseID: lease})
	if err != nil {
		return err
	}
	conversation, err := history.LoadConversation(ctx, op, record.Turn.ConversationId, turnID)
	if err != nil {
		return err
	}
	step := func(key gen.AnswerStepKey, status gen.AnswerStepStatus, title, detail string, items []string) {
		emitStep(ctx, op, turnID, lease, key, status, title, detail, items)
	}
	// 理解问题：前置改写把追问补成独立问题（检索用它），同时分类并上报风险；前置不可用时退回访客原话。
	step(gen.Understand, gen.Running, "理解问题", "", nil)
	planned, ctx, err := planQuestion(ctx, op, turnID, record, resolved, conversation.Summary, conversation.Recent(6))
	if err != nil {
		return err
	}
	var query string
	if planned != nil {
		query = planned.plan.CanonicalQuestion
	} else {
		canonical, err := normalize.NormalizeQuestion(ctx, op, normalize.NormalizeQuestionInput{Question: record.Question, Language: record.Language})
		if err != nil {
			return err
		}
		query = canonical.Normalized
	}
	step(gen.Understand, gen.Done, "理解问题", query, nil)
	// 回答和当前页有关时缓存的答案不一定对得上，带了当前页的提问不读也不写缓存。
	cacheable := planned != nil && planned.cacheable && resolved.Policy.CacheEnabled && currentDocument == ""
	if cacheable {
		if hit, err := answerFromCache(ctx, op, turnID, lease, record, resolved, planned, conversation.Summary == "" && len(conversation.Messages) == 0); err != nil || hit {
			return err
		}
	}
	task, profile := "qa.answer", string(resolved.Policy.ProfileId)
	if routed, err := qaruntime.ResolveTaskProfile(ctx, op, "qa.answer"); err == nil {
		profile = routed
	}
	budget := materialBudget(ctx, op, resolved.Policy, task)
	// 当前页：访客在哪一页打开问答，就把那一页全文交给回答模型（问题常常指“这里”“这个功能”）。读不到不影响本轮。
	page := answer.RetrieveEvidenceResult{Citations: []gen.Citation{}, EvidenceText: "[]"}
	if currentDocument != "" {
		step(gen.Page, gen.Running, "阅读当前页面", "", nil)
		read, title, err := answer.ReadCurrentPage(ctx, op, answer.ReadDocumentsInput{TurnID: turnID, DocumentIDs: []string{currentDocument}, Language: record.Language, MaximumTokens: budget})
		if err == nil {
			page = read
			step(gen.Page, gen.Done, "阅读当前页面", "", []string{title})
		} else {
			step(gen.Page, gen.Skipped, "阅读当前页面", "", nil)
		}
	}
	// 检索：混合检索（关键词 BM25 + 向量，按排名融合后重排），找全站相关片段。
	step(gen.Search, gen.Running, "检索文档", query, nil)
	evidence, err := answer.RetrieveEvidence(ctx, op, answer.RetrieveEvidenceInput{TurnID: turnID, Normalized: query, Policy: resolved.Policy, Language: record.Language})
	if err != nil {
		return err
	}
	step(gen.Search, gen.Done, fmt.Sprintf("找到 %d 段相关内容", len(evidence.Citations)), query, citedTitles(evidence.Citations))
	// 全站概括（每天由层级摘要合成）让模型知道整个网站讲什么；读不到不影响本轮。
	overview, _ := qaruntime.ReadSiteOverview(ctx, op)
	if _, err := takeState(ctx, op, turnID, lease, "generating"); err != nil {
		return err
	}
	step(gen.Answer, gen.Running, "生成回答", "", nil)
	prompt, err := answerPrompt(ctx, op, record, resolved, answer.Materials{Overview: overview, Page: page.EvidenceText, Evidence: evidence.EvidenceText}, conversation, task)
	if err != nil {
		return err
	}
	outcome, err := qaruntime.InvokeModel(ctx, op, qaruntime.InvokeModelInput{Stream: true, Request: platform.CreateInvocationInput{TaskID: task, ProfileID: profile, Capability: platform.CapabilityChat, Messages: prompt.Messages, MaximumOutputTokens: int64(min(prompt.MaximumOutputTokens, resolved.Policy.MaximumOutputTokens)), IDempotencyKey: "answer"}})
	if err != nil {
		return err
	}
	text, err := qaruntime.ModelText(outcome)
	if err != nil {
		return err
	}
	citations := usedCitations(text, append(append([]gen.Citation{}, page.Citations...), evidence.Citations...))
	for _, citation := range citations {
		c := citation
		if _, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("citation"), Citation: &c}}); err != nil {
			return err
		}
	}
	step(gen.Answer, gen.Done, "生成回答", "", nil)
	kind := "answered"
	if len(citations) == 0 {
		kind = "no_evidence"
	}
	if _, err = CompleteTurn(ctx, op, CompleteTurnInput{TurnID: turnID, Answer: text, Citations: citations, LeaseID: lease, AnswerKind: kind, TokenCount: len(text)/4 + 1}); err != nil {
		return err
	}
	if cacheable && kind == "answered" {
		writeCache(ctx, op, turnID, planned, text, resolved.Memory.VersionHash)
	}
	return nil
}

// takeLease 取得执行租约并进入新状态；轮次已终止则放弃执行。
func takeLease(ctx context.Context, op OperationContext, turnID, lease, state string) (qaruntime.TurnRecord, error) {
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return qaruntime.TurnRecord{}, err
	}
	defer tx.Close()
	record, err := qaruntime.ReadTurn(tx, op, turnID, false)
	if err != nil {
		return record, err
	}
	if qaruntime.IsTerminal(record.State) || record.CancelRequested {
		return record, platform.NewError(platform.CodeCancelled, false, "问答轮次已终止")
	}
	until := time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano)
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE turns SET lease_id=?,lease_until=? WHERE turn_id=?`, lease, until, turnID); err != nil {
		return record, qaruntime.StorageError(err)
	}
	record.LeaseID, record.LeaseUntil = lease, until
	record.Turn.State, record.State = gen.TurnState(state), state
	if err := stream.SaveEvent(tx, &record, gen.AnswerEvent{Event: gen.AnswerEventEvent("progress"), Turn: &record.Turn}, "", nil); err != nil {
		return record, err
	}
	return record, tx.Commit()
}

func renewLease(ctx context.Context, op OperationContext, turnID, lease string) error {
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return err
	}
	defer tx.Close()
	until := time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano)
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE turns SET lease_until=? WHERE turn_id=? AND lease_id=?`, until, turnID, lease); err != nil {
		return qaruntime.StorageError(err)
	}
	return tx.Commit()
}

// renewPermit 在原根和风控允许的最长窗口内续租准入，换上新签的许可。
// 风控明确拒绝（封禁、到期、已释放）时返回错误，本轮随即停止，不再发起新的付费步骤；连不上风控就等下一次再试。
func renewPermit(ctx context.Context, runtime qaruntime.Runtime, record qaruntime.TurnRecord) error {
	if runtime.Guard == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "准入续租权威未装配")
	}
	op := record.Operation
	op.Deadline = time.Now().UTC().Add(10 * time.Second).Format(time.RFC3339Nano)
	raw, err := runtime.Guard.RenewAdmission(qaruntime.ServiceReadContext(ctx, op), record.Permit.AdmissionID, struct{}{}, "qa.renew."+platform.NewIdentifier())
	if err != nil {
		var problem *platform.Error
		if errors.As(err, &problem) && !problem.Retryable {
			return err
		}
		return nil
	}
	var response struct {
		Permit platform.AdmissionPermit `json:"permit"`
	}
	if err := qaruntime.DecodeResponse(runtime, "GuardRenewPermitResult", raw, &response); err != nil {
		return err
	}
	runtime.SetPermit(response.Permit)
	return nil
}

// recordInvocation 把网关已接受的模型调用记进轮次，取消时逐个取消；轮次已取消就拒绝，平台随即取消这次调用。
func recordInvocation(ctx context.Context, op OperationContext, turnID string, id platform.Identifier) error {
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return err
	}
	defer tx.Close()
	record, err := qaruntime.ReadTurn(tx, op, turnID, false)
	if err != nil {
		return err
	}
	if qaruntime.IsTerminal(record.State) || record.CancelRequested {
		return errTurnCancelled
	}
	if slices.Contains(record.InvocationIDs, id) {
		return nil
	}
	raw, _ := json.Marshal(append(record.InvocationIDs, id))
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE turns SET invocation_ids_json=? WHERE turn_id=?`, string(raw), turnID); err != nil {
		return qaruntime.StorageError(err)
	}
	return tx.Commit()
}

func takeState(ctx context.Context, op OperationContext, turnID, lease, state string) (qaruntime.TurnRecord, error) {
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return qaruntime.TurnRecord{}, err
	}
	defer tx.Close()
	record, err := qaruntime.ReadTurn(tx, op, turnID, false)
	if err != nil {
		return record, err
	}
	if err := qaruntime.CheckTurnLease(record, lease); err != nil {
		return record, err
	}
	record.Turn.State, record.State = gen.TurnState(state), state
	if err := stream.SaveEvent(tx, &record, gen.AnswerEvent{Event: gen.AnswerEventEvent("progress"), Turn: &record.Turn}, "", nil); err != nil {
		return record, err
	}
	return record, tx.Commit()
}

// summaryRoom 是给压缩后新摘要留的估算空间（摘要最多 1500 输出 token，按网关口径约 4000）。
const summaryRoom = 4000

// answerPrompt 组装回答提示词。超过回答模型窗口（扣掉预留比例和输出额度）时，像 ADE 一样先把早期对话压缩进摘要再组装；
// 后台关闭压缩时改为丢掉最早的消息。估算和网关核对窗口用同一口径。
func answerPrompt(ctx context.Context, op OperationContext, record qaruntime.TurnRecord, resolved ResolveTurnContextResult, materials answer.Materials, conversation history.Conversation, task string) (answer.BuildAnswerPromptResult, error) {
	build := func() (answer.BuildAnswerPromptResult, error) {
		return answer.BuildAnswerPrompt(ctx, op, answer.BuildAnswerPromptInput{Question: record.Question, Memory: resolved.Memory, Materials: materials, Summary: conversation.Summary, History: conversation.Messages})
	}
	prompt, err := build()
	if err != nil {
		return prompt, err
	}
	window, err := qaruntime.ResolveTaskWindow(ctx, op, task)
	if err != nil || window == 0 {
		return prompt, err
	}
	policy := resolved.Policy
	budget := window*(100-policy.ContextReservePercent)/100 - min(prompt.MaximumOutputTokens, policy.MaximumOutputTokens)
	if os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "回答上下文 turn=%s window=%d budget=%d estimate=%d messages=%d\n", record.Turn.TurnId, window, budget, platform.EstimateMessageTokens(prompt.Messages), len(conversation.Messages))
	}
	if platform.EstimateMessageTokens(prompt.Messages) <= budget {
		return prompt, nil
	}
	if !policy.CompactionEnabled {
		for len(conversation.Messages) > 0 && platform.EstimateMessageTokens(prompt.Messages) > budget {
			conversation.Messages = conversation.Messages[1:]
			if prompt, err = build(); err != nil {
				return prompt, err
			}
		}
		return prompt, nil
	}
	fixed := platform.EstimateMessageTokens([]platform.GatewayMessage{prompt.Messages[0], prompt.Messages[len(prompt.Messages)-1]})
	keep := max(0, min(policy.RetainedRecentTokens, budget-fixed-summaryRoom))
	if os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "压缩对话 turn=%s keep=%d\n", record.Turn.TurnId, keep)
	}
	if conversation, err = history.Compact(ctx, op, history.CompactInput{Conversation: conversation, Language: record.Language, KeepTokens: keep}); err != nil {
		return prompt, err
	}
	return build()
}

// emitStep 把回答过程的一步推给访客界面（思考过程展示）；推送失败不影响回答。
func emitStep(ctx context.Context, op OperationContext, turnID, lease string, key gen.AnswerStepKey, status gen.AnswerStepStatus, title, detail string, items []string) {
	value := gen.AnswerStep{Key: key, Status: status, Title: title}
	if detail != "" {
		detail = clip(detail, 2000)
		value.Detail = &detail
	}
	if len(items) > 0 {
		value.Items = &items
	}
	_, _ = stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("step"), Step: &value}})
}

// citedTitles 是命中片段所在文档的标题（去重、保持顺序，最多 8 个），给访客看检索到了什么。
func citedTitles(citations []gen.Citation) []string {
	seen := map[string]bool{}
	titles := []string{}
	for _, citation := range citations {
		if citation.Title == "" || seen[citation.Title] || len(titles) >= 8 {
			continue
		}
		seen[citation.Title] = true
		titles = append(titles, clip(citation.Title, 300))
	}
	return titles
}

func clip(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit])
}

// usedCitations 只留回答里真正引用到的材料。一处都没引用的回答（通常是“文档未提及”）不列来源，记为无依据，也不进缓存。
func usedCitations(text string, all []gen.Citation) []gen.Citation {
	used := []gen.Citation{}
	seen := map[string]bool{} // 当前页分块和检索结果可能是同一段（引用标识相同），只留一份
	for _, citation := range all {
		if !seen[citation.CitationId] && strings.Contains(text, citation.CitationId) {
			seen[citation.CitationId] = true
			used = append(used, citation)
		}
	}
	return used
}

// materialBudget 是整篇读取时本轮材料的词元上限（网关估算口径）：策略里的“上下文最大词元”，且不超过回答模型可用窗口的三分之一，给记忆、摘要和历史留位置。
func materialBudget(ctx context.Context, op OperationContext, policy gen.TypesQaPolicy, task string) int {
	budget := policy.MaximumContextTokens
	if window, err := qaruntime.ResolveTaskWindow(ctx, op, task); err == nil && window > 0 {
		budget = min(budget, (window*(100-policy.ContextReservePercent)/100-policy.MaximumOutputTokens)/3)
	}
	return max(budget, 1)
}

// FailInterruptedTurns 收尾上次进程留下的未终止轮次：执行它们的协程已随进程退出，落 failed 并释放准入，同会话才能继续提问。
// 只收尾、不重新执行，不会用新的幂等键再花一次钱。
func FailInterruptedTurns(ctx context.Context, op OperationContext, startedNS int64) error {
	read, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return err
	}
	rows, err := read.Tx.QueryContext(read.Context, `SELECT turn_id FROM turns WHERE site_id=? AND data_epoch=? AND state NOT IN ('completed','cancelled','failed','invalidated') AND created_ns<?`, op.Actor.SiteID, op.DataEpoch, startedNS)
	if err != nil {
		read.Close()
		return qaruntime.StorageError(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	read.Close()
	if err != nil {
		return qaruntime.StorageError(err)
	}
	for _, id := range ids {
		failTurn(ctx, op, id, platform.NewError(platform.CodeDependencyUnavailable, true, "服务重启，回答已中断"))
	}
	return nil
}

// failTurn 把未终止的轮次落成 failed，并尽量释放准入占位。
func failTurn(ctx context.Context, op OperationContext, turnID string, cause error) {
	code := platform.CodeInternal
	if failure, ok := cause.(*platform.Error); ok {
		code = failure.Code
	}
	work, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := qaruntime.BeginWrite(work, op)
	if err != nil {
		return
	}
	defer tx.Close()
	record, err := qaruntime.ReadTurn(tx, op, turnID, false)
	if err != nil || qaruntime.IsTerminal(record.State) {
		return
	}
	record.Turn.State, record.State = gen.TurnState("failed"), "failed"
	record.TerminalNS = time.Now().UnixNano()
	if err := stream.SaveEvent(tx, &record, gen.AnswerEvent{Event: gen.AnswerEventEvent("failed"), Turn: &record.Turn, Code: &code}, "", nil); err != nil {
		return
	}
	if tx.Commit() != nil {
		return
	}
	tx.Close()
	_ = releaseTurnPermit(work, record)
}
