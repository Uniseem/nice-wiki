package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
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

// AssetPageRequest 由严格HTTP查询转换，省略limit时归一为20。
type AssetPageRequest struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}
type ListAssetsInput struct {
	Page AssetPageRequest `json:"page"`
	Kind *gen.AssetKind   `json:"kind"`
}
type ListAssetsResult = gen.ListAssetsResult

type assetListingSources struct {
	Content gen.Revision `json:"content"`
	Config  gen.Revision `json:"siteConfig"`
}
type assetListingCursor struct {
	SchemaVersion  int                  `json:"schemaVersion"`
	LocalRevision  int64                `json:"localRevision"`
	AfterKind      string               `json:"afterKind"`
	AfterCreatedAt string               `json:"afterCreatedAt"`
	AfterAssetId   Identifier           `json:"afterAssetId"`
	Sources        *assetListingSources `json:"sources"`
}
type listedAsset struct {
	Asset      gen.Asset
	Fence      int
	Generation int
	CreatedAt  string
}
type assetListingPage struct {
	Revision int64
	Assets   []listedAsset
	HasMore  bool
}

// InstallAssetListingKey 只由已验证密钥的可信启动链装配，不把密钥引用持久化。
func InstallAssetListingKey(key platform.SigningKey) error {
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "资源列表私有存储未装配")
	}
	s.listingKey = &key
	return nil
}

func assetListingStore(op OperationContext) (*localBlobStore, error) {
	s, err := cleanupStore(op, true)
	if err != nil {
		return nil, err
	}
	if s.listingKey == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "资源列表签名密钥未装配")
	}
	return s, nil
}

func (s *localBlobStore) readAssetListingCursor(op OperationContext, input ListAssetsInput, unused bool) (assetListingCursor, string, error) {
	material, err := platform.CanonicalJSON(struct {
		Caller, ActorKind, Actor, Site, Epoch, Session string
		SessionRevision, PermissionRevision            int64
		Limit                                          int
		Kind                                           *gen.AssetKind
		Unused                                         bool
	}{op.CallerService, op.Actor.Kind, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.Actor.SessionID, op.Actor.SessionRevision, op.Actor.PermissionRevision, input.Page.Limit, input.Kind, unused})
	if err != nil {
		return assetListingCursor{}, "", err
	}
	scope := "media.assets.v1:" + platform.HashBytes(material)
	position := assetListingCursor{SchemaVersion: 1}
	if input.Page.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(*s.listingKey, scope, input.Page.Cursor)
		if err != nil {
			return position, "", err
		}
		if json.Unmarshal(raw, &position) != nil {
			return position, "", platform.NewError(platform.CodeValidation, false, "资源分页位置不可读")
		}
		canonical, err := platform.CanonicalJSON(position)
		_, timeErr := time.Parse(time.RFC3339Nano, position.AfterCreatedAt)
		if err != nil || string(canonical) != string(raw) || position.SchemaVersion != 1 || position.LocalRevision < 1 || !platform.ValidIdentifier(position.AfterAssetId) || position.AfterKind == "" || timeErr != nil || (unused && position.Sources == nil) || (!unused && position.Sources != nil) || (position.Sources != nil && (position.Sources.Content < 0 || position.Sources.Config < 0)) {
			return position, "", platform.NewError(platform.CodeValidation, false, "资源分页位置或版本不合法")
		}
	}
	return position, scope, nil
}

func readAssetListingRevision(ctx context.Context, q sqlExecer) (int64, error) {
	var revision int64
	err := q.QueryRowContext(ctx, `SELECT revision FROM asset_listing_state WHERE singleton=1`).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && revision < 1) {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "资源列表权威水位缺失或损坏")
	}
	if err != nil {
		return 0, mapMediaDBError(err, "读取资源列表水位失败")
	}
	return revision, nil
}

// readAssetListingPage 在单一短只读快照读水位和有界候选，结束事务后才允许回源/签名。
func (s *localBlobStore) readAssetListingPage(ctx context.Context, op OperationContext, input ListAssetsInput, position assetListingCursor, unused bool) (assetListingPage, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return assetListingPage{}, mapMediaDBError(err, "开始资源列表快照失败")
	}
	defer tx.Rollback()
	revision, err := readAssetListingRevision(ctx, tx)
	if err != nil {
		return assetListingPage{}, err
	}
	if input.Page.Cursor != "" && position.LocalRevision != revision {
		return assetListingPage{}, platform.NewError(platform.CodeVersionMismatch, true, "资源列表水位已变化，请从第一页重新读取")
	}
	kind := ""
	if input.Kind != nil {
		kind = string(*input.Kind)
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.asset_id,a.site_id,a.sha256,a.byte_size,a.media_type,a.state,CASE WHEN length(CAST(a.asset_json AS BLOB))<=16777216 THEN a.asset_json ELSE NULL END,r.deletion_fence,r.reference_generation,json_extract(a.asset_json,'$.createdAt') FROM assets a LEFT JOIN reference_objects r ON r.asset_id=a.asset_id WHERE a.site_id=? AND (?='' OR json_extract(a.asset_json,'$.kind')=?) AND (?=0 OR a.state='ready') AND (?='' OR (json_extract(a.asset_json,'$.kind'),json_extract(a.asset_json,'$.createdAt'),a.asset_id)>(?,?,?)) ORDER BY json_extract(a.asset_json,'$.kind'),json_extract(a.asset_json,'$.createdAt'),a.asset_id LIMIT ?`, op.Actor.SiteID, kind, kind, unused, position.AfterAssetId, position.AfterKind, position.AfterCreatedAt, position.AfterAssetId, input.Page.Limit+1)
	if err != nil {
		return assetListingPage{}, mapMediaDBError(err, "读取资源列表候选失败")
	}
	page := assetListingPage{Revision: revision, Assets: make([]listedAsset, 0, input.Page.Limit+1)}
	for rows.Next() {
		var id, site, sha, mediaType, state string
		var size int
		var raw sql.NullString
		var fence, generation sql.NullInt64
		var listed listedAsset
		if err := rows.Scan(&id, &site, &sha, &size, &mediaType, &state, &raw, &fence, &generation, &listed.CreatedAt); err != nil {
			rows.Close()
			return page, mapMediaDBError(err, "读取资源列表材料失败")
		}
		asset := &listed.Asset
		if !raw.Valid || s.validateAsset(json.RawMessage(raw.String)) != nil || json.Unmarshal([]byte(raw.String), asset) != nil || asset.Ref.AssetId != id || asset.SiteId != site || asset.Ref.Sha256 != sha || asset.Ref.ByteSize != size || asset.Ref.MediaType != mediaType || string(asset.State) != state || !fence.Valid || fence.Int64 < 0 || !generation.Valid || generation.Int64 < 0 {
			rows.Close()
			return page, platform.NewError(platform.CodeIntegrityError, false, "资源列表元数据、索引或对象水位不一致")
		}
		listed.Fence, listed.Generation = int(fence.Int64), int(generation.Int64)
		page.Assets = append(page.Assets, listed)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, mapMediaDBError(err, "读取资源列表候选集合失败")
	}
	if err := tx.Rollback(); err != nil {
		return page, mapMediaDBError(err, "结束资源列表快照失败")
	}
	page.HasMore = len(page.Assets) > input.Page.Limit
	if page.HasMore {
		page.Assets = page.Assets[:input.Page.Limit]
	}
	return page, nil
}

func (s *localBlobStore) assetListingPageInfo(scope string, position assetListingCursor, page assetListingPage) (gen.PageInfo, error) {
	info := gen.PageInfo{HasMore: page.HasMore}
	if page.HasMore {
		last := page.Assets[len(page.Assets)-1]
		position.LocalRevision = page.Revision
		position.AfterKind = string(last.Asset.Kind)
		position.AfterCreatedAt = last.CreatedAt
		position.AfterAssetId = last.Asset.Ref.AssetId
		raw, err := platform.CanonicalJSON(position)
		if err != nil {
			return info, err
		}
		cursor, err := serviceauth.SignCursor(*s.listingKey, scope, raw)
		if err != nil {
			return info, err
		}
		info.NextCursor = &cursor
	}
	return info, nil
}

// ListAssets 只读本站真实安全元数据；引用详情通过独立ReadAssetReferences读取。
func ListAssets(ctx context.Context, op OperationContext, input ListAssetsInput) (ListAssetsResult, error) {
	s, err := assetListingStore(op)
	if err != nil {
		return ListAssetsResult{}, err
	}
	if input.Page.Limit == 0 {
		input.Page.Limit = 20
	}
	position, scope, err := s.readAssetListingCursor(op, input, false)
	if err != nil {
		return ListAssetsResult{}, err
	}
	page, err := s.readAssetListingPage(ctx, op, input, position, false)
	if err != nil {
		return ListAssetsResult{}, err
	}
	result := ListAssetsResult{Assets: make([]gen.Asset, 0, len(page.Assets))}
	for _, listed := range page.Assets {
		result.Assets = append(result.Assets, listed.Asset)
	}
	result.PageInfo, err = s.assetListingPageInfo(scope, position, page)
	if err != nil {
		return ListAssetsResult{}, err
	}
	return result, nil
}
