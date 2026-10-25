package turn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelagent"
	"nice-wiki/modules/qa/internal/answer"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/history"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"nice-wiki/modules/qa/internal/stream"
)

// ExecuteTurn 在本进程里跑完一轮回答：预读当前页、自主查阅、风险上报、流式回答及引用终态；
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
	progress := &turnProgress{op: op, turnID: turnID, lease: lease, started: map[string]time.Time{}}
	progress.emit(ctx, "prepare", gen.AnswerStep{Key: gen.Understand, Status: gen.AnswerStepStatusRunning, Title: "准备回答"})

	resolved, err := ResolveTurnContext(ctx, op, ResolveTurnContextInput{TurnID: turnID, Language: record.Language, LeaseID: lease})
	if err != nil {
		return err
	}
	conversation, err := history.LoadConversation(ctx, op, record.Turn.ConversationId, turnID)
	if err != nil {
		return err
	}
	progress.emit(ctx, "prepare", gen.AnswerStep{Key: gen.Understand, Status: gen.AnswerStepStatusDone, Title: "准备回答"})
	step := func(key gen.AnswerStepKey, status gen.AnswerStepStatus, title, detail string, items []string) {
		progress.stage(ctx, key, status, title, detail, items)
	}
	task, profile := "qa.answer", string(resolved.Policy.ProfileId)
	if routed, err := qaruntime.ResolveTaskProfile(ctx, op, "qa.answer"); err == nil {
		profile = routed
	}
	budget := materialBudget(ctx, op, resolved.Policy, task)
	// 当前页先按窗口预读，模型可按nextOffset续读；标题给前置改写，把“这里”写明成具体文档。
	page := answer.RetrieveEvidenceResult{Citations: []gen.Citation{}, EvidenceText: "[]"}
	if currentDocument != "" {
		step(gen.Page, gen.AnswerStepStatusRunning, "阅读当前页面", "", nil)
		read, title, err := answer.ReadCurrentPage(ctx, op, answer.ReadDocumentsInput{TurnID: turnID, DocumentIDs: []string{currentDocument}, Language: record.Language, MaximumTokens: budget})
		if err == nil {
			page = read
			step(gen.Page, gen.AnswerStepStatusDone, "阅读当前页面", "", []string{title})
		} else {
			step(gen.Page, gen.AnswerStepStatusSkipped, "阅读当前页面", "", nil)
		}
	}
	// 主Agent直接结合对话和当前页处理原问题，不再经过前置改写或共享答案缓存。
	overview, _ := qaruntime.ReadSiteOverview(ctx, op)
	// 全站导航预载一次；模型随后自行选择搜索、分区指南和正文读取。
	guide := ""
	step(gen.Search, gen.AnswerStepStatusRunning, "阅读全站指南", "", nil)
	guideBytes := 16000
	if window, windowErr := qaruntime.ResolveTaskWindow(ctx, op, task); windowErr == nil && window > 0 {
		guideBytes = min(guideBytes, max(4096, window/5))
	}
	read, guideErr := answer.ReadAgentGuide(ctx, op, "", 0, guideBytes)
	if guideErr == nil {
		guide = read.Content
		step(gen.Search, gen.AnswerStepStatusDone, "阅读全站指南", "", []string{read.Title})
	} else {
		step(gen.Search, gen.AnswerStepStatusSkipped, "阅读全站指南", "", nil)
	}
	if _, err := takeState(ctx, op, turnID, lease, "generating"); err != nil {
		return err
	}
	prompt, err := answerPrompt(ctx, op, record, resolved, answer.Materials{Overview: overview.Text, Page: page.EvidenceText, Evidence: "[]", Agent: true, Guide: guide}, conversation, task)
	if err != nil {
		return err
	}
	// 保留原有问题快照字段；原生工具历史在独立列保存，后续轮次先复核来源再重放。
	if err := saveModelText(ctx, op, turnID, prompt.Messages[len(prompt.Messages)-1].Content); err != nil {
		return err
	}
	generated, err := answer.GenerateAgentAnswer(ctx, op, answer.AgentAnswerInput{
		TurnID: turnID, TaskID: task, ProfileID: profile, Language: record.Language, Policy: resolved.Policy, Prompt: prompt,
		State:     conversation.AgentState(),
		Citations: append(page.Citations, conversation.AgentCitations()...),
		OnModelStart: func(ctx context.Context, round int) error {
			progress.model(ctx, round, nil)
			return nil
		},
		OnModelDone: func(ctx context.Context, round int, output platform.ModelOutput) error {
			progress.model(ctx, round, &output)
			return nil
		},
		OnToolRound: func(ctx context.Context, output platform.ModelOutput) error {
			if output.Text != nil && *output.Text != "" {
				_, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: "invalidate"}})
				return err
			}
			return nil
		},
		OnToolStart: func(ctx context.Context, call platform.GatewayToolCall) error {
			progress.tool(ctx, call, nil)
			return nil
		},
		OnToolDone: func(ctx context.Context, call platform.GatewayToolCall, result modelagent.ToolResult) error {
			progress.tool(ctx, call, &result)
			return nil
		},
	})
	if err != nil {
		return err
	}
	step(gen.Answer, gen.AnswerStepStatusRunning, "整理引用", "", nil)
	text, citations := answer.FinalizeCitations(generated.Text, generated.Citations)
	if err := history.SaveAgentTurn(ctx, op, turnID, history.AgentTurn{Messages: generated.Messages, Citations: generated.Citations, State: json.RawMessage(generated.State)}); err != nil {
		return err
	}
	if err := saveAgentCounters(ctx, op, turnID, lease, generated.Rounds, generated.ToolCalls); err != nil {
		return err
	}
	for _, citation := range citations {
		c := citation
		if _, err := stream.AppendAnswerEvent(ctx, op, stream.AppendAnswerEventInput{TurnID: turnID, LeaseID: lease, Event: gen.AnswerEvent{Event: gen.AnswerEventEvent("citation"), Citation: &c}}); err != nil {
			return err
		}
	}
	step(gen.Answer, gen.AnswerStepStatusDone, "整理引用", "", nil)
	kind := "answered"
	if len(citations) == 0 {
		kind = "no_evidence"
	}
	if _, err = CompleteTurn(ctx, op, CompleteTurnInput{TurnID: turnID, Answer: text, Citations: citations, LeaseID: lease, AnswerKind: kind, TokenCount: len(text)/4 + 1}); err != nil {
		return err
	}
	return nil
}

func saveAgentCounters(ctx context.Context, op OperationContext, turnID, lease string, rounds, calls int) error {
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return err
	}
	defer tx.Close()
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE turns SET rounds=?,tool_calls=? WHERE turn_id=? AND lease_id=?`, rounds, calls, turnID, lease); err != nil {
		return qaruntime.StorageError(err)
	}
	return tx.Commit()
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

// answerPrompt 保留原始历史；接近模型默认窗口时由 ZCode 原生运行时压缩。
func answerPrompt(ctx context.Context, op OperationContext, record qaruntime.TurnRecord, resolved ResolveTurnContextResult, materials answer.Materials, conversation history.Conversation, task string) (answer.BuildAnswerPromptResult, error) {
	summary := conversation.Summary
	if conversation.AgentState() != "" {
		summary = ""
	}
	return answer.BuildAnswerPrompt(ctx, op, answer.BuildAnswerPromptInput{Question: record.Question, Memory: resolved.Memory, Materials: materials, Summary: summary, GatewayHistory: conversation.ForAgentGateway()})
}

// saveModelText 保留本轮问题快照；原生历史重放使用独立的agent_messages_json。
func saveModelText(ctx context.Context, op OperationContext, turnID, text string) error {
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return err
	}
	defer tx.Close()
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE messages SET model_text=? WHERE message_id=?`, text, "question."+turnID); err != nil {
		return qaruntime.StorageError(err)
	}
	return tx.Commit()
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

// materialBudget 仅分配当前页预读份额，不缩小 Agent 的完整上下文窗口。
func materialBudget(ctx context.Context, op OperationContext, policy gen.TypesQaPolicy, task string) int {
	window, err := qaruntime.ResolveTaskWindow(ctx, op, task)
	if err != nil {
		return 4096
	}
	return max(1, (window-policy.MaximumOutputTokens)/4)
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
	record.AnswerKind = "failed"
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
