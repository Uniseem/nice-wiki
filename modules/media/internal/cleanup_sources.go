package media

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

type ValidateCleanupSourcesInput struct {
	AssetId            Identifier    `json:"assetId"`
	ExpectedGeneration *gen.Revision `json:"expectedGeneration"`
}
type ValidateCleanupSourcesResult struct {
	Snapshot     gen.ReferenceSnapshot `json:"snapshot"`
	SafeToDelete bool                  `json:"safeToDelete"`
}

func ValidateCleanupSources(ctx context.Context, op OperationContext, input ValidateCleanupSourcesInput) (ValidateCleanupSourcesResult, error) {
	read, err := ReadAssetReferences(ctx, op, ReadAssetReferencesInput{AssetId: input.AssetId})
	if err != nil {
		return ValidateCleanupSourcesResult{}, err
	}
	if input.ExpectedGeneration != nil && *input.ExpectedGeneration != read.Snapshot.ReferenceGeneration {
		return ValidateCleanupSourcesResult{}, platform.NewError(platform.CodeVersionMismatch, false, "清理扫描引用水位已变化")
	}
	complete := len(read.Snapshot.SourceWatermarks) == 2
	for _, source := range read.Snapshot.SourceWatermarks {
		complete = complete && source.Complete
	}
	safe := complete && len(read.Snapshot.CurrentReferences) == 0
	if safe && isImageVariant(ctx, input.AssetId) {
		safe = false // 尺寸变体只被原图的元数据引用，不能当作未使用文件清理
	}
	return ValidateCleanupSourcesResult{Snapshot: read.Snapshot, SafeToDelete: safe}, nil
}

func isImageVariant(ctx context.Context, assetID Identifier) bool {
	store, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return false
	}
	var found int
	err := store.db.QueryRowContext(ctx, `SELECT 1 FROM assets WHERE state='ready' AND EXISTS (SELECT 1 FROM json_each(asset_json,'$.imageVariants') WHERE json_extract(value,'$.asset.assetId')=?) LIMIT 1`, assetID).Scan(&found)
	return err == nil && found == 1
}
