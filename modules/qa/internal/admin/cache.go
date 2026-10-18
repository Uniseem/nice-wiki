package admin

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/qa/internal/cache"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type ListCacheEntriesInput struct {
	State  string `json:"state,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}
type ListCacheEntriesResult = gen.ListCacheEntriesResult

func ListCacheEntries(ctx context.Context, op OperationContext, input ListCacheEntriesInput) (ListCacheEntriesResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.manage"); err != nil {
		return ListCacheEntriesResult{}, err
	}
	if input.Limit == 0 {
		input.Limit = 20
	}
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return ListCacheEntriesResult{}, err
	}
	defer tx.Close()
	binding, _ := platform.CanonicalJSON(struct {
		Actor        any
		Epoch, State string
		Limit        int
	}{qaruntime.CursorActor(op), op.DataEpoch, input.State, input.Limit})
	scope := "qa.cache.v1:" + platform.HashBytes(binding)
	after := ""
	if input.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(tx.Runtime.SigningKey, scope, input.Cursor)
		if err != nil {
			return ListCacheEntriesResult{}, err
		}
		if json.Unmarshal(raw, &after) != nil || !platform.ValidIdentifier(after) {
			return ListCacheEntriesResult{}, platform.NewError(platform.CodeValidation, false, "缓存分页游标不合法")
		}
	}
	rows, err := tx.Tx.QueryContext(tx.Context, `SELECT cache_id FROM answer_cache WHERE site_id=? AND data_epoch=? AND cache_id>? AND (?='' OR state=?) ORDER BY cache_id LIMIT ?`, op.Actor.SiteID, op.DataEpoch, after, input.State, input.State, input.Limit+1)
	if err != nil {
		return ListCacheEntriesResult{}, qaruntime.StorageError(err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return ListCacheEntriesResult{}, qaruntime.StorageError(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ListCacheEntriesResult{}, qaruntime.StorageError(err)
	}
	result := ListCacheEntriesResult{Entries: make([]gen.CacheEntry, 0)}
	for _, id := range ids[:min(len(ids), input.Limit)] {
		record, err := cache.ReadCache(tx, op, id)
		if err != nil {
			return ListCacheEntriesResult{}, err
		}
		result.Entries = append(result.Entries, record.Entry)
	}
	if len(ids) > input.Limit {
		raw, _ := json.Marshal(ids[input.Limit-1])
		cursor, err := serviceauth.SignCursor(tx.Runtime.SigningKey, scope, raw)
		if err != nil {
			return ListCacheEntriesResult{}, err
		}
		result.NextCursor = &cursor
	}
	return result, nil
}
