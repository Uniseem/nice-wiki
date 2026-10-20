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
	var result StartTurnResult
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return result, err
	}
	replay, err := qaruntime.ReplayReceipt(tx, op, "qa.StartTurn", input.IdempotencyKey, input, &result)
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
	var active int
	if err == nil {
		err = qaruntime.StorageError(read.Tx.QueryRowContext(read.Context, `SELECT COUNT(*) FROM turns WHERE conversation_id=? AND state NOT IN ('completed','cancelled','failed','invalidated')`, input.ConversationId).Scan(&active))
	}
	read.Close()
	if err != nil {
		return result, err
	}
	if !policy.Enabled {
		return result, platform.NewError(platform.CodeForbidden, false, "公开问答尚未开启")
	}
	if active > 0 {
		return result, platform.NewError(platform.CodeConflict, false, "上一个问题还在回答，请稍候")
	}
	if utf8.RuneCountInString(input.Question) > policy.MaximumQuestionCharacters || len(input.Question) > 8192 {
		return result, platform.NewError(platform.CodeValidation, false, "问题过长")
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
	replay, err = qaruntime.ReplayReceipt(tx, op, "qa.StartTurn", input.IdempotencyKey, input, &result)
	if err != nil || replay {
		return result, err
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
	if err := qaruntime.SaveReceipt(tx, op, "qa.StartTurn", input.IdempotencyKey, input, result); err != nil {
		return StartTurnResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return StartTurnResult{}, err
	}
	execution := stored
	execution.Deadline = time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339Nano)
	execution.Actor.ExpiresAt = execution.Deadline
	go ExecuteTurn(qaruntime.WithRuntime(context.Background(), runtime), execution, turn.TurnId)
	return result, nil
}
