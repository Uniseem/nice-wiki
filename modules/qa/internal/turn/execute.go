package turn

import (
	"context"
	"fmt"
	"os"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/answer"
	"nice-wiki/modules/qa/internal/gen"
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
	canonical, err := normalize.NormalizeQuestion(ctx, op, normalize.NormalizeQuestionInput{Question: record.Question, Language: record.Language})
	if err != nil {
		return err
	}
	history, err := recentHistory(ctx, op, record.Turn.ConversationId, turnID)
	if err != nil {
		return err
	}
	var cp *cachePlan
	if resolved.Policy.CacheEnabled {
		if cp, ctx, err = planForCache(ctx, op, turnID, record, resolved, history); err != nil {
			return err
		}
		if cp != nil {
			if hit, err := answerFromCache(ctx, op, turnID, lease, record, resolved, cp, len(history) == 0); err != nil || hit {
				return err
			}
		}
	}
	evidence, err := answer.RetrieveEvidence(ctx, op, answer.RetrieveEvidenceInput{TurnID: turnID, Normalized: canonical.Normalized, Policy: resolved.Policy, Language: record.Language})
	if err != nil {
		return err
	}
	if _, err := takeState(ctx, op, turnID, lease, "generating"); err != nil {
		return err
	}
	prompt, err := answer.BuildAnswerPrompt(ctx, op, answer.BuildAnswerPromptInput{Question: record.Question, Memory: resolved.Memory, EvidenceText: evidence.EvidenceText, History: history})
	if err != nil {
		return err
	}
	outcome, err := qaruntime.InvokeModel(ctx, op, qaruntime.InvokeModelInput{Stream: true, Request: platform.CreateInvocationInput{TaskID: "qa.answer", ProfileID: resolved.Policy.ProfileId, Capability: platform.CapabilityChat, Messages: prompt.Messages, MaximumOutputTokens: int64(min(prompt.MaximumOutputTokens, resolved.Policy.MaximumOutputTokens)), IDempotencyKey: "answer"}})
	if err != nil {
		return err
	}
	text, err := qaruntime.ModelText(outcome)
	if err != nil {
		return err
	}
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
	if cp != nil && kind == "answered" {
		writeCache(ctx, op, turnID, cp, text, resolved.Memory.VersionHash)
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

// recentHistory 取本会话此前已完成的最近几条消息作为上下文。
func recentHistory(ctx context.Context, op OperationContext, conversationID, turnID string) ([]gen.Message, error) {
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return nil, err
	}
	defer tx.Close()
	rows, err := tx.Tx.QueryContext(tx.Context, `SELECT message_id,ordinal,role,text,turn_id,revision FROM (SELECT * FROM messages WHERE conversation_id=? AND turn_id<>? ORDER BY ordinal DESC LIMIT 6) ORDER BY ordinal`, conversationID, turnID)
	if err != nil {
		return nil, qaruntime.StorageError(err)
	}
	defer rows.Close()
	var history []gen.Message
	for rows.Next() {
		var message gen.Message
		if err := rows.Scan(&message.MessageId, &message.Ordinal, &message.Role, &message.Text, &message.TurnId, &message.Revision); err != nil {
			return nil, qaruntime.StorageError(err)
		}
		history = append(history, message)
	}
	return history, rows.Err()
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
