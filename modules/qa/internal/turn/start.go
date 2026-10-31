package turn

import (
	"context"
	"encoding/json"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/admission"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"nice-wiki/modules/qa/internal/stream"
)

type StartTurnInput = gen.StartTurnInput
type StartTurnResult = gen.StartTurnResult

// StartTurn 接受访客提问：校验会话与准入、写入问题与 queued 轮次，随后在本进程里异步执行回答。
func StartTurn(ctx context.Context, op OperationContext, input StartTurnInput) (StartTurnResult, error) {
	if op.CallerService != "portal" || op.Actor.Kind != "visitor" {
		return StartTurnResult{}, platform.NewError(platform.CodeForbidden, false, "仅允许当前伪名访客提问")
	}
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return StartTurnResult{}, err
	}
	if runtime.BaseLanguage == "" {
		return StartTurnResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "站点基准语言暂不可读，请稍后再试")
	}
	var result StartTurnResult
	// 业务幂等只看访客提交的内容：准入是入口每次按原根交来的短期凭据（续租还会改到期时间），不算业务输入。
	request := input
	request.Admission = gen.AdmissionPermit{}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return result, err
	}
	replay, err := qaruntime.ReplayReceipt(tx, op, "qa.StartTurn", input.IdempotencyKey, request, &result)
	tx.Close()
	if err != nil || replay {
		return result, err
	}
	read, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return result, err
	}
	conversation, err := qaruntime.ReadConversation(read, op, input.ConversationId, true)
	var policy gen.TypesQaPolicy
	if err == nil {
		policy, err = qaruntime.ReadPolicy(read)
	}
	var active, today int
	var dailyCap int
	if err == nil {
		dailyCap = runtime.VisitorDailyCap(ctx, op)
		if dailyCap > 0 {
			dayStart := time.Now().UTC().Truncate(24 * time.Hour).UnixNano()
			err = qaruntime.StorageError(read.Tx.QueryRowContext(read.Context, `SELECT COUNT(*) FROM turns WHERE site_id=? AND created_ns>=?`, op.Actor.SiteID, dayStart).Scan(&today))
		}
	}
	if err == nil {
		err = qaruntime.StorageError(read.Tx.QueryRowContext(read.Context, `SELECT COUNT(*) FROM turns WHERE conversation_id=? AND state NOT IN ('completed','cancelled','failed','invalidated')`, input.ConversationId).Scan(&active))
	}
	read.Close()
	switch {
	case err != nil:
	case !policy.Enabled:
		err = platform.NewError(platform.CodeForbidden, false, "公开问答尚未开启")
	case dailyCap > 0 && today >= dailyCap:
		runtime.NoteVisitorCapReached(ctx, op)
		err = platform.NewError(platform.CodeRateLimited, true, "今天这个站点的提问次数已达上限，请明天再来")
	case active > 0:
		err = platform.NewError(platform.CodeConflict, false, "上一个问题还在回答，请稍候")
	case utf8.RuneCountInString(input.Question) > policy.MaximumQuestionCharacters || len(input.Question) > 8192:
		err = platform.NewError(platform.CodeValidation, false, "问题过长")
	}
	if err != nil {
		// 入口为这次提问领的准入还没用就被拒了（比如会话已过期）：释放占位，访客马上换新会话再问不会被并发上限挡住。
		if runtime.Guard != nil {
			_, _ = runtime.Guard.ReleaseAdmission(qaruntime.ServiceReadContext(ctx, op), input.Admission.AdmissionId, struct{}{}, "qa.turn.release."+platform.HashBytes([]byte(op.RootOperationID+":"+input.Admission.AdmissionId)))
		}
		return result, err
	}
	permit := platform.AdmissionPermit{AdmissionID: input.Admission.AdmissionId, RequestID: input.Admission.RequestId, SubjectID: input.Admission.SubjectId, Action: input.Admission.Action, Token: input.Admission.Token, ExpiresAt: input.Admission.ExpiresAt.UTC().Format(time.RFC3339Nano), SessionBudgetScope: input.Admission.SessionBudgetScope, NetworkBudgetScope: input.Admission.NetworkBudgetScope, DataEpoch: input.Admission.DataEpoch, Audience: string(input.Admission.Audience)}
	validated, err := admission.ValidateAdmission(ctx, op, admission.ValidateAdmissionInput{Admission: permit, ConversationID: input.ConversationId, Action: "qa.ask"})
	if err != nil {
		return result, err
	}
	tx, err = qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return result, err
	}
	defer tx.Close()
	replay, err = qaruntime.ReplayReceipt(tx, op, "qa.StartTurn", input.IdempotencyKey, request, &result)
	if err != nil || replay {
		return result, err
	}
	// 上面的计数在只读事务里，并发提问可能都看到没满；写事务互斥，在这里按同一口径再数一次才算数。
	if dailyCap > 0 {
		dayStart := time.Now().UTC().Truncate(24 * time.Hour).UnixNano()
		if err := tx.Tx.QueryRowContext(tx.Context, `SELECT COUNT(*) FROM turns WHERE site_id=? AND created_ns>=?`, op.Actor.SiteID, dayStart).Scan(&today); err != nil {
			return result, qaruntime.StorageError(err)
		}
		if today >= dailyCap {
			tx.Close()
			runtime.NoteVisitorCapReached(ctx, op)
			return result, platform.NewError(platform.CodeRateLimited, true, "今天这个站点的提问次数已达上限，请明天再来")
		}
	}
	stored := op
	stored.CostClass = platform.CostClassQA
	stored.LoginNetworkScopes, stored.NetworkBudgetScopes = nil, nil
	turn := gen.Turn{TurnId: platform.NewIdentifier(), ConversationId: input.ConversationId, RootOperationId: op.RootOperationID, State: gen.TurnState("queued"), QuestionHash: platform.HashBytes([]byte(input.Question)), ContextHash: platform.HashBytes(nil), CacheHit: gen.TurnCacheHit("none")}
	rawTurn, _ := platform.CanonicalJSON(turn)
	rawPermit, _ := json.Marshal(validated.Permit)
	rawOperation, _ := json.Marshal(stored)
	now := time.Now()
	if _, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO turns(turn_id,conversation_id,site_id,data_epoch,visitor_subject,language,turn_json,question,state,created_at,created_ns,permit_json,operation_json) VALUES(?,?,?,?,?,?,?,?,'queued',?,?,?,?)`, turn.TurnId, input.ConversationId, op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, conversation.Language, rawTurn, input.Question, now.UTC().Format(time.RFC3339Nano), now.UnixNano(), rawPermit, rawOperation); err != nil {
		return result, qaruntime.StorageError(err)
	}
	var ordinal int
	if err := tx.Tx.QueryRowContext(tx.Context, `SELECT COALESCE(MAX(ordinal)+1,0) FROM messages WHERE conversation_id=?`, input.ConversationId).Scan(&ordinal); err != nil {
		return result, qaruntime.StorageError(err)
	}
	if _, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO messages(message_id,conversation_id,ordinal,role,text,turn_id,revision,token_count) VALUES(?,?,?,'user',?,?,1,0)`, "question."+turn.TurnId, input.ConversationId, ordinal, input.Question, turn.TurnId); err != nil {
		return result, qaruntime.StorageError(err)
	}
	record := qaruntime.TurnRecord{Turn: turn}
	if err := stream.SaveEvent(tx, &record, gen.AnswerEvent{Event: gen.AnswerEventEvent("accepted"), Turn: &record.Turn}, "", nil); err != nil {
		return result, err
	}
	result = StartTurnResult{Turn: record.Turn, StreamPath: "/v1/turns/" + turn.TurnId + "/events"}
	if err := qaruntime.SaveReceipt(tx, op, "qa.StartTurn", input.IdempotencyKey, request, result); err != nil {
		return StartTurnResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return StartTurnResult{}, err
	}
	execution := stored
	execution.Deadline = time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339Nano)
	execution.Actor.ExpiresAt = execution.Deadline
	// 访客所在页面只用于本轮回答（交给回答模型的预读材料），随执行一起交过去，不另存。
	current := ""
	if input.CurrentDocumentId != nil {
		current = string(*input.CurrentDocumentId)
	}
	go ExecuteTurn(qaruntime.WithRuntime(context.Background(), runtime), execution, turn.TurnId, current)
	return result, nil
}
