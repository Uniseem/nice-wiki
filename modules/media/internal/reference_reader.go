package media

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

type ReadAssetReferencesInput struct {
	AssetId Identifier `json:"assetId"`
}
type ReadAssetReferencesResult = gen.ReadAssetReferencesResult

// ReadAssetReferences 只把两个权威源的完整同水位快照和真实本地pending保护合并。
func ReadAssetReferences(ctx context.Context, op OperationContext, input ReadAssetReferencesInput) (ReadAssetReferencesResult, error) {
	s, err := cleanupStore(op, false)
	if err != nil {
		return ReadAssetReferencesResult{}, err
	}
	asset, _, generation, err := s.readCleanupAsset(ctx, s.db, op.Actor.SiteID, input.AssetId)
	if err != nil {
		return ReadAssetReferencesResult{}, err
	}
	content, contentWatermark, err := s.readSourceReferences(ctx, op, asset.Ref, "content")
	if err != nil {
		return ReadAssetReferencesResult{}, err
	}
	config, configWatermark, err := s.readSourceReferences(ctx, op, asset.Ref, "site-config")
	if err != nil {
		return ReadAssetReferencesResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReadAssetReferencesResult{}, mapMediaDBError(err, "开始引用快照读取失败")
	}
	defer tx.Rollback()
	current, _, actualGeneration, err := s.readCleanupAsset(ctx, tx, op.Actor.SiteID, input.AssetId)
	if err != nil {
		return ReadAssetReferencesResult{}, err
	}
	if current.Ref != asset.Ref || actualGeneration != generation {
		return ReadAssetReferencesResult{}, platform.NewError(platform.CodeVersionMismatch, false, "回源期间资源引用水位改变")
	}
	pending, err := s.pendingAssetReferences(ctx, tx, op, asset.Ref)
	if err != nil {
		return ReadAssetReferencesResult{}, err
	}
	snapshot := gen.ReferenceSnapshot{AssetId: input.AssetId, CurrentReferences: []gen.AssetReference{}, HistoryReferences: []gen.AssetReference{}, ReferenceGeneration: generation}
	// 水位仅为本次完整权威读取的证明，不能作为未来删除时跳过回源的授权。
	snapshot.SourceWatermarks = append(snapshot.SourceWatermarks, struct {
		Complete      bool         `json:"complete"`
		SourceService string       `json:"sourceService"`
		Watermark     gen.Revision `json:"watermark"`
	}{true, "content", contentWatermark}, struct {
		Complete      bool         `json:"complete"`
		SourceService string       `json:"sourceService"`
		Watermark     gen.Revision `json:"watermark"`
	}{true, "site-config", configWatermark})
	seen := make(map[string]string)
	for _, ref := range append(append(content, config...), pending...) {
		raw, err := platform.CanonicalJSON(ref)
		if err != nil {
			return ReadAssetReferencesResult{}, err
		}
		key := string(ref.SourceService) + "\x00" + ref.SourceObjectId + "\x00" + ref.SourceRevisionId + "\x00" + strconv.Itoa(ref.SourceVersion) + "\x00" + ref.ReferenceId + "\x00" + string(ref.Kind)
		if previous, found := seen[key]; found {
			if previous != string(raw) {
				return ReadAssetReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "同一引用身份存在冲突材料")
			}
			continue
		}
		seen[key] = string(raw)
		if ref.Kind == gen.History {
			snapshot.HistoryReferences = append(snapshot.HistoryReferences, ref)
		} else {
			snapshot.CurrentReferences = append(snapshot.CurrentReferences, ref)
		}
	}
	for _, refs := range [][]gen.AssetReference{snapshot.CurrentReferences, snapshot.HistoryReferences} {
		slices.SortFunc(refs, func(a, b gen.AssetReference) int {
			x, _ := platform.CanonicalJSON(a)
			y, _ := platform.CanonicalJSON(b)
			return strings.Compare(string(x), string(y))
		})
	}
	raw, err := platform.CanonicalJSON(snapshot)
	if err != nil {
		return ReadAssetReferencesResult{}, err
	}
	if s.validateSnapshot(raw) != nil {
		return ReadAssetReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "完整引用快照超出登记预算")
	}
	return ReadAssetReferencesResult{Snapshot: snapshot}, nil
}

// readSourceReferences 使用生产方唯一schema验证后才解码本域投影；不依赖裁剪后的生成DTO。
func (s *localBlobStore) readSourceReferences(ctx context.Context, op OperationContext, asset gen.AssetVersionRef, source string) ([]gen.AssetReference, gen.Revision, error) {
	if s.contentDecisions == nil || s.configDecisions == nil || s.validateContentPage == nil || s.validateConfigPage == nil {
		return nil, 0, platform.NewError(platform.CodeDependencyUnavailable, true, "完整引用源客户端未装配")
	}
	readOp := op
	readOp.CallerService = "media"
	readOp.Actor = platform.AuthContext{Kind: "service", SubjectID: "media", SiteID: op.Actor.SiteID, ExpiresAt: op.Deadline}
	readCtx := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), readOp)
	result := []gen.AssetReference{}
	cursor := ""
	seen := make(map[string]bool)
	watermark := -1
	rowCount := 0
	for {
		query := url.Values{"assetId": {asset.AssetId}, "limit": {"100"}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		var raw json.RawMessage
		var err error
		validate := s.validateContentPage
		if source == "content" {
			raw, err = s.contentDecisions.ListResourceReferences(readCtx, query)
		} else {
			validate = s.validateConfigPage
			raw, err = s.configDecisions.ListResourceReferences(readCtx, query)
		}
		if err != nil {
			return nil, 0, err
		}
		var page struct {
			References []json.RawMessage `json:"references"`
			Watermark  gen.Revision      `json:"watermark"`
			PageInfo   gen.PageInfo      `json:"pageInfo"`
		}
		if validate(raw) != nil || json.Unmarshal(raw, &page) != nil {
			return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "引用源分页不符合唯一schema")
		}
		if watermark < 0 {
			watermark = page.Watermark
		} else if watermark != page.Watermark {
			return nil, 0, platform.NewError(platform.CodeDependencyUnavailable, true, "引用源分页水位变化，不能认定完整")
		}
		rowCount += len(page.References)
		if rowCount > 20000 {
			return nil, 0, platform.NewError(platform.CodeDependencyUnavailable, true, "引用源分页总量超出登记扫描预算")
		}
		for _, rawRef := range page.References {
			var ref gen.AssetReference
			if source == "content" {
				var item struct {
					SourceService    string              `json:"sourceService"`
					DocumentId       Identifier          `json:"documentId"`
					SourceRevisionId Identifier          `json:"sourceRevisionId"`
					SourceVersion    gen.Revision        `json:"sourceVersion"`
					Kind             string              `json:"kind"`
					BlockId          Identifier          `json:"blockId"`
					Asset            gen.AssetVersionRef `json:"asset"`
					Retained         bool                `json:"retained"`
				}
				if json.Unmarshal(rawRef, &item) != nil || item.SourceService != source || item.Asset != asset {
					return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "文档引用源或不可变资产不匹配")
				}
				if item.Kind == "pending_pin" && !item.Retained {
					return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "待提交引用必须保留保护")
				}
				if !item.Retained {
					continue
				}
				ref = gen.AssetReference{SourceService: gen.AssetReferenceSourceService(source), SourceObjectId: item.DocumentId, SourceRevisionId: item.SourceRevisionId, SourceVersion: item.SourceVersion, ReferenceId: item.BlockId, Asset: item.Asset, Kind: gen.Current, Confirmed: true}
				switch item.Kind {
				case "current_draft", "current_published":
				case "history":
					ref.Kind = gen.History
				case "pending_pin":
					ref.Kind = gen.PendingPin
					ref.Confirmed = false
				default:
					return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "文档引用源kind未登记")
				}
			} else if json.Unmarshal(rawRef, &ref) != nil || string(ref.SourceService) != source || ref.Asset != asset || !ref.Confirmed || ref.ExpiresAt != nil || (ref.Kind != gen.Current && ref.Kind != gen.History) {
				return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "配置引用来源、终局或资产不匹配")
			}
			result = append(result, ref)
			if len(result) > 20000 {
				return nil, 0, platform.NewError(platform.CodeDependencyUnavailable, true, "引用源完整集合超出登记预算")
			}
		}
		if !page.PageInfo.HasMore {
			if page.PageInfo.NextCursor != nil && *page.PageInfo.NextCursor != "" {
				return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "引用末页仍声明下一游标")
			}
			break
		}
		if len(page.References) == 0 || page.PageInfo.NextCursor == nil || *page.PageInfo.NextCursor == "" || seen[*page.PageInfo.NextCursor] {
			return nil, 0, platform.NewError(platform.CodeDependencyUnavailable, true, "引用源分页未完整或游标未前进")
		}
		cursor = *page.PageInfo.NextCursor
		seen[cursor] = true
	}
	return result, watermark, nil
}

func (s *localBlobStore) pendingAssetReferences(ctx context.Context, q sqlExecer, op OperationContext, asset gen.AssetVersionRef) ([]gen.AssetReference, error) {
	rows, err := q.QueryContext(ctx, `SELECT s.source_service,s.source_operation_id FROM reference_sources s JOIN reference_pin_assets p ON p.pin_id=s.pin_id WHERE p.asset_id=? AND s.data_epoch=? AND s.site_id=? AND s.state='pending' ORDER BY s.source_service,s.source_operation_id`, asset.AssetId, op.DataEpoch, op.Actor.SiteID)
	if err != nil {
		return nil, mapMediaDBError(err, "读取待提交引用保护失败")
	}
	type sourceKey struct{ service, operation string }
	keys := []sourceKey{}
	for rows.Next() {
		var key sourceKey
		if err := rows.Scan(&key.service, &key.operation); err != nil {
			rows.Close()
			return nil, mapMediaDBError(err, "读取待提交来源失败")
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, mapMediaDBError(err, "遍历待提交来源失败")
	}
	result := []gen.AssetReference{}
	for _, key := range keys {
		sourceOp := op
		sourceOp.CallerService = key.service
		record, found, err := s.readReferenceRecord(ctx, q, sourceOp, key.operation)
		if err != nil {
			return nil, err
		}
		if !found || record.State != "pending" || record.Input == nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "待提交保护缺完整来源")
		}
		original, _, err := s.readReferenceSource(ctx, q, sourceOp, *record.Input, record.BindingHash)
		if err != nil {
			return nil, err
		}
		for _, ref := range record.Input.References {
			if ref.Asset.AssetId == asset.AssetId {
				if ref.Asset != asset {
					return nil, platform.NewError(platform.CodeIntegrityError, false, "待提交保护不可变资产不匹配")
				}
				expires := original.Pin.ExpiresAt
				ref.ExpiresAt = &expires
				result = append(result, ref)
			}
		}
	}
	return result, nil
}
