package feedback

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/qa/internal/cache"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type OperationContext = platform.OperationContext
type RecordFeedbackInput = gen.RecordFeedbackInput
type RecordFeedbackResult = gen.RecordFeedbackResult

func RecordFeedback(ctx context.Context, op OperationContext, input RecordFeedbackInput) (RecordFeedbackResult, error) {
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return RecordFeedbackResult{}, err
	}
	defer tx.Close()
	var result RecordFeedbackResult
	replay, err := qaruntime.ReplayReceipt(tx, op, "qa.RecordFeedback", input.IdempotencyKey, input, &result)
	if err != nil {
		return result, err
	}
	if replay {
		return result, nil
	}
	record, err := qaruntime.ReadTurn(tx, op, input.TurnId, true)
	if err != nil {
		return result, err
	}
	if record.State != "completed" || record.Turn.AnswerRevision != input.AnswerRevision {
		return result, platform.NewError(platform.CodeVersionMismatch, false, "反馈答案版本不是当前完成答案")
	}
	var version int
	err = tx.Tx.QueryRowContext(tx.Context, `SELECT version FROM feedback WHERE turn_id=? AND visitor_subject=?`, input.TurnId, op.Actor.SubjectID).Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		return result, qaruntime.StorageError(err)
	}
	version++
	_, err = tx.Tx.ExecContext(tx.Context, `INSERT INTO feedback(turn_id,visitor_subject,answer_revision,value,version) VALUES(?,?,?,?,?) ON CONFLICT(turn_id,visitor_subject) DO UPDATE SET answer_revision=excluded.answer_revision,value=excluded.value,version=excluded.version`, input.TurnId, op.Actor.SubjectID, input.AnswerRevision, input.Value, version)
	if err != nil {
		return result, qaruntime.StorageError(err)
	}
	if string(input.Value) == "unhelpful" {
		rows, err := tx.Tx.QueryContext(tx.Context, `SELECT cache_id FROM answer_cache WHERE source_turn_id=? AND curated=0 AND state='active' AND site_id=? AND data_epoch=?`, input.TurnId, op.Actor.SiteID, op.DataEpoch)
		if err != nil {
			return result, qaruntime.StorageError(err)
		}
		ids := make([]string, 0)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return result, qaruntime.StorageError(err)
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, qaruntime.StorageError(err)
		}
		for _, id := range ids {
			cached, err := cache.ReadCache(tx, op, id)
			if err != nil {
				return result, err
			}
			cached.Entry.Version++
			cached.Entry.State = gen.CacheEntryState("stale")
			if err := cache.SaveCache(tx, op, cached); err != nil {
				return result, err
			}
			if err := cache.EmitCacheInvalidated(tx, op, cached.Entry, "unhelpful"); err != nil {
				return result, err
			}
		}
	}
	payload, _ := json.Marshal(struct {
		TurnID          string `json:"turnId"`
		AnswerRevision  int    `json:"answerRevision"`
		Value           string `json:"value"`
		FeedbackVersion int    `json:"feedbackVersion"`
		DataEpoch       string `json:"dataEpoch"`
	}{input.TurnId, input.AnswerRevision, string(input.Value), version, op.DataEpoch})
	if err := outbox.EnqueueEvent(tx.Tx, platform.EventEnvelope{EventID: "qa.feedback." + platform.HashBytes(payload), EventType: "qa.feedback.changed", EventVersion: 1, Producer: "qa", SubjectID: input.TurnId, Revision: int64(version), OccurredAt: platform.Now(), DataEpoch: op.DataEpoch, Payload: payload}); err != nil {
		return result, err
	}
	result = RecordFeedbackResult{Version: version, Value: gen.RecordFeedbackResultValue(input.Value)}
	if err := qaruntime.SaveReceipt(tx, op, "qa.RecordFeedback", input.IdempotencyKey, input, result); err != nil {
		return RecordFeedbackResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return RecordFeedbackResult{}, err
	}
	return result, nil
}
