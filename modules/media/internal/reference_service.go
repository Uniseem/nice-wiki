package media

import (
	"cmp"
	"context"
	"slices"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

type AcquireResourceReferencesInput = gen.AcquireResourceReferencesInput
type AcquireResourceReferencesResult = gen.AcquireResourceReferencesResult

// AcquireResourceReferences 将完整源材料与原 pin 同事务绑定；空集合也保护来源。
func AcquireResourceReferences(ctx context.Context, op OperationContext, input AcquireResourceReferencesInput) (AcquireResourceReferencesResult, error) {
	if err := validateReferenceSource(op); err != nil {
		return AcquireResourceReferencesResult{}, err
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return AcquireResourceReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体引用存储未装配")
	}
	if op.DataEpoch != s.openedEpoch {
		return AcquireResourceReferencesResult{}, platform.NewError(platform.CodeVersionMismatch, false, "引用预留不属于已打开数据代次")
	}
	canonical, bindingHash, assets, err := canonicalReferenceInput(op, input)
	if err != nil {
		return AcquireResourceReferencesResult{}, err
	}
	releaseSource := s.lockReferenceKey("source\x00" + op.CallerService + "\x00" + op.DataEpoch + "\x00" + input.SourceOperationId)
	defer releaseSource()
	result, found, err := s.readReferenceSource(ctx, s.db, op, canonical, bindingHash)
	if err != nil || found {
		return result, err
	}
	releaseAssets := s.lockReferenceAssets(assets)
	defer releaseAssets()
	return s.acquireReferencePin(ctx, op, canonical, bindingHash, assets)
}

func validateReferenceSource(op OperationContext) error {
	if (op.CallerService != "content" && op.CallerService != "site-config") || op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService {
		return platform.NewError(platform.CodeForbidden, false, "引用协议仅接受对应来源纯服务")
	}
	return nil
}

func canonicalReferenceInput(op OperationContext, input AcquireResourceReferencesInput) (AcquireResourceReferencesInput, Hash, []gen.AssetVersionRef, error) {
	input.References = append(make([]gen.AssetReference, 0, len(input.References)), input.References...)
	slices.SortFunc(input.References, func(a, b gen.AssetReference) int {
		if c := cmp.Compare(a.ReferenceId, b.ReferenceId); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Asset.AssetId, b.Asset.AssetId); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Asset.Version, b.Asset.Version); c != 0 {
			return c
		}
		return cmp.Compare(a.Asset.Sha256, b.Asset.Sha256)
	})
	assetsByID := make(map[string]gen.AssetVersionRef)
	for i, ref := range input.References {
		if string(ref.SourceService) != op.CallerService || ref.SourceObjectId != input.SourceObjectId || ref.SourceRevisionId != input.SourceRevisionId || ref.SourceVersion != input.SourceVersion || ref.Kind != gen.PendingPin || ref.Confirmed || ref.ExpiresAt != nil {
			return input, "", nil, platform.NewError(platform.CodeIntegrityError, false, "引用项与完整来源或预提交状态不符")
		}
		if i > 0 {
			previous := input.References[i-1]
			if previous.ReferenceId == ref.ReferenceId && previous.Asset.AssetId == ref.Asset.AssetId && previous.Asset.Version == ref.Asset.Version {
				return input, "", nil, platform.NewError(platform.CodeIntegrityError, false, "同一来源引用重复或材料冲突")
			}
		}
		if previous, found := assetsByID[ref.Asset.AssetId]; found && previous != ref.Asset {
			return input, "", nil, platform.NewError(platform.CodeIntegrityError, false, "同一资源的不可变引用材料冲突")
		}
		assetsByID[ref.Asset.AssetId] = ref.Asset
	}
	referenceBytes, err := platform.CanonicalJSON(input.References)
	if err != nil {
		return input, "", nil, err
	}
	if platform.HashBytes(referenceBytes) != input.ReferenceDigest {
		return input, "", nil, platform.NewError(platform.CodeIntegrityError, false, "完整引用集合摘要不符")
	}
	raw, err := platform.CanonicalJSON(struct {
		SourceService string       `json:"sourceService"`
		DataEpoch     Identifier   `json:"dataEpoch"`
		SiteID        Identifier   `json:"siteId"`
		ObjectID      Identifier   `json:"sourceObjectId"`
		RevisionID    Identifier   `json:"sourceRevisionId"`
		OperationID   Identifier   `json:"sourceOperationId"`
		Version       gen.Revision `json:"sourceVersion"`
		Digest        Hash         `json:"referenceDigest"`
	}{op.CallerService, op.DataEpoch, input.SiteId, input.SourceObjectId, input.SourceRevisionId, input.SourceOperationId, input.SourceVersion, input.ReferenceDigest})
	if err != nil {
		return input, "", nil, err
	}
	assets := make([]gen.AssetVersionRef, 0, len(assetsByID))
	for _, asset := range assetsByID {
		assets = append(assets, asset)
	}
	slices.SortFunc(assets, func(a, b gen.AssetVersionRef) int { return cmp.Compare(a.AssetId, b.AssetId) })
	return input, platform.HashBytes(raw), assets, nil
}
