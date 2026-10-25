package runtime

import (
	"context"
	"nice-wiki/libs/platform"
	"time"
)

type PruneExpiredRecordsInput struct {
	Now   time.Time `json:"now"`
	Limit int       `json:"limit"`
}
type PruneExpiredRecordsResult struct {
	DeletedConversations int `json:"deletedConversations"`
	DeletedDebug         int `json:"deletedDebug"`
	DeletedEvents        int `json:"deletedEvents"`
}

// RedactConversation 清除正文和派生上下文，保留无正文轮次标识、状态及模型账单引用。
func RedactConversation(tx *Transaction, id, state string) error {
	statements := []string{
		`DELETE FROM messages WHERE conversation_id=?`,
		`DELETE FROM feedback WHERE turn_id IN (SELECT turn_id FROM turns WHERE conversation_id=?)`,
		`DELETE FROM turn_events WHERE turn_id IN (SELECT turn_id FROM turns WHERE conversation_id=?)`,
		`DELETE FROM compactions WHERE conversation_id=?`,
		`UPDATE turns SET question='',canonical_question='',english_question='',answer='',citations_json=X'5B5D',permit_json=X'7B7D',plan_json=NULL,current_page_json=NULL,redacted=1 WHERE conversation_id=?`,
	}
	for _, statement := range statements {
		if _, err := tx.Tx.ExecContext(tx.Context, statement, id); err != nil {
			return StorageError(err)
		}
	}
	_, err := tx.Tx.ExecContext(tx.Context, `UPDATE conversations SET state=?,summary='',summary_hash='',summary_until=0,version=version+1 WHERE conversation_id=?`, state, id)
	return StorageError(err)
}

func PruneExpiredRecords(ctx context.Context, op OperationContext, input PruneExpiredRecordsInput) (PruneExpiredRecordsResult, error) {
	tx, err := BeginWrite(ctx, op)
	if err != nil {
		return PruneExpiredRecordsResult{}, err
	}
	defer tx.Close()
	result := PruneExpiredRecordsResult{}
	policy, err := ReadPolicy(tx)
	if err != nil {
		return result, err
	}
	if _, err := tx.Tx.ExecContext(tx.Context, `DELETE FROM qa_receipts WHERE created_ns<=?`, input.Now.Add(-time.Duration(policy.RecordRetentionDays)*24*time.Hour).UnixNano()); err != nil {
		return result, StorageError(err)
	}
	rows, err := tx.Tx.QueryContext(tx.Context, `SELECT conversation_id,expires_at FROM conversations c WHERE site_id=? AND data_epoch=? AND state='active' AND NOT EXISTS(SELECT 1 FROM turns t WHERE t.conversation_id=c.conversation_id AND t.state IN ('admitted','queued','retrieving','generating')) ORDER BY expires_at,conversation_id LIMIT ?`, op.Actor.SiteID, op.DataEpoch, input.Limit)
	if err != nil {
		return result, StorageError(err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id, expiresRaw string
		if err := rows.Scan(&id, &expiresRaw); err != nil {
			rows.Close()
			return result, StorageError(err)
		}
		expires, err := platform.ParseTimestamp(expiresRaw)
		if err != nil {
			rows.Close()
			return result, platform.NewError(platform.CodeIntegrityError, false, "会话保留期时间损坏")
		}
		if !expires.After(input.Now) {
			ids = append(ids, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, StorageError(err)
	}
	for _, id := range ids {
		if err := RedactConversation(tx, id, "expired"); err != nil {
			return result, err
		}
		result.DeletedConversations++
	}
	rows, err = tx.Tx.QueryContext(tx.Context, `SELECT debug_id FROM debug_turns WHERE site_id=? AND data_epoch=? AND expires_ns<=? ORDER BY expires_ns,debug_id LIMIT ?`, op.Actor.SiteID, op.DataEpoch, input.Now.UnixNano(), input.Limit)
	if err != nil {
		return result, StorageError(err)
	}
	ids = make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return result, StorageError(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, StorageError(err)
	}
	for _, id := range ids {
		if _, err := tx.Tx.ExecContext(tx.Context, `DELETE FROM debug_events WHERE debug_id=?`, id); err != nil {
			return result, StorageError(err)
		}
		if _, err := tx.Tx.ExecContext(tx.Context, `DELETE FROM debug_turns WHERE debug_id=?`, id); err != nil {
			return result, StorageError(err)
		}
		result.DeletedDebug++
	}
	deleted, err := tx.Tx.ExecContext(tx.Context, `DELETE FROM turn_events WHERE rowid IN (SELECT e.rowid FROM turn_events e JOIN turns t ON t.turn_id=e.turn_id WHERE t.site_id=? AND t.data_epoch=? AND t.state IN ('completed','cancelled','failed','invalidated') AND t.terminal_ns>0 AND t.terminal_ns<=? ORDER BY e.created_ns,e.turn_id,e.sequence LIMIT ?)`, op.Actor.SiteID, op.DataEpoch, input.Now.Add(-24*time.Hour).UnixNano(), input.Limit)
	if err != nil {
		return result, StorageError(err)
	}
	count, err := deleted.RowsAffected()
	if err != nil {
		return result, StorageError(err)
	}
	result.DeletedEvents = int(count)
	if err := tx.Commit(); err != nil {
		return PruneExpiredRecordsResult{}, err
	}
	return result, nil
}
