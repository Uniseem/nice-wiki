package siteconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"sort"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/site-config/internal/gen"
)

type ResourceReferencePageRequest struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor,omitempty"`
}

type ListResourceReferencesInput struct {
	AssetID *gen.Identifier              `json:"assetId"`
	Page    ResourceReferencePageRequest `json:"page"`
}

type ListResourceReferencesResult = gen.ListResourceReferencesResult

type ResourceReferenceCursor struct {
	Watermark gen.Revision              `json:"watermark"`
	After     ResourceReferencePosition `json:"after"`
}

type ResourceReferencePosition struct {
	SourceVersion    gen.Revision   `json:"sourceVersion"`
	SourceRevisionID gen.Identifier `json:"sourceRevisionId"`
	ReferenceID      gen.Identifier `json:"referenceId"`
	AssetID          gen.Identifier `json:"assetId"`
	AssetVersion     gen.Revision   `json:"assetVersion"`
	AssetSHA256      gen.Hash       `json:"assetSha256"`
}

// ListResourceReferences 从同一活动 epoch 的 COMMITTED 历史生成完整引用页。
// 来源标识复用首页持久 pin 的规则，媒体只需消费当前页并在分页未完成时继续读取。
func ListResourceReferences(ctx context.Context, op platform.OperationContext, input ListResourceReferencesInput) (ListResourceReferencesResult, error) {
	if err := authorizeResourceReferenceRead(op); err != nil {
		return ListResourceReferencesResult{}, err
	}
	limit := input.Page.Limit
	if limit == 0 {
		limit = 20
	}
	store := currentActiveStore()
	if store == nil {
		return ListResourceReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	key, err := store.historyCursorKey()
	if err != nil {
		return ListResourceReferencesResult{}, err
	}

	store.configMu.Lock()
	defer store.configMu.Unlock()
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ListResourceReferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开资源引用事务失败")
	}
	defer tx.Rollback()

	siteID, watermark, currentRevisionID, err := store.readConfigWatermark(ctx, op, tx)
	if err != nil {
		return ListResourceReferencesResult{}, err
	}
	if op.Actor.SiteID != siteID {
		return ListResourceReferencesResult{}, platform.NewError(platform.CodeForbidden, false, "资源引用站点与受信主体不一致")
	}
	if _, _, err := store.readConfigAtWatermark(ctx, op, tx, siteID, watermark, currentRevisionID); err != nil {
		return ListResourceReferencesResult{}, err
	}

	revisions, err := store.listCommittedRevisions(siteID, op.DataEpoch)
	if err != nil {
		return ListResourceReferencesResult{}, err
	}
	references := make([]gen.AssetReference, 0)
	for _, historical := range revisions {
		if historical.ConfigRevision > watermark {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源引用历史超过当前水位")
		}
		if historical.ConfigRevision == watermark && historical.RevisionID != currentRevisionID {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源引用当前修订存在歧义")
		}
		config, record, _, err := store.readIndexedConfig(ctx, tx, historical.RevisionID)
		if err != nil {
			return ListResourceReferencesResult{}, err
		}
		if record.RevisionID != historical.RevisionID || record.Object.Version != historical.ConfigRevision || record.DataEpoch != op.DataEpoch {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源引用历史与配置索引不一致")
		}
		kind := gen.AssetReferenceKind("history")
		if record.RevisionID == currentRevisionID {
			kind = gen.AssetReferenceKind("current")
		}
		homeRefs, _, err := homeReferenceMaterial(config, record.RevisionID)
		if err != nil {
			return ListResourceReferencesResult{}, err
		}
		brandingRefs, _, err := brandingReferenceMaterial(config, record.RevisionID)
		if err != nil {
			return ListResourceReferencesResult{}, err
		}
		for _, reference := range append(homeRefs, brandingRefs...) {
			references = append(references, gen.AssetReference{
				SourceService:    gen.AssetReferenceSourceService(reference.SourceService),
				SourceObjectId:   gen.Identifier(reference.SourceObjectID),
				SourceRevisionId: gen.Identifier(reference.SourceRevisionID),
				SourceVersion:    gen.Revision(reference.SourceVersion),
				ReferenceId:      gen.Identifier(reference.ReferenceID),
				Asset:            reference.Asset,
				Kind:             kind,
				Confirmed:        true,
				ExpiresAt:        nil,
			})
		}
	}
	sort.Slice(references, func(i, j int) bool {
		left, right := references[i], references[j]
		if left.SourceVersion != right.SourceVersion {
			return left.SourceVersion > right.SourceVersion
		}
		if left.SourceRevisionId != right.SourceRevisionId {
			return left.SourceRevisionId > right.SourceRevisionId
		}
		if left.ReferenceId != right.ReferenceId {
			return left.ReferenceId < right.ReferenceId
		}
		if left.Asset.AssetId != right.Asset.AssetId {
			return left.Asset.AssetId < right.Asset.AssetId
		}
		if left.Asset.Version != right.Asset.Version {
			return left.Asset.Version < right.Asset.Version
		}
		return left.Asset.Sha256 < right.Asset.Sha256
	})
	if input.AssetID != nil {
		references = slices.DeleteFunc(references, func(reference gen.AssetReference) bool {
			return reference.Asset.AssetId != *input.AssetID
		})
	}
	positionOf := func(reference gen.AssetReference) ResourceReferencePosition {
		return ResourceReferencePosition{
			SourceVersion:    reference.SourceVersion,
			SourceRevisionID: reference.SourceRevisionId,
			ReferenceID:      reference.ReferenceId,
			AssetID:          reference.Asset.AssetId,
			AssetVersion:     reference.Asset.Version,
			AssetSHA256:      reference.Asset.Sha256,
		}
	}

	filter := ""
	if input.AssetID != nil {
		filter = string(*input.AssetID)
	}
	binding, err := platform.CanonicalJSON(struct {
		Caller    string `json:"caller"`
		Actor     string `json:"actor"`
		SiteID    string `json:"siteId"`
		DataEpoch string `json:"dataEpoch"`
		AssetID   string `json:"assetId"`
		Limit     int    `json:"limit"`
	}{op.CallerService, op.Actor.SubjectID, siteID, op.DataEpoch, filter, limit})
	if err != nil {
		return ListResourceReferencesResult{}, err
	}
	cursorScope := "site-config.resource-references.v1:" + string(platform.HashBytes(binding))
	offset := 0
	if input.Page.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(key, cursorScope, input.Page.Cursor)
		if err != nil {
			return ListResourceReferencesResult{}, err
		}
		var cursor ResourceReferenceCursor
		if decodeConfigJSON(raw, &cursor) != nil {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeValidation, false, "资源引用游标载荷不合法")
		}
		if cursor.Watermark < 1 {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeValidation, false, "资源引用游标水位不合法")
		}
		if cursor.Watermark != watermark {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeVersionMismatch, false, "资源引用游标水位已变化")
		}
		position := cursor.After
		if position.SourceVersion < 1 ||
			!platform.ValidIdentifier(string(position.SourceRevisionID)) ||
			!platform.ValidIdentifier(string(position.ReferenceID)) ||
			!platform.ValidIdentifier(string(position.AssetID)) ||
			position.AssetVersion < 1 || !platform.ValidHash(position.AssetSHA256) {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeValidation, false, "资源引用游标载荷不合法")
		}
		found := -1
		for i, reference := range references {
			if positionOf(reference) == position {
				found = i
				break
			}
		}
		if found < 0 {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeVersionMismatch, false, "资源引用游标位置已失效")
		}
		offset = found + 1
	}
	end := offset + limit
	if end > len(references) {
		end = len(references)
	}
	page := append([]gen.AssetReference(nil), references[offset:end]...)
	if page == nil {
		page = []gen.AssetReference{}
	}
	pageInfo := gen.PageInfo{HasMore: end < len(references)}
	if pageInfo.HasMore {
		next, err := json.Marshal(ResourceReferenceCursor{Watermark: watermark, After: positionOf(references[end-1])})
		if err != nil {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeInternal, false, "资源引用游标无法编码")
		}
		token, err := serviceauth.SignCursor(key, cursorScope, next)
		if err != nil {
			return ListResourceReferencesResult{}, err
		}
		pageInfo.NextCursor = &token
	}
	return ListResourceReferencesResult{References: page, Watermark: watermark, PageInfo: pageInfo}, nil
}

func authorizeResourceReferenceRead(op platform.OperationContext) error {
	if op.Actor.Kind == "service" {
		if slices.Contains([]string{"media", "data-protection"}, op.CallerService) && op.Actor.SiteID != "" && op.Actor.SiteID != "system" {
			return nil
		}
	} else if op.Actor.Kind == "admin" && op.Actor.SessionID != "" && op.Actor.SiteID != "" && op.Actor.SubjectID != "" &&
		(slices.Contains(op.Actor.Grants, "site.manage") || slices.Contains(op.Actor.Grants, "superadmin")) {
		return nil
	}
	return platform.NewError(platform.CodeForbidden, false, "没有读取站点资源引用的权限")
}
