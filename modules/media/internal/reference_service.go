package media

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
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
	locked, plans, err := s.referenceCleanupAssets(ctx, op, input.SiteId, assets, assets)
	if err != nil {
		return AcquireResourceReferencesResult{}, err
	}
	releaseAssets := s.lockReferenceAssets(locked)
	defer releaseAssets()
	return s.acquireReferencePin(ctx, op, canonical, bindingHash, assets, locked, plans)
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
	bindingHash, err := referenceBindingHash(op, input)
	if err != nil {
		return input, "", nil, err
	}
	assets := make([]gen.AssetVersionRef, 0, len(assetsByID))
	for _, asset := range assetsByID {
		assets = append(assets, asset)
	}
	slices.SortFunc(assets, func(a, b gen.AssetVersionRef) int { return cmp.Compare(a.AssetId, b.AssetId) })
	return input, bindingHash, assets, nil
}

func referenceBindingHash(op OperationContext, input AcquireResourceReferencesInput) (Hash, error) {
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
		return "", err
	}
	return platform.HashBytes(raw), nil
}

type ConfirmResourceReferencesInput = gen.ConfirmResourceReferencesInput
type ConfirmResourceReferencesResult = gen.ConfirmResourceReferencesResult
type ReleaseResourceReferencesInput = gen.ReleaseResourceReferencesInput
type ReleaseResourceReferencesResult = gen.ReleaseResourceReferencesResult

// ConfirmResourceReferences 先无锁核源COMMITTED，再提交本域完整引用集合。
func ConfirmResourceReferences(ctx context.Context, op OperationContext, input ConfirmResourceReferencesInput) (ConfirmResourceReferencesResult, error) {
	if err := validateReferenceSource(op); err != nil {
		return ConfirmResourceReferencesResult{}, err
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "引用存储未装配")
	}
	if op.DataEpoch != s.openedEpoch {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeVersionMismatch, false, "引用确认不属于活动代次")
	}
	var sourceOperationID string
	err := s.db.QueryRowContext(ctx, `SELECT source_operation_id FROM reference_sources WHERE source_service=? AND data_epoch=? AND pin_id=?`, op.CallerService, op.DataEpoch, input.PinId).Scan(&sourceOperationID)
	if errors.Is(err, sql.ErrNoRows) {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeNotFound, false, "来源pin不存在")
	}
	if err != nil {
		return ConfirmResourceReferencesResult{}, mapMediaDBError(err, "读取pin来源失败")
	}
	record, found, err := s.readReferenceRecord(ctx, s.db, op, sourceOperationID)
	if err != nil {
		return ConfirmResourceReferencesResult{}, err
	}
	if !found || record.PinID != input.PinId || record.Material.SourceRevisionId != input.SourceRevisionId || record.Material.SourceVersion != input.SourceVersion {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "确认pin与原来源修订或水位不符")
	}
	if record.State == "confirmed" {
		if err := readReferenceResolution(ctx, s.db, op, record, "confirm"); err != nil {
			return ConfirmResourceReferencesResult{}, err
		}
		return ConfirmResourceReferencesResult{Confirmed: true}, nil
	}
	decision, err := s.readReferenceDecision(ctx, op, record.Material)
	if err != nil {
		return ConfirmResourceReferencesResult{}, err
	}
	if string(decision.State) != "committed" {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "来源尚无可确认COMMITTED决定，保留pin")
	}
	if record.State == "sealed" {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "已封存来源与COMMITTED决定矛盾")
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return ConfirmResourceReferencesResult{}, err
	}
	defer cancel()
	return s.confirmReferencePin(work, op, record)
}

// ReleaseResourceReferences 仅凭完整、明确的源终止决定封存；无pin也要持久化。
func ReleaseResourceReferences(ctx context.Context, op OperationContext, input ReleaseResourceReferencesInput) (ReleaseResourceReferencesResult, error) {
	if err := validateReferenceSource(op); err != nil {
		return ReleaseResourceReferencesResult{}, err
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "引用存储未装配")
	}
	if op.DataEpoch != s.openedEpoch {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeVersionMismatch, false, "引用释放不属于活动代次")
	}
	material := AcquireResourceReferencesInput{SiteId: input.SiteId, SourceObjectId: input.SourceObjectId, SourceRevisionId: input.SourceRevisionId, SourceVersion: input.SourceVersion, SourceOperationId: input.SourceOperationId, ReferenceDigest: input.ReferenceDigest}
	bindingHash, err := referenceBindingHash(op, material)
	if err != nil {
		return ReleaseResourceReferencesResult{}, err
	}
	record, found, err := s.readReferenceRecord(ctx, s.db, op, input.SourceOperationId)
	if err != nil {
		return ReleaseResourceReferencesResult{}, err
	}
	if found && record.BindingHash != bindingHash {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "释放来源已绑定其他完整材料")
	}
	if input.PinId != nil && (!found || record.PinID != *input.PinId) {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "已知pin不属于完整来源")
	}
	if found && record.State == "confirmed" {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "已确认引用不能释放")
	}
	if found && record.State == "sealed" {
		if record.Reason != string(input.Reason) {
			return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "来源已绑定其他终止原因")
		}
		if err := readReferenceResolution(ctx, s.db, op, record, "release"); err != nil {
			return ReleaseResourceReferencesResult{}, err
		}
		return ReleaseResourceReferencesResult{Released: true}, nil
	}
	decision, err := s.readReferenceDecision(ctx, op, material)
	if err != nil {
		return ReleaseResourceReferencesResult{}, err
	}
	if string(decision.State) == "committed" {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "已提交来源不得释放")
	}
	expectedState := "aborted"
	if input.Reason == gen.SourceRejected {
		expectedState = "rejected"
	}
	if string(decision.State) != expectedState {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "来源尚无匹配的明确终止决定，保留pin")
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return ReleaseResourceReferencesResult{}, err
	}
	defer cancel()
	return s.releaseReferencePin(work, op, input, material, bindingHash)
}
