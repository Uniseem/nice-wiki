package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
	"time"
)

type qaTurnProjection struct {
	Revision                                                     int64
	EventType, Hash, Day, Locale, AnswerKind, CacheHit, Code, At string
}

// ProjectQaTurn在同一事务去重/单调修订，替换事实时撤旧分类，绝不累计重复事件。
func ProjectQaTurn(ctx context.Context, op OperationContext, input EventDelivery) (EventHandled, error) {
	event := input.Envelope
	if event.Producer != "qa" || event.EventVersion != 1 || (event.EventType != "qa.turn.completed" && event.EventType != "qa.turn.failed") {
		return EventHandled{}, platform.NewError(platform.CodeForbidden, false, "问答统计事件生产者或版本不匹配")
	}
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return EventHandled{}, err
	}
	if event.DataEpoch != rt.Epoch {
		return EventHandled{}, platform.NewError(platform.CodeVersionMismatch, false, "问答统计事件代次不匹配")
	}
	if rt.CountingLocation == nil {
		return EventHandled{}, platform.NewError(platform.CodeDependencyUnavailable, false, "可信报表时区未配置")
	}
	var payload struct {
		TurnID         string `json:"turnId"`
		Language       string `json:"language"`
		CacheHit       string `json:"cacheHit"`
		AnswerKind     string `json:"answerKind"`
		Code           string `json:"code"`
		AnswerRevision int64  `json:"answerRevision"`
		Epoch          string `json:"dataEpoch"`
	}
	if json.Unmarshal(event.Payload, &payload) != nil || payload.Epoch != event.DataEpoch || payload.TurnID != event.SubjectID {
		return EventHandled{}, platform.NewError(platform.CodeIntegrityError, false, "问答统计载荷与信封不一致")
	}
	if event.EventType == "qa.turn.completed" && payload.AnswerRevision != event.Revision {
		return EventHandled{}, platform.NewError(platform.CodeIntegrityError, false, "问答完成修订与信封不同")
	}
	digest, _ := platform.CanonicalJSON([]any{event.Producer, event.EventType, event.SubjectID, event.Revision, event.OccurredAt, event.Payload})
	at, _ := time.Parse(time.RFC3339Nano, event.OccurredAt)
	fact := qaTurnProjection{Revision: event.Revision, EventType: event.EventType, Hash: platform.HashBytes(digest), Day: at.In(rt.CountingLocation).Format("2006-01-02"), Locale: payload.Language, AnswerKind: payload.AnswerKind, CacheHit: payload.CacheHit, Code: payload.Code, At: at.UTC().Format(analyticsTimeLayout)}
	work, _, tx, cancel, err := beginWrite(ctx, op)
	if err != nil {
		return EventHandled{}, err
	}
	defer cancel()
	defer tx.Rollback()
	claimed, err := inbox.ClaimInbox(tx, event)
	if err != nil {
		return EventHandled{}, err
	}
	if !claimed {
		return EventHandled{}, storageError(tx.Commit())
	}
	var old qaTurnProjection
	err = tx.QueryRowContext(work, `SELECT source_revision,event_type,payload_hash,day,locale,answer_kind,cache_hit,error_code,occurred_at FROM qa_turn_projection WHERE turn_id=? AND data_epoch=?`, payload.TurnID, rt.Epoch).Scan(&old.Revision, &old.EventType, &old.Hash, &old.Day, &old.Locale, &old.AnswerKind, &old.CacheHit, &old.Code, &old.At)
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return EventHandled{}, storageError(err)
	}
	if exists && fact.Revision <= old.Revision {
		if fact.Revision == old.Revision && (fact.Hash != old.Hash || fact.EventType != old.EventType) {
			return EventHandled{}, platform.NewError(platform.CodeConflict, false, "同问答修订对应不同原事实")
		}
		if _, err = advanceWatermark(work, tx, "qa-inbox"); err != nil {
			return EventHandled{}, err
		}
		return EventHandled{}, storageError(tx.Commit())
	}
	if exists {
		if err = applyQaCounts(work, tx, rt.Epoch, old, -1); err != nil {
			return EventHandled{}, err
		}
	}
	if err = applyQaCounts(work, tx, rt.Epoch, fact, 1); err != nil {
		return EventHandled{}, err
	}
	if _, err = tx.ExecContext(work, `DELETE FROM unanswered_records WHERE record_id=?`, payload.TurnID); err != nil {
		return EventHandled{}, storageError(err)
	}
	if fact.AnswerKind == "no_evidence" || fact.AnswerKind == "out_of_scope" {
		if _, err = tx.ExecContext(work, `INSERT INTO unanswered_records VALUES(?,?,?,?)`, payload.TurnID, fact.Locale, fact.At, rt.Epoch); err != nil {
			return EventHandled{}, storageError(err)
		}
	}
	_, err = tx.ExecContext(work, `INSERT INTO qa_turn_projection VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(turn_id) DO UPDATE SET source_revision=excluded.source_revision,event_type=excluded.event_type,payload_hash=excluded.payload_hash,day=excluded.day,locale=excluded.locale,answer_kind=excluded.answer_kind,cache_hit=excluded.cache_hit,error_code=excluded.error_code,occurred_at=excluded.occurred_at,data_epoch=excluded.data_epoch`, payload.TurnID, fact.Revision, fact.EventType, fact.Hash, fact.Day, fact.Locale, fact.AnswerKind, fact.CacheHit, fact.Code, fact.At, rt.Epoch)
	if err == nil {
		_, err = advanceWatermark(work, tx, "qa")
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		return EventHandled{}, storageError(err)
	}
	return EventHandled{Applied: true}, nil
}

func applyQaCounts(ctx context.Context, tx *sql.Tx, epoch string, fact qaTurnProjection, delta int64) error {
	kinds := []string{"failed." + fact.Code}
	if fact.EventType == "qa.turn.completed" {
		kinds = []string{"completed", "answer." + fact.AnswerKind, "cache." + fact.CacheHit}
	}
	for _, kind := range kinds {
		if delta > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO qa_turn_counts VALUES(?,?,?,1,?) ON CONFLICT(day,locale,kind) DO UPDATE SET count=count+1`, fact.Day, fact.Locale, kind, epoch); err != nil {
				return storageError(err)
			}
		} else {
			result, err := tx.ExecContext(ctx, `UPDATE qa_turn_counts SET count=count-1 WHERE day=? AND locale=? AND kind=? AND data_epoch=? AND count>0`, fact.Day, fact.Locale, kind, epoch)
			if err != nil {
				return storageError(err)
			}
			count, err := result.RowsAffected()
			if err != nil {
				return storageError(err)
			}
			if count != 1 {
				return platform.NewError(platform.CodeIntegrityError, false, "原问答分类计数与修订事实不同")
			}
		}
	}
	return nil
}
