package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

type AttachImageVariantsInput struct {
	AssetId          Identifier
	Original         gen.AssetVersionRef
	Variants         []gen.ImageVariant
	ExpectedRevision gen.Revision
}

type AttachImageVariantsResult struct {
	Asset gen.Asset `json:"asset"`
}

// AttachImageVariants 把浏览器生成并已上传的多宽 WebP 关联到原图；服务端不做图像处理。
func AttachImageVariants(ctx context.Context, op OperationContext, input AttachImageVariantsInput) (AttachImageVariantsResult, error) {
	if op.Actor.Kind != "admin" || op.Actor.SessionID == "" || (!hasGrant(op.Actor.Grants, "media.upload") && !hasGrant(op.Actor.Grants, "superadmin")) {
		return AttachImageVariantsResult{}, platform.NewError(platform.CodeForbidden, false, "没有关联图片尺寸变体的权限")
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return AttachImageVariantsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储未装配")
	}
	if input.Original.AssetId != input.AssetId || len(input.Variants) > 3 {
		return AttachImageVariantsResult{}, platform.NewError(platform.CodeValidation, false, "变体请求与资源不一致")
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return AttachImageVariantsResult{}, err
	}
	defer cancel()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return AttachImageVariantsResult{}, mapMediaDBError(err, "开始变体事务失败")
	}
	defer tx.Rollback()
	asset, err := s.loadReadyAsset(work, tx, op.Actor.SiteID, input.AssetId)
	if err != nil {
		return AttachImageVariantsResult{}, err
	}
	if asset.Kind != gen.AssetKind("image") || asset.Animated || asset.Width == nil {
		return AttachImageVariantsResult{}, platform.NewError(platform.CodeValidation, false, "只有静态图片可以有尺寸变体")
	}
	if asset.Ref != input.Original || asset.Revision != input.ExpectedRevision {
		return AttachImageVariantsResult{}, platform.NewError(platform.CodeVersionMismatch, false, "资源版本已变化")
	}
	for _, variant := range input.Variants {
		child, err := s.loadReadyAsset(work, tx, op.Actor.SiteID, variant.Asset.AssetId)
		if err != nil {
			return AttachImageVariantsResult{}, err
		}
		if child.Ref != variant.Asset || child.Kind != gen.AssetKind("image") || variant.Width >= *asset.Width {
			return AttachImageVariantsResult{}, platform.NewError(platform.CodeValidation, false, "变体必须是更小的已就绪图片")
		}
	}
	variants := slices.Clone(input.Variants)
	slices.SortFunc(variants, func(a, b gen.ImageVariant) int { return a.Width - b.Width })
	asset.ImageVariants = variants
	asset.Revision++
	raw, err := platform.CanonicalJSON(asset)
	if err != nil || s.validateAsset(json.RawMessage(raw)) != nil {
		return AttachImageVariantsResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源元数据无法编码")
	}
	if _, err := tx.ExecContext(work, `UPDATE assets SET asset_json=?,updated_at=? WHERE asset_id=? AND site_id=?`, string(raw), time.Now().UTC().Format(time.RFC3339Nano), asset.Ref.AssetId, op.Actor.SiteID); err != nil {
		return AttachImageVariantsResult{}, mapMediaDBError(err, "写入变体失败")
	}
	if err := tx.Commit(); err != nil {
		return AttachImageVariantsResult{}, mapMediaDBError(err, "提交变体失败")
	}
	return AttachImageVariantsResult{Asset: asset}, nil
}

func (s *localBlobStore) loadReadyAsset(ctx context.Context, tx *sql.Tx, siteID, assetID Identifier) (gen.Asset, error) {
	var raw string
	err := tx.QueryRowContext(ctx, `SELECT asset_json FROM assets WHERE asset_id=? AND site_id=? AND state='ready'`, assetID, siteID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.Asset{}, platform.NewError(platform.CodeNotFound, false, "资源不存在")
	}
	if err != nil {
		return gen.Asset{}, mapMediaDBError(err, "读取资源失败")
	}
	var asset gen.Asset
	if json.Unmarshal([]byte(raw), &asset) != nil || asset.Ref.AssetId != assetID {
		return gen.Asset{}, platform.NewError(platform.CodeIntegrityError, false, "资源元数据不可读")
	}
	return asset, nil
}
