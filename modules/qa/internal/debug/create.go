package debug

import (
	"context"
	"fmt"
	"os"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/answer"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/normalize"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type CreateDebugTurnInput = gen.CreateDebugTurnInput
type CreateDebugTurnResult = gen.CreateDebugTurnResult

// CreateDebugTurn 管理员诊断：不经访客准入、不碰访客会话和缓存，用指定档位直接跑一次“检索 → 回答”，
// 结果（回答、引用、失败原因）写进隔离的诊断记录，保留 7 天。费用按“后台任务”类别计（管理员触发、无访客准入；网关把 admin-probe 限定在连接探测和记忆助手两条专用通道，调试没有借用它）。
func CreateDebugTurn(ctx context.Context, op OperationContext, input CreateDebugTurnInput) (CreateDebugTurnResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.debug"); err != nil {
		return CreateDebugTurnResult{}, err
	}
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return CreateDebugTurnResult{}, err
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return CreateDebugTurnResult{}, err
	}
	defer tx.Close()
	var result CreateDebugTurnResult
	if replay, err := qaruntime.ReplayReceipt(tx, op, "qa.CreateDebugTurn", input.IdempotencyKey, input, &result); err != nil || replay {
		return result, err
	}
	policy, err := qaruntime.ReadPolicy(tx)
	if err != nil {
		return CreateDebugTurnResult{}, err
	}
	if utf8.RuneCountInString(input.Question) > policy.MaximumQuestionCharacters {
		return CreateDebugTurnResult{}, platform.NewError(platform.CodeValidation, false, "诊断问题过长")
	}
	debugID := platform.NewIdentifier()
	turn := gen.Turn{TurnId: platform.NewIdentifier(), ConversationId: debugID, RootOperationId: op.RootOperationID, State: gen.TurnState("queued"), QuestionHash: platform.HashBytes([]byte(input.Question)), ContextHash: platform.HashBytes(nil), CacheHit: gen.TurnCacheHit("none")}
	record := gen.DebugTurn{DebugId: debugID, ActorId: op.Actor.SubjectID, Namespace: gen.DebugTurnNamespace("debug"), Turn: turn, ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour)}
	raw, err := platform.CanonicalJSON(record)
	if err != nil {
		return CreateDebugTurnResult{}, err
	}
	if _, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO debug_turns(debug_id,site_id,data_epoch,actor_id,debug_json,expires_ns) VALUES(?,?,?,?,?,?)`, debugID, op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, raw, record.ExpiresAt.UnixNano()); err != nil {
		return CreateDebugTurnResult{}, qaruntime.StorageError(err)
	}
	result = CreateDebugTurnResult{Debug: record}
	if err := qaruntime.SaveReceipt(tx, op, "qa.CreateDebugTurn", input.IdempotencyKey, input, result); err != nil {
		return CreateDebugTurnResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CreateDebugTurnResult{}, err
	}
	execution := op
	execution.Deadline = time.Now().UTC().Add(3 * time.Minute).Format(time.RFC3339Nano)
	execution.Actor.ExpiresAt = execution.Deadline
	go runDebugTurn(qaruntime.WithRuntime(context.Background(), runtime), execution, record, input, policy)
	return result, nil
}

func runDebugTurn(ctx context.Context, op OperationContext, record gen.DebugTurn, input CreateDebugTurnInput, policy gen.TypesQaPolicy) {
	work, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	sequence := 0
	emit := func(event gen.AnswerEvent) {
		sequence++
		event.Sequence = sequence
		raw, _ := platform.CanonicalJSON(event)
		tx, err := qaruntime.BeginWrite(work, op)
		if err != nil {
			return
		}
		defer tx.Close()
		if _, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO debug_events(debug_id,sequence,event_json) VALUES(?,?,?)`, record.DebugId, sequence, raw); err == nil {
			_ = tx.Commit()
		}
	}
	finish := func(state string, code string) {
		record.Turn.State = gen.TurnState(state)
		record.Turn.LastSequence = sequence + 1
		event := gen.AnswerEvent{Event: gen.AnswerEventEvent(map[string]string{"completed": "completed", "failed": "failed"}[state]), Turn: &record.Turn}
		if code != "" {
			event.Code = &code
		}
		emit(event)
		raw, _ := platform.CanonicalJSON(record)
		if tx, err := qaruntime.BeginWrite(work, op); err == nil {
			defer tx.Close()
			if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE debug_turns SET debug_json=? WHERE debug_id=?`, raw, record.DebugId); err == nil {
				_ = tx.Commit()
			}
		}
	}
	if err := executeDebug(work, op, record, input, policy, emit); err != nil {
		if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "诊断轮次失败 debug=%s costClass=%q err=%v\n", record.DebugId, op.CostClass, err)
		}
		code := platform.CodeInternal
		if failure, ok := err.(*platform.Error); ok {
			code = failure.Code
		}
		finish("failed", code)
		return
	}
	finish("completed", "")
}

func executeDebug(ctx context.Context, op OperationContext, record gen.DebugTurn, input CreateDebugTurnInput, policy gen.TypesQaPolicy, emit func(gen.AnswerEvent)) error {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return err
	}
	canonical, err := normalize.NormalizeQuestion(ctx, op, normalize.NormalizeQuestionInput{Question: input.Question, Language: input.Language})
	if err != nil {
		return err
	}
	evidence, err := answer.RetrieveEvidence(ctx, op, answer.RetrieveEvidenceInput{TurnID: record.Turn.TurnId, Normalized: canonical.Normalized, Policy: policy, Language: input.Language})
	if err != nil {
		return err
	}
	memory, err := qaruntime.ResolveTaskMemory(ctx, op, "qa.answer", input.Language)
	if err != nil {
		return err
	}
	prompt, err := answer.BuildAnswerPrompt(ctx, op, answer.BuildAnswerPromptInput{Question: input.Question, Memory: memory, EvidenceText: evidence.EvidenceText, History: []gen.Message{}})
	if err != nil {
		return err
	}
	_ = runtime
	outcome, err := qaruntime.InvokeModel(ctx, op, qaruntime.InvokeModelInput{Request: platform.CreateInvocationInput{TaskID: "qa.answer", ProfileID: input.ProfileId, Capability: platform.CapabilityChat, Messages: prompt.Messages, MaximumOutputTokens: int64(min(prompt.MaximumOutputTokens, policy.MaximumOutputTokens)), IDempotencyKey: "debug.answer"}})
	if err != nil {
		return err
	}
	text, err := qaruntime.ModelText(outcome)
	if err != nil {
		return err
	}
	emit(gen.AnswerEvent{Event: gen.AnswerEventEvent("delta"), Delta: &text})
	for _, citation := range evidence.Citations {
		c := citation
		emit(gen.AnswerEvent{Event: gen.AnswerEventEvent("citation"), Citation: &c})
	}
	return nil
}
