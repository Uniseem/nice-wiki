package memorycontext

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type ReadContextManifestInput struct {
	VersionHash string `json:"versionHash"`
}
type ReadContextManifestResult struct {
	Manifest ContextManifest `json:"manifest"`
}

func ReadContextManifest(ctx context.Context, op OperationContext, input ReadContextManifestInput) (ReadContextManifestResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return ReadContextManifestResult{}, err
	}
	var result ReadContextManifestResult
	err := memoryruntime.ReadSnapshot(ctx, func(_ *memoryruntime.Store, tx *sql.Tx) error {
		var raw []byte
		err := tx.QueryRowContext(ctx, `SELECT manifest_json FROM context_snapshots WHERE site_id=? AND manifest_hash=?`, op.Actor.SiteID, input.VersionHash).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			return platform.NewError(platform.CodeNotFound, false, "记忆注入清单不存在")
		}
		if err != nil {
			return memoryruntime.DBError(err)
		}
		if json.Unmarshal(raw, &result.Manifest) != nil || result.Manifest.VersionHash != input.VersionHash {
			return platform.NewError(platform.CodeIntegrityError, false, "记忆注入清单损坏")
		}
		return nil
	})
	return result, err
}
