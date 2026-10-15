package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

type ReadAssetInput struct {
	AssetId Identifier    `json:"assetId"`
	Version *gen.Revision `json:"version"`
}
type ReadAssetResult = gen.ReadAssetResult

// ReadAsset 只读当前真实ready元数据；指定版本不存在时不构造历史。
func ReadAsset(ctx context.Context, op OperationContext, input ReadAssetInput) (ReadAssetResult, error) {
	service := op.Actor.Kind == "service" && slices.Contains([]string{"content", "site-config", "public-site", "localization", "knowledge"}, op.CallerService)
	if !service && (op.Actor.Kind != "admin" || op.Actor.SessionID == "" || (!hasGrant(op.Actor.Grants, "media.read") && !hasGrant(op.Actor.Grants, "media.upload") && !hasGrant(op.Actor.Grants, "media.manage") && !hasGrant(op.Actor.Grants, "superadmin"))) {
		return ReadAssetResult{}, platform.NewError(platform.CodeForbidden, false, "没有读取资源元数据的权限")
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return ReadAssetResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储未装配")
	}
	if op.DataEpoch != s.openedEpoch {
		return ReadAssetResult{}, platform.NewError(platform.CodeVersionMismatch, false, "媒体读取不属于已打开数据代次")
	}
	var raw sql.NullString
	var id, site, sha, kind, state string
	var size int
	err := s.db.QueryRowContext(ctx, `SELECT asset_id,site_id,sha256,byte_size,media_type,state,CASE WHEN length(CAST(asset_json AS BLOB))<=16777216 THEN asset_json ELSE NULL END FROM assets WHERE asset_id=?`, input.AssetId).Scan(&id, &site, &sha, &size, &kind, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ReadAssetResult{}, platform.NewError(platform.CodeNotFound, false, "资源不存在")
	}
	if err != nil {
		return ReadAssetResult{}, mapMediaDBError(err, "读取资源元数据失败")
	}
	if !service && site != op.Actor.SiteID {
		return ReadAssetResult{}, platform.NewError(platform.CodeForbidden, false, "资源不属于当前站点")
	}
	if !raw.Valid || s.validateAsset == nil || s.validateAsset(json.RawMessage(raw.String)) != nil {
		return ReadAssetResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源持久元数据不符合唯一schema")
	}
	var asset gen.Asset
	if json.Unmarshal([]byte(raw.String), &asset) != nil || asset.Ref.AssetId != id || asset.SiteId != site || asset.Ref.Sha256 != sha || asset.Ref.ByteSize != size || asset.Ref.MediaType != kind || string(asset.State) != state {
		return ReadAssetResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源元数据与索引不符")
	}
	if asset.State != gen.AssetStateReady || (input.Version != nil && *input.Version != asset.Ref.Version) {
		return ReadAssetResult{}, platform.NewError(platform.CodeNotFound, false, "指定ready资源版本不存在")
	}
	return ReadAssetResult{Asset: asset}, nil
}
