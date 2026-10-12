package assets

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/doc/nodes"
	"nice-wiki/internal/store"
	"nice-wiki/internal/store/sqlc"
)

type ReferenceDocument struct {
	ID       string
	Source   string
	Document doc.Document
}

func (s *Store) refreshReferences(ctx context.Context, documents []ReferenceDocument) error {
	if s.source == nil {
		return errors.New("资源引用来源尚未接入，不能清理")
	}
	return store.Transaction(ctx, s.db, func(tx *sql.Tx) error {
		queries := sqlc.New(tx)
		if err := queries.ClearAssetReferences(ctx); err != nil {
			return err
		}
		for _, document := range documents {
			var scanErr error
			doc.Walk(document.Document.Content, func(node, parent doc.Node, depth int) bool {
				if scanErr != nil {
					return false
				}
				hashes := []string{}
				for _, key := range []string{"hash", "lightHash", "darkHash", "sceneHash", "posterHash"} {
					if value := nodes.String(node, key); value != "" {
						hashes = append(hashes, value)
					}
				}
				if variants, ok := node.Attrs["variants"].([]any); ok {
					for _, raw := range variants {
						if variant, ok := raw.(map[string]any); ok {
							if hash, ok := variant["hash"].(string); ok && hash != "" {
								hashes = append(hashes, hash)
							}
						}
					}
				}
				for _, hash := range hashes {
					if _, err := queries.GetAsset(ctx, hash); errors.Is(err, sql.ErrNoRows) {
						continue
					} else if err != nil {
						scanErr = err
						return false
					}
					if err := queries.AddAssetReference(ctx, sqlc.AddAssetReferenceParams{AssetHash: hash, DocumentID: document.ID, BlockID: nodes.String(node, "id"), Source: document.Source}); err != nil {
						scanErr = err
						return false
					}
				}
				return true
			})
			if scanErr != nil {
				return scanErr
			}
		}
		return nil
	})
}
