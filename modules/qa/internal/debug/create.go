package debug

import (
	"context"
	"fmt"
	"os"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelagent"
	"nice-wiki/modules/qa/internal/answer"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type CreateDebugTurnInput = gen.CreateDebugTurnInput
type CreateDebugTurnResult = gen.CreateDebugTurnResult

// CreateDebugTurn 管理员诊断：不经访客准入、不碰访客会话和缓存，运行同一只读agent，
// 结果（回答、引用、失败原因）写进隔离的诊断记录，保留 7 天。费用按“后台任务”类别计（管理员触发、无访客准入；网关把 admin-probe 限定在连接探测和记忆助手两条专用通道，调试没有借用它）。
func CreateDebugTurn(ctx context.Context, op OperationContext, input CreateDebugTurnInput) (CreateDebugTurnResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.debug"); err != nil {
		return CreateDebugTurnResult{}, err
	}
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return CreateDebugTurnResult{}, err
	}
	if runtime.BaseLanguage == "" {
		return CreateDebugTurnResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "站点基准语言暂不可读，请稍后再试")
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
	guide, err := answer.ReadAgentGuide(ctx, op, "", 0, 16000)
	if err != nil {
		return err
	}
	overview, _ := qaruntime.ReadSiteOverview(ctx, op)
	memory, err := qaruntime.ResolveTaskMemory(ctx, op, "qa.answer", input.Language)
	if err != nil {
		return err
	}
	prompt, err := answer.BuildAnswerPrompt(ctx, op, answer.BuildAnswerPromptInput{Question: input.Question, Memory: memory, Materials: answer.Materials{Evidence: "[]", Overview: overview.Text, Agent: true, Guide: guide.Content}, History: []gen.Message{}})
	if err != nil {
		return err
	}
	runtime.OnModelEvent = func(event platform.StreamEvent) error {
		if event.Event == platform.StreamDelta && event.Delta != "" {
			text := event.Delta
			emit(gen.AnswerEvent{Event: "delta", Delta: &text})
		}
		return nil
	}
	ctx = qaruntime.WithRuntime(ctx, runtime)
	// 档位以模型接入里的任务分配为准（网关只接受与任务路线一致的档位），解析不到时才用请求里给的。
	profile := input.ProfileId
	if routed, err := qaruntime.ResolveTaskProfile(ctx, op, "qa.answer"); err == nil {
		profile = platform.Identifier(routed)
	}
	generated, err := answer.GenerateAgentAnswer(ctx, op, answer.AgentAnswerInput{
		TurnID: record.Turn.TurnId, TaskID: "qa.answer", ProfileID: string(profile), Language: input.Language, Policy: policy, Prompt: prompt,
		OnToolRound: func(context.Context, platform.ModelOutput) error {
			emit(gen.AnswerEvent{Event: "invalidate"})
			return nil
		},
		OnToolDone: func(_ context.Context, _ platform.GatewayToolCall, tool modelagent.ToolResult) error {
			items := []string{tool.Title}
			emit(gen.AnswerEvent{Event: "step", Step: &gen.AnswerStep{Key: gen.Search, Status: gen.AnswerStepStatusDone, Title: "查阅文档", Items: &items}})
			return nil
		},
	})
	if err != nil {
		return err
	}
	text, citations := answer.FinalizeCitations(generated.Text, generated.Citations)
	checked, err := qaruntime.ValidateCitations(ctx, op, citations, input.Language)
	if err != nil {
		return err
	}
	if text != generated.Text {
		emit(gen.AnswerEvent{Event: "invalidate"})
		emit(gen.AnswerEvent{Event: "delta", Delta: &text})
	}
	for _, citation := range checked {
		c := citation
		emit(gen.AnswerEvent{Event: gen.AnswerEventEvent("citation"), Citation: &c})
	}
	return nil
}
