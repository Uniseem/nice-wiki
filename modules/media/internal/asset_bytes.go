package media

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

// openAssetBytes 按下载规则打开不可变原文件：
// 管理员（media.read / superadmin）可读本站任一就绪资源；其余调用方必须带正确的 sha，且资源已被正文或站点配置引用。
func openAssetBytes(r *http.Request, op OperationContext, assetID Identifier) (*os.File, gen.AssetVersionRef, string, error) {
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return nil, gen.AssetVersionRef{}, "", platform.NewError(platform.CodeDependencyUnavailable, true, "媒体存储未装配")
	}
	if op.DataEpoch != s.openedEpoch {
		return nil, gen.AssetVersionRef{}, "", platform.NewError(platform.CodeVersionMismatch, false, "资源不属于已打开数据代次")
	}
	var site, sha, mediaType, state, name string
	var size, version int
	err := s.db.QueryRowContext(r.Context(), `SELECT site_id,sha256,byte_size,media_type,state,json_extract(asset_json,'$.originalName'),json_extract(asset_json,'$.ref.version') FROM assets WHERE asset_id=?`, assetID).Scan(&site, &sha, &size, &mediaType, &state, &name, &version)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (site != op.Actor.SiteID || state != "ready")) {
		return nil, gen.AssetVersionRef{}, "", platform.NewError(platform.CodeNotFound, false, "资源不存在")
	}
	if err != nil {
		return nil, gen.AssetVersionRef{}, "", mapMediaDBError(err, "读取资源失败")
	}
	admin := op.Actor.Kind == "admin" && op.Actor.SessionID != "" && (hasGrant(op.Actor.Grants, "media.read") || hasGrant(op.Actor.Grants, "superadmin"))
	// 知识库与模型网关以服务身份读取已发布资源：按版本核对，不需要公开地址里的 sha。
	service := op.Actor.Kind == "service" && (op.CallerService == "knowledge" || op.CallerService == "model-gateway")
	if !admin {
		if service {
			if wanted := r.URL.Query().Get("version"); wanted != "" && wanted != strconv.Itoa(version) {
				return nil, gen.AssetVersionRef{}, "", platform.NewError(platform.CodeNotFound, false, "资源不存在")
			}
		} else if r.URL.Query().Get("sha") != sha {
			return nil, gen.AssetVersionRef{}, "", platform.NewError(platform.CodeNotFound, false, "资源不存在")
		}
		var referenced int
		if err = s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reference_pin_assets p JOIN reference_sources rs ON rs.pin_id=p.pin_id WHERE p.asset_id=? AND rs.state='confirmed'`, assetID).Scan(&referenced); err != nil {
			return nil, gen.AssetVersionRef{}, "", mapMediaDBError(err, "核对资源引用失败")
		}
		if referenced == 0 {
			return nil, gen.AssetVersionRef{}, "", platform.NewError(platform.CodeNotFound, false, "资源不存在")
		}
	}
	file, err := os.Open(s.blobPath(site, sha))
	if err != nil {
		return nil, gen.AssetVersionRef{}, "", platform.NewError(platform.CodeIntegrityError, false, "资源文件缺失")
	}
	return file, gen.AssetVersionRef{AssetId: assetID, Version: version, Sha256: sha, MediaType: mediaType, ByteSize: size}, name, nil
}

// ServeAssetBytes 输出不可变资源字节，支持单个 Range；head 为 true 时只写响应头。
func ServeAssetBytes(w http.ResponseWriter, r *http.Request, op OperationContext, assetID Identifier, head bool) error {
	file, ref, _, err := openAssetBytes(r, op, assetID)
	if err != nil {
		return err
	}
	defer file.Close()
	etag := `"` + ref.Sha256 + `"`
	byteRange := ByteRange{Start: 0, End: int64(ref.ByteSize) - 1, Total: int64(ref.ByteSize)}
	if header := r.Header.Get("Range"); header != "" {
		var ifRange *string
		if value := r.Header.Get("If-Range"); value != "" {
			ifRange = &value
		}
		parsed, parseErr := ParseByteRange(ParseByteRangeInput{Header: &header, Total: int64(ref.ByteSize), IfRange: ifRange, Etag: etag})
		if parseErr != nil {
			w.Header().Set("Content-Range", "bytes */"+strconv.Itoa(ref.ByteSize))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return nil
		}
		byteRange = parsed.Range
	}
	headers := w.Header()
	headers.Set("Content-Type", ref.MediaType)
	headers.Set("ETag", etag)
	headers.Set("Accept-Ranges", "bytes")
	headers.Set("X-Content-Type-Options", "nosniff")
	headers.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	if r.URL.Query().Get("sha") != "" {
		headers.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		headers.Set("Cache-Control", "private, no-store")
	}
	if r.Header.Get("If-None-Match") == etag && !byteRange.Partial {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	length := byteRange.End - byteRange.Start + 1
	headers.Set("Content-Length", strconv.FormatInt(length, 10))
	if byteRange.Partial {
		headers.Set("Content-Range", "bytes "+strconv.FormatInt(byteRange.Start, 10)+"-"+strconv.FormatInt(byteRange.End, 10)+"/"+strconv.FormatInt(byteRange.Total, 10))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	if head {
		return nil
	}
	_, _ = io.Copy(w, io.NewSectionReader(file, byteRange.Start, length))
	return nil
}

type BuildDeliveryManifestInput struct {
	Assets             []gen.AssetVersionRef     `json:"assets"`
	PublishedReference *gen.PublishedRevisionRef `json:"publishedReference"`
	PreviewId          *Identifier               `json:"previewId"`
	PreviewToken       *string                   `json:"previewToken"`
}
type BuildDeliveryManifestResult = gen.BuildDeliveryManifestResult

// BuildDeliveryManifest 给公开页返回按 sha 寻址的不可变资源地址；资源必须就绪且与引用的 sha/版本一致。
func BuildDeliveryManifest(r *http.Request, op OperationContext, input BuildDeliveryManifestInput) (BuildDeliveryManifestResult, error) {
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return BuildDeliveryManifestResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体存储未装配")
	}
	items := make([]gen.DeliveryItem, 0, len(input.Assets))
	for _, ref := range input.Assets {
		var site, sha, state, assetJSON string
		err := s.db.QueryRowContext(r.Context(), `SELECT site_id,sha256,state,asset_json FROM assets WHERE asset_id=?`, ref.AssetId).Scan(&site, &sha, &state, &assetJSON)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (site != op.Actor.SiteID || state != "ready" || sha != ref.Sha256)) {
			return BuildDeliveryManifestResult{}, platform.NewError(platform.CodeStaleResult, false, "引用的资源不存在或已变化")
		}
		if err != nil {
			return BuildDeliveryManifestResult{}, mapMediaDBError(err, "读取资源失败")
		}
		item := gen.DeliveryItem{Asset: ref, Url: "/v1/assets/" + ref.AssetId + "/bytes?sha=" + sha, CachePolicy: gen.DeliveryItemCachePolicy("immutable_public"), ContentDisposition: gen.DeliveryItemContentDisposition("inline"), Variants: []gen.ImageVariant{}}
		var asset gen.Asset
		if json.Unmarshal([]byte(assetJSON), &asset) == nil && !asset.Animated {
			item.Width, item.Height = asset.Width, asset.Height
			if asset.ImageVariants != nil {
				item.Variants = asset.ImageVariants
			}
		}
		items = append(items, item)
	}
	return BuildDeliveryManifestResult{Manifest: gen.DeliveryManifest{Items: items, PublishedReference: input.PublishedReference, PreviewId: input.PreviewId, ExpiresAt: time.Now().UTC().Add(10 * time.Minute)}}, nil
}
