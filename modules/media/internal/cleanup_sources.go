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
	return ValidateCleanupSourcesResult{Snapshot: read.Snapshot, SafeToDelete: complete && len(read.Snapshot.CurrentReferences) == 0}, nil
}
