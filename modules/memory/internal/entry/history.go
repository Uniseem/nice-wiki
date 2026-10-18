package entry

import (
	"context"
	"database/sql"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type ListEntryRevisionsInput struct {
	EntryId       string `json:"entryId"`
	BeforeVersion *int   `json:"beforeVersion,omitempty"`
	Limit         int    `json:"limit"`
}
type ListEntryRevisionsResult = gen.ListEntryRevisionsResult

func ListEntryRevisions(ctx context.Context, op OperationContext, input ListEntryRevisionsInput) (ListEntryRevisionsResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return ListEntryRevisionsResult{}, err
	}
	result := ListEntryRevisionsResult{Entries: []gen.MemoryEntry{}}
	err := memoryruntime.ReadSnapshot(ctx, func(s *memoryruntime.Store, tx *sql.Tx) error {
		query := `SELECT version FROM entry_revisions WHERE site_id=? AND entry_id=?`
		args := []any{op.Actor.SiteID, input.EntryId}
		if input.BeforeVersion != nil {
			query += ` AND version<?`
			args = append(args, *input.BeforeVersion)
		}
		query += ` ORDER BY version DESC LIMIT ?`
		args = append(args, input.Limit+1)
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return memoryruntime.DBError(err)
		}
		versions := []int{}
		for rows.Next() {
			var v int
			if err = rows.Scan(&v); err != nil {
				rows.Close()
				return memoryruntime.DBError(err)
			}
			versions = append(versions, v)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return memoryruntime.DBError(err)
		}
		more := len(versions) > input.Limit
		if more {
			versions = versions[:input.Limit]
		}
		for _, v := range versions {
			e, err := s.ReadEntryObject(ctx, tx, op.Actor.SiteID, input.EntryId, &v)
			if err != nil {
				return err
			}
			result.Entries = append(result.Entries, e)
		}
		if more {
			v := versions[len(versions)-1]
			result.NextVersion = &v
		}
		return nil
	})
	return result, err
}
