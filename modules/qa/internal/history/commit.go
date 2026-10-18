package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"time"
)

type OperationContext = platform.OperationContext
type Compaction struct {
	CompactionID           string  `json:"compactionId"`
	ConversationID         string  `json:"conversationId"`
	SourceGeneration       int     `json:"sourceGeneration"`
	Summary                string  `json:"summary"`
	SummaryHash            string  `json:"summaryHash"`
	State                  string  `json:"state"`
	PreviousSummaryHash    *string `json:"previousSummaryHash,omitempty"`
	NewMessageStartOrdinal int     `json:"newMessageStartOrdinal"`
	NewMessageEndOrdinal   int     `json:"newMessageEndOrdinal"`
	RetainedFromOrdinal    int     `json:"retainedFromOrdinal"`
}
type CommitCompactionInput struct {
	Compaction                Compaction `json:"compaction"`
	ExpectedHistoryGeneration int        `json:"expectedHistoryGeneration"`
	LeaseID                   string     `json:"leaseId"`
}
type CommitCompactionResult struct {
	Committed         bool `json:"committed"`
	CurrentGeneration int  `json:"currentGeneration"`
}

func CommitCompaction(ctx context.Context, op OperationContext, input CommitCompactionInput) (CommitCompactionResult, error) {
	if input.Compaction.Summary == "" || platform.HashBytes([]byte(input.Compaction.Summary)) != input.Compaction.SummaryHash {
		return CommitCompactionResult{}, platform.NewError(platform.CodeIntegrityError, false, "压缩摘要为空或摘要hash不一致")
	}
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return CommitCompactionResult{}, err
	}
	defer tx.Close()
	var raw, operationRaw []byte
	var lease, until, state string
	err = tx.Tx.QueryRowContext(tx.Context, `SELECT compaction_json,state,lease_id,lease_until,operation_json FROM compactions WHERE compaction_id=?`, input.Compaction.CompactionID).Scan(&raw, &state, &lease, &until, &operationRaw)
	if err == sql.ErrNoRows {
		return CommitCompactionResult{}, platform.NewError(platform.CodeNotFound, false, "压缩任务不存在")
	}
	if err != nil {
		return CommitCompactionResult{}, qaruntime.StorageError(err)
	}
	var original Compaction
	var originalOp OperationContext
	if qaruntime.DecodeResponse(tx.Runtime, "QaCompaction", raw, &original) != nil || json.Unmarshal(operationRaw, &originalOp) != nil || originalOp.DataEpoch != op.DataEpoch || originalOp.Actor.SiteID != op.Actor.SiteID || originalOp.RootOperationID != op.RootOperationID {
		return CommitCompactionResult{}, platform.NewError(platform.CodeIntegrityError, false, "压缩任务来源不一致")
	}
	if original.ConversationID != input.Compaction.ConversationID || original.SourceGeneration != input.Compaction.SourceGeneration || original.NewMessageStartOrdinal != input.Compaction.NewMessageStartOrdinal || original.NewMessageEndOrdinal != input.Compaction.NewMessageEndOrdinal || original.RetainedFromOrdinal != input.Compaction.RetainedFromOrdinal {
		return CommitCompactionResult{}, platform.NewError(platform.CodeIntegrityError, false, "压缩结果修改了固定输入范围")
	}
	previousInput, _ := platform.CanonicalJSON(input.Compaction.PreviousSummaryHash)
	previousOriginal, _ := platform.CanonicalJSON(original.PreviousSummaryHash)
	if string(previousInput) != string(previousOriginal) {
		return CommitCompactionResult{}, platform.NewError(platform.CodeIntegrityError, false, "压缩结果修改了原摘要引用")
	}
	conversation, err := qaruntime.ReadConversation(tx, op, original.ConversationID, false)
	if err != nil {
		return CommitCompactionResult{}, err
	}
	if state == "committed" {
		if original.SummaryHash != input.Compaction.SummaryHash {
			return CommitCompactionResult{}, platform.NewError(platform.CodeConflict, false, "压缩任务已有不同的提交结果")
		}
		return CommitCompactionResult{Committed: true, CurrentGeneration: conversation.HistoryGeneration}, nil
	}
	expires, err := platform.ParseTimestamp(until)
	if err != nil || lease != input.LeaseID || !expires.After(time.Now()) || state != "running" {
		return CommitCompactionResult{}, platform.NewError(platform.CodeStaleResult, false, "压缩租约已失效")
	}
	compaction := input.Compaction
	if conversation.HistoryGeneration != input.ExpectedHistoryGeneration || original.SourceGeneration != input.ExpectedHistoryGeneration {
		compaction.State = "superseded"
		encoded, _ := platform.CanonicalJSON(compaction)
		if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE compactions SET state='superseded',compaction_json=? WHERE compaction_id=?`, encoded, compaction.CompactionID); err != nil {
			return CommitCompactionResult{}, qaruntime.StorageError(err)
		}
		if err := tx.Commit(); err != nil {
			return CommitCompactionResult{}, err
		}
		return CommitCompactionResult{CurrentGeneration: conversation.HistoryGeneration}, nil
	}
	var previousHash string
	var covered int
	if err := tx.Tx.QueryRowContext(tx.Context, `SELECT summary_hash,summary_until FROM conversations WHERE conversation_id=?`, original.ConversationID).Scan(&previousHash, &covered); err != nil {
		return CommitCompactionResult{}, qaruntime.StorageError(err)
	}
	expectedPrevious := ""
	if original.PreviousSummaryHash != nil {
		expectedPrevious = *original.PreviousSummaryHash
	}
	if previousHash != expectedPrevious || original.NewMessageStartOrdinal != covered || original.NewMessageEndOrdinal < covered || original.RetainedFromOrdinal != original.NewMessageEndOrdinal {
		return CommitCompactionResult{}, platform.NewError(platform.CodeStaleResult, false, "压缩前摘要或连续消息范围已变化")
	}
	result, err := tx.Tx.ExecContext(tx.Context, `UPDATE conversations SET summary=?,summary_hash=?,summary_until=?,history_generation=history_generation+1,version=version+1 WHERE conversation_id=? AND history_generation=?`, compaction.Summary, compaction.SummaryHash, compaction.NewMessageEndOrdinal, original.ConversationID, input.ExpectedHistoryGeneration)
	if err != nil {
		return CommitCompactionResult{}, qaruntime.StorageError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return CommitCompactionResult{}, platform.NewError(platform.CodeStaleResult, false, "压缩提交CAS失败")
	}
	compaction.State = "committed"
	encoded, _ := platform.CanonicalJSON(compaction)
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE compactions SET state='committed',compaction_json=? WHERE compaction_id=?`, encoded, compaction.CompactionID); err != nil {
		return CommitCompactionResult{}, qaruntime.StorageError(err)
	}
	if err := tx.Commit(); err != nil {
		return CommitCompactionResult{}, err
	}
	return CommitCompactionResult{Committed: true, CurrentGeneration: conversation.HistoryGeneration + 1}, nil
}
