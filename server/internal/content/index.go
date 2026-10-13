package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/store"
	"nice-wiki/internal/store/sqlc"
	"nice-wiki/internal/textutil"
	"path/filepath"
)

func (r *Repo) rebuildLocked(ctx context.Context) error {
	err := store.Transaction(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM blocks"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM documents"); err != nil {
			return err
		}
		queries := sqlc.New(tx)
		seen := map[string]bool{}
		for id, value := range r.files {
			path, err := PathFor(id, r.files)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(r.layout.Root, r.paths[id])
			if err != nil {
				return err
			}
			hash, err := textutil.HashJSON(value.Document)
			if err != nil {
				return err
			}
			published := int64(0)
			if value.Meta.Published {
				published = 1
			}
			if err := queries.UpsertDocument(ctx, sqlc.UpsertDocumentParams{ID: id, ParentID: value.Meta.ParentID, Title: value.Meta.Title, Slug: value.Meta.Slug, SortKey: value.Meta.SortKey, FilePath: filepath.ToSlash(relative), Path: path, Published: published, Revision: value.Meta.Revision, ContentHash: hash, UpdatedAt: value.Meta.UpdatedAt}); err != nil {
				return err
			}
			blocks, err := doc.Blocks(value.Document)
			if err != nil {
				return err
			}
			for _, block := range blocks {
				if seen[block.ID] {
					return errors.New("不同文档出现相同块 ID")
				}
				seen[block.ID] = true
				headings, err := json.Marshal(block.HeadingPath)
				if err != nil {
					return err
				}
				if err := queries.InsertBlock(ctx, sqlc.InsertBlockParams{ID: block.ID, DocumentID: id, ParentID: block.ParentID, Kind: block.Kind, Position: int64(block.Position), HeadingPath: string(headings), Text: block.Text, ContentHash: block.ContentHash}); err != nil {
					return err
				}
			}
		}
		// 仅保留仍与已发布正文一致的付费派生数据。
		invalid := "SELECT u.id FROM retrieval_units u WHERE NOT EXISTS (SELECT 1 FROM documents d JOIN blocks b ON b.document_id=d.id WHERE d.id=u.document_id AND d.published=1 AND b.id=u.id AND b.text=u.text)"
		for _, statement := range []string{
			"DELETE FROM vectors WHERE unit_id IN (" + invalid + ")",
			"DELETE FROM keyword_index WHERE unit_id IN (" + invalid + ")",
			"DELETE FROM retrieval_unit_blocks WHERE unit_id IN (" + invalid + ")",
			"DELETE FROM retrieval_units WHERE id IN (" + invalid + ")",
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil && r.onChange != nil {
		r.onChange()
	}
	return err
}
