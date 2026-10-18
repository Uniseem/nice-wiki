package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"time"
)

type CacheRecord struct {
	Entry           gen.CacheEntry
	SourceTurnID    string
	PublicationHash string
}

func ScopeHash(op OperationContext, policy gen.TypesQaPolicy, language string) string {
	raw, _ := platform.CanonicalJSON(struct {
		Site, Epoch, Version, Language string
		Space                          gen.EmbeddingSpace
	}{op.Actor.SiteID, op.DataEpoch, "qa.normalize.v1", language, policy.RetrievalSpace})
	return platform.HashBytes(raw)
}
func ReadCache(tx *qaruntime.Transaction, op OperationContext, id string) (CacheRecord, error) {
	var record CacheRecord
	var raw []byte
	var version int
	var state string
	err := tx.Tx.QueryRowContext(tx.Context, `SELECT cache_json,source_turn_id,publication_hash,version,state FROM answer_cache WHERE cache_id=? AND site_id=? AND data_epoch=?`, id, op.Actor.SiteID, op.DataEpoch).Scan(&raw, &record.SourceTurnID, &record.PublicationHash, &version, &state)
	if err == sql.ErrNoRows {
		return record, platform.NewError(platform.CodeNotFound, false, "缓存条目不存在")
	}
	if err != nil {
		return record, qaruntime.StorageError(err)
	}
	if qaruntime.DecodeResponse(tx.Runtime, "QaCacheEntry", raw, &record.Entry) != nil || record.Entry.CacheId != id || record.Entry.Version != version || string(record.Entry.State) != state {
		return CacheRecord{}, platform.NewError(platform.CodeIntegrityError, false, "缓存记录身份或状态损坏")
	}
	return record, nil
}
func SaveCache(tx *qaruntime.Transaction, op OperationContext, record CacheRecord) error {
	raw, err := platform.CanonicalJSON(record.Entry)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "缓存无法编码")
	}
	var expires any
	if record.Entry.ExpiresAt != nil {
		expires = record.Entry.ExpiresAt.UnixNano()
	}
	_, err = tx.Tx.ExecContext(tx.Context, `UPDATE answer_cache SET cache_json=?,version=?,state=?,scope_hash=?,question_hash=?,memory_hash=?,curated=?,expires_ns=?,publication_hash=? WHERE cache_id=? AND site_id=? AND data_epoch=?`, raw, record.Entry.Version, record.Entry.State, record.Entry.ScopeHash, record.Entry.QuestionHash, record.Entry.MemoryVersionHash, record.Entry.Curated, expires, record.PublicationHash, record.Entry.CacheId, op.Actor.SiteID, op.DataEpoch)
	if err != nil {
		return qaruntime.StorageError(err)
	}
	if string(record.Entry.State) != "active" {
		_, err = tx.Tx.ExecContext(tx.Context, `DELETE FROM cache_vectors WHERE cache_id=?`, record.Entry.CacheId)
	}
	return qaruntime.StorageError(err)
}
func InsertCache(tx *qaruntime.Transaction, op OperationContext, record CacheRecord) error {
	raw, err := platform.CanonicalJSON(record.Entry)
	if err != nil {
		return err
	}
	operation, _ := json.Marshal(op)
	var expires any
	if record.Entry.ExpiresAt != nil {
		expires = record.Entry.ExpiresAt.UnixNano()
	}
	_, err = tx.Tx.ExecContext(tx.Context, `INSERT INTO answer_cache(cache_id,site_id,data_epoch,source_turn_id,cache_json,version,state,scope_hash,question_hash,memory_hash,curated,expires_ns,publication_watermark,publication_hash,operation_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,0,?,?)`, record.Entry.CacheId, op.Actor.SiteID, op.DataEpoch, record.SourceTurnID, raw, record.Entry.Version, record.Entry.State, record.Entry.ScopeHash, record.Entry.QuestionHash, record.Entry.MemoryVersionHash, record.Entry.Curated, expires, record.PublicationHash, operation)
	return qaruntime.StorageError(err)
}
func EmitCacheChanged(tx *qaruntime.Transaction, op OperationContext, entry gen.CacheEntry) error {
	payload, _ := json.Marshal(struct {
		CacheID        string `json:"cacheId"`
		Version        int    `json:"version"`
		AnswerRevision int    `json:"answerRevision"`
		Curated        bool   `json:"curated"`
		State          string `json:"state"`
		DataEpoch      string `json:"dataEpoch"`
	}{entry.CacheId, entry.Version, entry.AnswerRevision, entry.Curated, string(entry.State), op.DataEpoch})
	return outbox.EnqueueEvent(tx.Tx, platform.EventEnvelope{EventID: "qa.cache." + entry.CacheId + ":" + platform.HashBytes(payload), EventType: "qa.cache.changed", EventVersion: 1, Producer: "qa", SubjectID: entry.CacheId, Revision: int64(entry.Version), OccurredAt: platform.Now(), DataEpoch: op.DataEpoch, Payload: payload})
}

type LookupExactCacheInput struct {
	QuestionHash string `json:"questionHash"`
	Language     string `json:"language"`
	ScopeHash    string `json:"scopeHash"`
	Complete     bool   `json:"complete"`
	FirstTurn    bool   `json:"firstTurn"`
}
type LookupExactCacheResult struct {
	Entry *gen.CacheEntry `json:"entry,omitempty"`
	Hit   bool            `json:"hit"`
}

// LookupExactCache 只用已完整问题键；精选优先，来源授权交统一有效性门禁。
func LookupExactCache(ctx context.Context, op OperationContext, input LookupExactCacheInput) (LookupExactCacheResult, error) {
	if !input.Complete {
		return LookupExactCacheResult{}, nil
	}
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return LookupExactCacheResult{}, err
	}
	defer tx.Close()
	policy, err := qaruntime.ReadPolicy(tx)
	if err != nil {
		return LookupExactCacheResult{}, err
	}
	if !policy.CacheEnabled {
		return LookupExactCacheResult{}, nil
	}
	rows, err := tx.Tx.QueryContext(tx.Context, `SELECT cache_id FROM answer_cache WHERE site_id=? AND data_epoch=? AND scope_hash=? AND question_hash=? AND state='active' AND (curated=1 OR expires_ns>?) ORDER BY curated DESC,cache_id`, op.Actor.SiteID, op.DataEpoch, input.ScopeHash, input.QuestionHash, time.Now().UnixNano())
	if err != nil {
		return LookupExactCacheResult{}, qaruntime.StorageError(err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return LookupExactCacheResult{}, qaruntime.StorageError(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return LookupExactCacheResult{}, qaruntime.StorageError(err)
	}
	records := make([]gen.CacheEntry, 0, len(ids))
	for _, id := range ids {
		record, err := ReadCache(tx, op, id)
		if err != nil {
			return LookupExactCacheResult{}, err
		}
		if record.Entry.QuestionComplete {
			records = append(records, record.Entry)
		}
	}
	tx.Close()
	for _, entry := range records {
		validated, err := ValidateCachedAnswer(ctx, op, ValidateCachedAnswerInput{Entry: entry, ContextHash: tx.Runtime.AnswerMemoryHash})
		if err != nil {
			return LookupExactCacheResult{}, err
		}
		if validated.Valid {
			return LookupExactCacheResult{Entry: validated.Entry, Hit: true}, nil
		}
	}
	return LookupExactCacheResult{}, nil
}
