package stream

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"time"
	"unicode/utf8"
)

type OperationContext = platform.OperationContext
type AppendAnswerEventInput struct {
	TurnID         string          `json:"turnId"`
	Event          gen.AnswerEvent `json:"event"`
	LeaseID        string          `json:"leaseId"`
	SourceStep     string          `json:"sourceStep,omitempty"`
	SourceSequence *int64          `json:"sourceSequence,omitempty"`
}
type AppendAnswerEventResult struct {
	Sequence int `json:"sequence"`
}

func AppendAnswerEvent(ctx context.Context, op OperationContext, input AppendAnswerEventInput) (AppendAnswerEventResult, error) {
	if (input.SourceStep == "") != (input.SourceSequence == nil) {
		return AppendAnswerEventResult{}, platform.NewError(platform.CodeValidation, false, "流事件来源身份和序号须同时提供")
	}
	switch string(input.Event.Event) {
	case "accepted", "progress", "step", "delta", "citation", "invalidate":
	default:
		return AppendAnswerEventResult{}, platform.NewError(platform.CodeValidation, false, "终态事件必须与轮次终态同事务写入")
	}
	if input.Event.Delta != nil && (!utf8.ValidString(*input.Event.Delta) || len(*input.Event.Delta) > 16384) {
		return AppendAnswerEventResult{}, platform.NewError(platform.CodeValidation, false, "流式片段超过预算或UTF8不完整")
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return AppendAnswerEventResult{}, err
	}
	defer tx.Close()
	record, err := qaruntime.ReadTurn(tx, op, input.TurnID, false)
	if err != nil {
		return AppendAnswerEventResult{}, err
	}
	if err := qaruntime.CheckTurnLease(record, input.LeaseID); err != nil {
		return AppendAnswerEventResult{}, err
	}
	if input.SourceStep != "" && input.SourceSequence != nil {
		var raw []byte
		err := tx.Tx.QueryRowContext(tx.Context, `SELECT event_json FROM turn_events WHERE turn_id=? AND source_step=? AND source_sequence=?`, input.TurnID, input.SourceStep, *input.SourceSequence).Scan(&raw)
		if err == nil {
			var previous gen.AnswerEvent
			if json.Unmarshal(raw, &previous) != nil {
				return AppendAnswerEventResult{}, platform.NewError(platform.CodeIntegrityError, false, "原流事件损坏")
			}
			expected := input.Event
			expected.Sequence = previous.Sequence
			a, _ := platform.CanonicalJSON(previous)
			b, _ := platform.CanonicalJSON(expected)
			if string(a) != string(b) {
				return AppendAnswerEventResult{}, platform.NewError(platform.CodeIntegrityError, false, "同源流事件序号对应不同内容")
			}
			return AppendAnswerEventResult{Sequence: previous.Sequence}, nil
		}
		if err != sql.ErrNoRows {
			return AppendAnswerEventResult{}, qaruntime.StorageError(err)
		}
	}
	event := input.Event
	event.Sequence = record.Turn.LastSequence + 1
	if err := SaveEvent(tx, &record, event, input.SourceStep, input.SourceSequence); err != nil {
		return AppendAnswerEventResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return AppendAnswerEventResult{}, err
	}
	return AppendAnswerEventResult{Sequence: event.Sequence}, nil
}

// SaveEvent 由已持有短事务且校验租约/终态的调用者使用；提交后读者才可见。
func SaveEvent(tx *qaruntime.Transaction, record *qaruntime.TurnRecord, event gen.AnswerEvent, source string, sequence *int64) error {
	event.Sequence = record.Turn.LastSequence + 1
	if event.Turn != nil {
		current := *event.Turn
		current.LastSequence = event.Sequence
		event.Turn = &current
	}
	raw, err := platform.CanonicalJSON(event)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "流事件无法编码")
	}
	var step any
	if source != "" {
		step = source
	}
	_, err = tx.Tx.ExecContext(tx.Context, `INSERT INTO turn_events(turn_id,sequence,event_json,created_ns,source_step,source_sequence) VALUES(?,?,?,?,?,?)`, record.Turn.TurnId, event.Sequence, raw, time.Now().UnixNano(), step, sequence)
	if err != nil {
		return qaruntime.StorageError(err)
	}
	record.Turn.LastSequence = event.Sequence
	return qaruntime.SaveTurn(tx, *record)
}
