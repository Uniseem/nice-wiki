package turn

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"nice-wiki/modules/qa/internal/stream"
	"time"
)

type CompleteTurnInput struct {
	TurnID     string         `json:"turnId"`
	Answer     string         `json:"answer"`
	Citations  []gen.Citation `json:"citations"`
	LeaseID    string         `json:"leaseId"`
	AnswerKind string         `json:"answerKind"`
	TokenCount int            `json:"tokenCount"`
}
type CompleteTurnResult struct {
	Turn    gen.Turn    `json:"turn"`
	Message gen.Message `json:"message"`
}

func CompleteTurn(ctx context.Context, op OperationContext, input CompleteTurnInput) (CompleteTurnResult, error) {
	read, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return CompleteTurnResult{}, err
	}
	record, err := qaruntime.ReadTurn(read, op, input.TurnID, false)
	read.Close()
	if err != nil {
		return CompleteTurnResult{}, err
	}
	if record.Operation.RootOperationID != op.RootOperationID {
		return CompleteTurnResult{}, platform.NewError(platform.CodeForbidden, false, "答案结果不属于原问答根操作")
	}
	if record.State == "completed" {
		return replayCompletedTurn(ctx, op, record, input)
	}
	if err := qaruntime.CheckTurnLease(record, input.LeaseID); err != nil {
		return CompleteTurnResult{}, err
	}
	citations, err := qaruntime.ValidateCitations(ctx, op, input.Citations, record.Language)
	if err != nil {
		return CompleteTurnResult{}, err
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return CompleteTurnResult{}, err
	}
	defer tx.Close()
	current, err := qaruntime.ReadTurn(tx, op, input.TurnID, false)
	if err != nil {
		return CompleteTurnResult{}, err
	}
	if current.State == "completed" {
		tx.Close()
		return replayCompletedTurn(ctx, op, current, input)
	}
	if err := qaruntime.CheckTurnLease(current, input.LeaseID); err != nil {
		return CompleteTurnResult{}, err
	}
	conversation, err := qaruntime.ReadConversation(tx, op, current.Turn.ConversationId, false)
	if err != nil {
		return CompleteTurnResult{}, err
	}
	if conversation.VisitorSubject != current.SubjectID {
		return CompleteTurnResult{}, platform.NewError(platform.CodeIntegrityError, false, "答案会话拥有者不一致")
	}
	var ordinal int
	if err := tx.Tx.QueryRowContext(tx.Context, `SELECT COALESCE(MAX(ordinal)+1,0) FROM messages WHERE conversation_id=?`, conversation.ConversationId).Scan(&ordinal); err != nil {
		return CompleteTurnResult{}, qaruntime.StorageError(err)
	}
	message := gen.Message{MessageId: "answer." + input.TurnID, Ordinal: ordinal, Role: gen.MessageRole("assistant"), Text: input.Answer, TurnId: input.TurnID, Revision: current.Turn.AnswerRevision + 1}
	if _, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO messages(message_id,conversation_id,ordinal,role,text,turn_id,revision,token_count) VALUES(?,?,?,?,?,?,?,?)`, message.MessageId, conversation.ConversationId, message.Ordinal, message.Role, message.Text, message.TurnId, message.Revision, input.TokenCount); err != nil {
		return CompleteTurnResult{}, qaruntime.StorageError(err)
	}
	current.Turn.State = gen.TurnState("completed")
	current.State = "completed"
	current.Turn.AnswerRevision = message.Revision
	current.Answer = input.Answer
	current.Citations = citations
	current.AnswerKind = input.AnswerKind
	hashInput := input
	hashInput.LeaseID = ""
	canonical, _ := platform.CanonicalJSON(hashInput)
	current.CompletionHash = platform.HashBytes(canonical)
	current.TerminalNS = time.Now().UnixNano()
	if err := stream.SaveEvent(tx, &current, gen.AnswerEvent{Event: gen.AnswerEventEvent("completed"), Turn: &current.Turn}, "", nil); err != nil {
		return CompleteTurnResult{}, err
	}
	payload, _ := json.Marshal(struct {
		TurnID          string `json:"turnId"`
		ConversationID  string `json:"conversationId"`
		RootOperationID string `json:"rootOperationId"`
		AnswerRevision  int    `json:"answerRevision"`
		Language        string `json:"language"`
		CacheHit        string `json:"cacheHit"`
		DurationMS      int64  `json:"durationMs"`
		DataEpoch       string `json:"dataEpoch"`
		AnswerKind      string `json:"answerKind"`
	}{input.TurnID, conversation.ConversationId, op.RootOperationID, message.Revision, current.Language, string(current.Turn.CacheHit), max(0, (current.TerminalNS-current.CreatedNS)/int64(time.Millisecond)), op.DataEpoch, input.AnswerKind})
	if err := outbox.EnqueueEvent(tx.Tx, platform.EventEnvelope{EventID: "qa.complete." + input.TurnID, EventType: "qa.turn.completed", EventVersion: 1, Producer: "qa", SubjectID: input.TurnID, Revision: int64(message.Revision), OccurredAt: platform.Now(), DataEpoch: op.DataEpoch, Payload: payload}); err != nil {
		return CompleteTurnResult{}, err
	}
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE conversations SET version=version+1 WHERE conversation_id=?`, conversation.ConversationId); err != nil {
		return CompleteTurnResult{}, qaruntime.StorageError(err)
	}
	if err := tx.Commit(); err != nil {
		return CompleteTurnResult{}, err
	}
	tx.Close()
	if err := releaseTurnPermit(ctx, current); err != nil {
		return CompleteTurnResult{}, err
	}
	return CompleteTurnResult{Turn: current.Turn, Message: message}, nil
}

// replayCompletedTurn 终态回放仍补做guard释放，不追加消息/事件或延长任何费用许可。
func replayCompletedTurn(ctx context.Context, op OperationContext, record qaruntime.TurnRecord, input CompleteTurnInput) (CompleteTurnResult, error) {
	hashInput := input
	hashInput.LeaseID = ""
	canonical, _ := platform.CanonicalJSON(hashInput)
	if record.CompletionHash != platform.HashBytes(canonical) {
		return CompleteTurnResult{}, platform.NewError(platform.CodeConflict, false, "已完成轮次不能覆盖为其他答案")
	}
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return CompleteTurnResult{}, err
	}
	var message gen.Message
	err = tx.Tx.QueryRowContext(tx.Context, `SELECT message_id,ordinal,role,text,turn_id,revision FROM messages WHERE message_id=?`, "answer."+input.TurnID).Scan(&message.MessageId, &message.Ordinal, &message.Role, &message.Text, &message.TurnId, &message.Revision)
	tx.Close()
	if err == sql.ErrNoRows {
		return CompleteTurnResult{}, platform.NewError(platform.CodeIntegrityError, false, "完成终态缺少原答案消息")
	}
	if err != nil {
		return CompleteTurnResult{}, qaruntime.StorageError(err)
	}
	if err := releaseTurnPermit(ctx, record); err != nil {
		return CompleteTurnResult{}, err
	}
	return CompleteTurnResult{Turn: record.Turn, Message: message}, nil
}

func releaseTurnPermit(ctx context.Context, record qaruntime.TurnRecord) error {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return err
	}
	if runtime.Guard == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "准入释放权威未装配")
	}
	raw, err := runtime.Guard.ReleaseAdmission(qaruntime.ServiceReadContext(ctx, record.Operation), record.Permit.AdmissionID, struct{}{}, "qa.release."+record.Turn.TurnId)
	if err != nil {
		return err
	}
	var response struct {
		Released bool `json:"released"`
	}
	if err := qaruntime.DecodeResponse(runtime, "GuardReleasePermitResult", raw, &response); err != nil {
		return err
	}
	if !response.Released {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "准入并发占位尚未释放")
	}
	return nil
}
