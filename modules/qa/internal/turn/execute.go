package turn

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/answer"
	questioncontext "nice-wiki/modules/qa/internal/context"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/history"
	"nice-wiki/modules/qa/internal/normalize"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"nice-wiki/modules/qa/internal/stream"
)

// ExecuteTurn 在本进程里跑完一轮回答：检索证据 → 流式生成 → 校验引用并落终态；任何失败都落 failed 终态并释放准入。
func ExecuteTurn(ctx context.Context, op OperationContext, turnID string) {
	work, cancel := context.WithDeadline(ctx, mustDeadline(op))
	defer cancel()
	if err := runTurn(work, op, turnID); err != nil {
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

func runTurn(ctx context.Context, op OperationContext, turnID string) error {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return err
	}
	lease := platform.NewIdentifier()
	record, err := takeLease(ctx, op, turnID, lease, "retrieving")
	if err != nil {
		return err
	}
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
			}
		}
	}()
	runtime.AdmissionPermit = &record.Permit
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
	// 每轮先让决策模型判断：滥用交风控，离题和闲聊直接用固定回答结束，不检索也不调回答模型。
	decision := decideTurn(ctx, op, record, conversation)
	if decision != nil {
		if err := questioncontext.ReportRisk(ctx, op, platform.HashBytes([]byte(record.Question)), decision.risk); err != nil {
			return err
		}
		if decision.intent == "smalltalk" || decision.intent == "off_topic" {
			return replyWithoutRetrieval(ctx, op, turnID, lease, record, decision.intent)
		}
	}
	planned, ctx, err := planQuestion(ctx, op, turnID, record, resolved, decision, conversation.Summary, conversation.Recent(6))
	if err != nil {
		return err
	}
	if planned != nil && planned.cacheable && resolved.Policy.CacheEnabled {
		if hit, err := answerFromCache(ctx, op, turnID, lease, record, resolved, planned, conversation.Summary == "" && len(conversation.Messages) == 0); err != nil || hit {
			return err
		}
	}
	// 检索用前置改写后的独立问题，追问才能检索准；前置不可用时退回访客原话。
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
	// 文档路由和材料筛选（决策模型只读英文）：独立问题在每轮决策里已经选好文档；
	// 追问把改写出的独立问题译成英文后再单独路由一次，译不出来就只做全站检索。
	var documents []string
	var selectEvidence func([]answer.EvidenceCandidate) []string
	if decision != nil {
		documents = decision.documents
		english := decision.english
		if !decision.standalone && planned != nil {
			if translated, err := toEnglish(ctx, op, query, runtime.BaseLanguage, "english.canonical"); err == nil {
				english = translated
				documents = routeDocuments(ctx, op, english, "", 0)
			}
		}
		selectEvidence = evidenceSelector(ctx, op, english)
	}
	// 简单问题改用快速档回答（任务 qa.answer-fast 没分配时仍用回答档位）。
	task, profile := "qa.answer", string(resolved.Policy.ProfileId)
	if decision != nil && decision.simple {
		if fast, err := qaruntime.ResolveTaskProfile(ctx, op, "qa.answer-fast"); err == nil {
			task, profile = "qa.answer-fast", fast
		}
	}
	// 决策模型选中了文档就把这些文章整篇交给回答模型；没选中、读不到或决策不可用时才走片段检索（全站检索加材料筛选）。
	var evidence answer.RetrieveEvidenceResult
	if len(documents) > 0 {
		if evidence, err = answer.ReadDocuments(ctx, op, answer.ReadDocumentsInput{TurnID: turnID, DocumentIDs: documents, Language: record.Language, MaximumTokens: materialBudget(ctx, op, resolved.Policy, task)}); err != nil {
			return err
		}
		if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "整篇读取 turn=%s 文档=%d 引用=%d\n", turnID, len(documents), len(evidence.Citations))
		}
	}
	if len(evidence.Citations) == 0 {
		if evidence, err = answer.RetrieveEvidence(ctx, op, answer.RetrieveEvidenceInput{TurnID: turnID, Normalized: query, Policy: resolved.Policy, Language: record.Language, Select: selectEvidence}); err != nil {
			return err
		}
	}
	if _, err := takeState(ctx, op, turnID, lease, "generating"); err != nil {
		return err
	}
	prompt, err := answerPrompt(ctx, op, record, resolved, evidence.EvidenceText, conversation, task)
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
	evidence.Citations = usedCitations(text, evidence.Citations)
	for _, citation := range evidence.Citations {
		c := citation
		if _, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("citation"), Citation: &c}}); err != nil {
			return err
		}
	}
	kind := "answered"
	if len(evidence.Citations) == 0 {
		kind = "no_evidence"
	}
	if _, err = CompleteTurn(ctx, op, CompleteTurnInput{TurnID: turnID, Answer: text, Citations: evidence.Citations, LeaseID: lease, AnswerKind: kind, TokenCount: len(text)/4 + 1}); err != nil {
		return err
	}
	if planned != nil && planned.cacheable && resolved.Policy.CacheEnabled && kind == "answered" {
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
func answerPrompt(ctx context.Context, op OperationContext, record qaruntime.TurnRecord, resolved ResolveTurnContextResult, evidenceText string, conversation history.Conversation, task string) (answer.BuildAnswerPromptResult, error) {
	build := func() (answer.BuildAnswerPromptResult, error) {
		return answer.BuildAnswerPrompt(ctx, op, answer.BuildAnswerPromptInput{Question: record.Question, Memory: resolved.Memory, EvidenceText: evidenceText, Summary: conversation.Summary, History: conversation.Messages})
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

// usedCitations 只留回答里真正引用到的材料。一处都没引用的回答（通常是“文档未提及”）不列来源，记为无依据，也不进缓存。
func usedCitations(text string, all []gen.Citation) []gen.Citation {
	used := []gen.Citation{}
	for _, citation := range all {
		if strings.Contains(text, citation.CitationId) {
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
