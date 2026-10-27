package siteconfig

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/site-config/internal/gen"
)

type ConfigPageRequest struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor,omitempty"`
}

type ReadConfigHistoryInput struct {
	Page ConfigPageRequest `json:"page"`
}

type RestoreConfigRevisionInput struct {
	RevisionID       gen.Identifier `json:"revisionId"`
	ExpectedRevision gen.Revision   `json:"expectedRevision"`
}

type historyCursor struct {
	BeforeRevision int `json:"beforeRevision"`
}

type historyRevision struct {
	AuthorAccountId gen.Identifier `json:"authorAccountId"`
	ConfigRevision  gen.Revision   `json:"configRevision"`
	CreatedAt       gen.Timestamp  `json:"createdAt"`
	Object          gen.ObjectRef  `json:"object"`
	RevisionId      gen.Identifier `json:"revisionId"`
}

func (s *ActiveStore) ConfigureCursorKey(key platform.SigningKey) {
	s.stateMu.Lock()
	s.cursorKey = key
	s.cursorReady = true
	s.stateMu.Unlock()
}

func (s *ActiveStore) historyCursorKey() (platform.SigningKey, error) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if !s.cursorReady || s.cursorKey.KeyID == "" || s.cursorKey.PrivateKeyRef == "" {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "历史游标签名密钥不可用")
	}
	return s.cursorKey, nil
}

func sameAssetRef(left, right *gen.AssetVersionRef) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sameConfigTime(left, right *gen.Timestamp) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.UTC().Equal(right.UTC())
}

func sameCanonical(left, right any) (bool, error) {
	rawLeft, err := platform.CanonicalJSON(left)
	if err != nil {
		return false, err
	}
	rawRight, err := platform.CanonicalJSON(right)
	if err != nil {
		return false, err
	}
	return bytes.Equal(rawLeft, rawRight), nil
}

func sameBannerProjection(left, right *gen.Announcement) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.AnnouncementId == right.AnnouncementId && left.RevisionId == right.RevisionId && left.BodyHash == right.BodyHash &&
		left.Enabled == right.Enabled && left.VisibilityRevision == right.VisibilityRevision && left.SiteId == right.SiteId &&
		sameConfigTime(left.StartsAt, right.StartsAt) && sameConfigTime(left.EndsAt, right.EndsAt)
}

func configChangedSections(previous, next gen.SiteConfig) ([]string, error) {
	sections := make([]string, 0)
	if previous.PublicBaseURL != next.PublicBaseURL {
		sections = append(sections, "identity")
	}
	if previous.Name != next.Name || previous.ThemeColor != next.ThemeColor || !sameAssetRef(previous.Logo, next.Logo) || !sameAssetRef(previous.Favicon, next.Favicon) || !sameAssetRef(previous.DefaultShareImage, next.DefaultShareImage) {
		sections = append(sections, "branding")
	}
	homeSame, err := sameCanonical(previous.Home, next.Home)
	if err != nil {
		return nil, err
	}
	if !homeSame {
		sections = append(sections, "home")
	}
	if previous.TitleTemplate != next.TitleTemplate || previous.Description != next.Description || previous.IndexingEnabled != next.IndexingEnabled {
		sections = append(sections, "seo")
	}
	securitySame, err := sameCanonical(previous.SecurityPolicy, next.SecurityPolicy)
	if err != nil {
		return nil, err
	}
	if !securitySame {
		sections = append(sections, "security")
	}
	limitsSame, err := sameCanonical(previous.ResourceLimits, next.ResourceLimits)
	if err != nil {
		return nil, err
	}
	if !limitsSame {
		sections = append(sections, "limits")
	}
	if previous.BaseLanguage != next.BaseLanguage {
		sections = append(sections, "language")
	}
	footerSame, err := sameCanonical(previous.Footer, next.Footer)
	if err != nil {
		return nil, err
	}
	if !footerSame {
		sections = append(sections, "footer")
	}
	if !sameBannerProjection(previous.Banner, next.Banner) {
		sections = append(sections, "banner")
	}
	return sections, nil
}

// ReadConfigHistory 按COMMITTED journal分页，不把SQL偏移当历史权威。
func ReadConfigHistory(ctx context.Context, op platform.OperationContext, input ReadConfigHistoryInput) (gen.ReadConfigHistoryResult, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return gen.ReadConfigHistoryResult{}, err
	}
	limit := input.Page.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return gen.ReadConfigHistoryResult{}, platform.NewError(platform.CodeValidation, false, "历史分页参数不合法")
	}
	store := currentActiveStore()
	if store == nil {
		return gen.ReadConfigHistoryResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	key, err := store.historyCursorKey()
	if err != nil {
		return gen.ReadConfigHistoryResult{}, err
	}
	store.configMu.Lock()
	defer store.configMu.Unlock()
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return gen.ReadConfigHistoryResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开配置历史事务失败")
	}
	defer tx.Rollback()
	siteID, _, _, err := store.readConfigWatermark(ctx, op, tx)
	if err != nil {
		return gen.ReadConfigHistoryResult{}, err
	}
	binding, err := platform.CanonicalJSON(struct {
		Actor string `json:"actor"`
		Site  string `json:"site"`
		Epoch string `json:"epoch"`
		Limit int    `json:"limit"`
	}{op.Actor.SubjectID, siteID, op.DataEpoch, limit})
	if err != nil {
		return gen.ReadConfigHistoryResult{}, err
	}
	scope := "site-config.history.v1:" + platform.HashBytes(binding)
	before := int(^uint(0) >> 1)
	if input.Page.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(key, scope, input.Page.Cursor)
		if err != nil {
			return gen.ReadConfigHistoryResult{}, err
		}
		var position historyCursor
		if decodeConfigJSON(raw, &position) != nil || position.BeforeRevision < 1 {
			return gen.ReadConfigHistoryResult{}, platform.NewError(platform.CodeValidation, false, "历史分页游标载荷不可用")
		}
		before = position.BeforeRevision
	}
	revisions, err := store.listCommittedRevisions(siteID, op.DataEpoch)
	if err != nil {
		return gen.ReadConfigHistoryResult{}, err
	}
	selected := make([]historyRevision, 0)
	for _, revision := range revisions {
		if revision.ConfigRevision < before {
			selected = append(selected, revision)
		}
	}
	end := limit
	if end > len(selected) {
		end = len(selected)
	}
	page := selected[:end]
	result := gen.ReadConfigHistoryResult{Revisions: make([]struct {
		AuthorAccountId gen.Identifier `json:"authorAccountId"`
		ConfigRevision  gen.Revision   `json:"configRevision"`
		CreatedAt       gen.Timestamp  `json:"createdAt"`
		Object          gen.ObjectRef  `json:"object"`
		RevisionId      gen.Identifier `json:"revisionId"`
	}, 0, len(page)), PageInfo: gen.PageInfo{HasMore: end < len(selected)}}
	for _, revision := range page {
		result.Revisions = append(result.Revisions, struct {
			AuthorAccountId gen.Identifier `json:"authorAccountId"`
			ConfigRevision  gen.Revision   `json:"configRevision"`
			CreatedAt       gen.Timestamp  `json:"createdAt"`
			Object          gen.ObjectRef  `json:"object"`
			RevisionId      gen.Identifier `json:"revisionId"`
		}{AuthorAccountId: revision.AuthorAccountId, ConfigRevision: revision.ConfigRevision, CreatedAt: revision.CreatedAt, Object: revision.Object, RevisionId: revision.RevisionId})
	}
	if result.PageInfo.HasMore {
		raw, err := json.Marshal(historyCursor{BeforeRevision: page[len(page)-1].ConfigRevision})
		if err != nil {
			return gen.ReadConfigHistoryResult{}, platform.NewError(platform.CodeIntegrityError, false, "历史游标无法编码")
		}
		cursor, err := serviceauth.SignCursor(key, scope, raw)
		if err != nil {
			return gen.ReadConfigHistoryResult{}, err
		}
		result.PageInfo.NextCursor = &cursor
	}
	return result, nil
}

func (s *ActiveStore) listCommittedRevisions(siteID, epoch string) ([]historyRevision, error) {
	paths, err := configJournalPaths(s.activeRoot, "COMMITTED")
	if err != nil {
		return nil, err
	}
	revisions := make([]historyRevision, 0)
	for _, path := range paths {
		record, err := readConfigRecord(path)
		if err != nil {
			return nil, err
		}
		if record.SiteID != siteID || record.DataEpoch != epoch {
			continue
		}
		config, err := readConfigDocument(s.activeRoot, record)
		if err != nil {
			return nil, err
		}
		if config.UpdatedAt.IsZero() || config.Revision != record.Object.Version {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "历史配置修订不完整")
		}
		revisions = append(revisions, historyRevision{AuthorAccountId: record.ActorID, ConfigRevision: config.Revision, CreatedAt: config.UpdatedAt, Object: record.Object, RevisionId: record.RevisionID})
	}
	for i := 0; i < len(revisions); i++ {
		for j := i + 1; j < len(revisions); j++ {
			if revisions[j].ConfigRevision > revisions[i].ConfigRevision {
				revisions[i], revisions[j] = revisions[j], revisions[i]
			}
		}
	}
	return revisions, nil
}

// RestoreConfigRevision 只回到真实COMMITTED修订，并生成新的配置版本。
func RestoreConfigRevision(ctx context.Context, op platform.OperationContext, input RestoreConfigRevisionInput) (gen.RestoreConfigRevisionResult, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return gen.RestoreConfigRevisionResult{}, err
	}
	store := currentActiveStore()
	if store == nil {
		return gen.RestoreConfigRevisionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	store.settingsMu.Lock()
	defer store.settingsMu.Unlock()
	requestRaw, err := platform.CanonicalJSON(restoreRequest{Kind: "restore", Input: input})
	if err != nil {
		return gen.RestoreConfigRevisionResult{}, err
	}
	requestHash := platform.HashBytes(requestRaw)
	receipt, err := ResolveConfigOperation(ctx, op, ResolveConfigOperationInput{OperationId: op.OperationID, RequestHash: requestHash})
	if err != nil {
		return gen.RestoreConfigRevisionResult{}, err
	}
	if receipt.State == "committed" {
		config, err := store.readConfigOperationResult(ctx, op, requestHash, receipt.ResultRevision)
		return gen.RestoreConfigRevisionResult{Config: config}, err
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return gen.RestoreConfigRevisionResult{}, err
	}
	updatedAt := time.Now().UTC()
	var candidate configCandidate
	var sections []string
	if receipt.State == "in_progress" {
		candidate, err = store.readConfigCandidate(ctx, store.db, op.OperationID, "restore")
		if err != nil {
			return gen.RestoreConfigRevisionResult{}, err
		}
		updatedAt = candidate.Config.UpdatedAt
	}
	rebuilt, sections, err := store.buildRestoredConfig(ctx, op, input, updatedAt)
	if err != nil {
		return gen.RestoreConfigRevisionResult{}, err
	}
	if receipt.State == "in_progress" {
		same, cmpErr := sameCanonical(rebuilt, candidate.Config)
		if cmpErr != nil {
			return gen.RestoreConfigRevisionResult{}, cmpErr
		}
		if !same || candidate.Config.Revision != input.ExpectedRevision+1 {
			return gen.RestoreConfigRevisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "恢复候选与原始历史修订不一致")
		}
	}
	uses, err := restoredMediaUses(rebuilt)
	if err != nil {
		return gen.RestoreConfigRevisionResult{}, err
	}
	if err := validateMediaAssets(ctx, op, rebuilt.SiteId, uses); err != nil {
		return gen.RestoreConfigRevisionResult{}, err
	}
	if receipt.State != "in_progress" {
		candidate, err = store.reserveConfigCandidate(ctx, op, "restore", requestHash, rebuilt)
		if err != nil {
			return gen.RestoreConfigRevisionResult{}, err
		}
	}
	written, err := WriteConfigRevision(ctx, op, WriteConfigRevisionInput{Config: candidate.Config, RevisionId: candidate.RevisionID})
	if err != nil {
		return gen.RestoreConfigRevisionResult{}, err
	}
	committed, err := CommitConfigHead(ctx, op, CommitConfigHeadInput{Object: written.Object, ExpectedRevision: input.ExpectedRevision, RequestHash: requestHash, ChangedSections: sections})
	if err != nil {
		return gen.RestoreConfigRevisionResult{}, err
	}
	return gen.RestoreConfigRevisionResult{Config: committed.Config}, nil
}

type restoreRequest struct {
	Kind  string                     `json:"kind"`
	Input RestoreConfigRevisionInput `json:"input"`
}

func (s *ActiveStore) buildRestoredConfig(ctx context.Context, op platform.OperationContext, input RestoreConfigRevisionInput, updatedAt time.Time) (gen.SiteConfig, []string, error) {
	current, err := s.loadCurrentConfig(ctx, op)
	if err != nil {
		return gen.SiteConfig{}, nil, err
	}
	if current.Revision != input.ExpectedRevision {
		return gen.SiteConfig{}, nil, platform.NewError(platform.CodeVersionMismatch, false, "配置expectedRevision已变化")
	}
	historical, record, err := s.readHistoricalConfig(string(input.RevisionID))
	if err != nil {
		return gen.SiteConfig{}, nil, err
	}
	if record.DataEpoch != op.DataEpoch {
		return gen.SiteConfig{}, nil, platform.NewError(platform.CodeVersionMismatch, false, "历史修订不属于活动dataEpoch")
	}
	if historical.SiteId != current.SiteId || record.SiteID != current.SiteId {
		return gen.SiteConfig{}, nil, platform.NewError(platform.CodeValidation, false, "历史修订不属于当前站点")
	}
	if historical.Banner != nil && historical.Banner.SiteId != current.SiteId {
		return gen.SiteConfig{}, nil, platform.NewError(platform.CodeValidation, false, "历史公告不属于当前站点")
	}
	if err := historicalAnnouncementAvailable(s.activeRoot, historical.Banner); err != nil {
		return gen.SiteConfig{}, nil, err
	}
	next := historical
	next.SiteId = current.SiteId
	next.BaseLanguage = current.BaseLanguage
	next.Initialized = current.Initialized
	next.Revision = current.Revision + 1
	next.UpdatedAt = updatedAt.UTC()
	if next.Banner != nil {
		banner := *next.Banner
		banner.SiteId = next.SiteId
		banner.Revision = next.Revision
		banner.UpdatedAt = next.UpdatedAt
		next.Banner = &banner
	}
	sections, err := configChangedSections(current, next)
	if err != nil {
		return next, nil, err
	}
	if len(sections) == 0 {
		sections = append([]string{}, record.ChangedSections...)
	}
	if len(sections) == 0 {
		return next, nil, platform.NewError(platform.CodeIntegrityError, false, "历史修订缺少变更组")
	}
	return next, sections, nil
}

func (s *ActiveStore) readHistoricalConfig(revisionID string) (gen.SiteConfig, configJournalRecord, error) {
	if !configPathID(revisionID) {
		return gen.SiteConfig{}, configJournalRecord{}, platform.NewError(platform.CodeValidation, false, "历史修订标识不合法")
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	paths, err := configJournalPaths(s.activeRoot, "COMMITTED")
	if err != nil {
		return gen.SiteConfig{}, configJournalRecord{}, err
	}
	path := configRecordPath(s.activeRoot, revisionID, "COMMITTED")
	found := false
	for _, candidate := range paths {
		if candidate == path {
			found = true
			break
		}
	}
	if !found {
		return gen.SiteConfig{}, configJournalRecord{}, platform.NewError(platform.CodeValidation, false, "历史修订不存在")
	}
	record, err := readConfigRecord(path)
	if err != nil {
		return gen.SiteConfig{}, configJournalRecord{}, err
	}
	config, err := readConfigDocument(s.activeRoot, record)
	if err != nil {
		return gen.SiteConfig{}, record, err
	}
	return config, record, nil
}

func historicalAnnouncementAvailable(root string, banner *gen.Announcement) error {
	if banner == nil {
		return nil
	}
	raw, exists, err := readRegularConfigFile(announcementBodyPath(root, string(banner.AnnouncementId), string(banner.RevisionId)))
	if err != nil {
		return err
	}
	if !exists || platform.HashBytes(raw) != banner.BodyHash || validateAnnouncementBody(raw) != nil {
		return platform.NewError(platform.CodeValidation, false, "历史公告正文缺失或不一致，拒绝回滚")
	}
	entries, _, err := readAnnouncementJournal(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.State == announcementStateCommitted && entry.AnnouncementID == banner.AnnouncementId && entry.RevisionID == banner.RevisionId && entry.BodyHash == banner.BodyHash {
			return nil
		}
	}
	return platform.NewError(platform.CodeValidation, false, "历史公告正文缺失或不一致，拒绝回滚")
}

func restoredMediaUses(config gen.SiteConfig) ([]brandingAssetUse, error) {
	_, assets, err := validateHomeStructure(config.Home)
	if err != nil {
		return nil, err
	}
	var background struct {
		Kind string `json:"kind"`
	}
	raw, err := config.Home.Background.MarshalJSON()
	if err != nil || json.Unmarshal(raw, &background) != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "历史首页背景无法读取")
	}
	uses := make([]brandingAssetUse, 0, len(assets)+3)
	for _, asset := range assets {
		use := brandingAssetUse{ReferenceID: asset.ReferenceID, Asset: asset.Asset, Kind: "image"}
		if strings.HasPrefix(asset.Asset.MediaType, "video/") {
			use.Kind = "video"
		}
		if asset.ReferenceID == "home.background" && (background.Kind == "animation" || background.Kind == "image") {
			animated := background.Kind == "animation"
			use.Animated = &animated
		}
		uses = append(uses, use)
	}
	return append(uses, brandingAssets(config)...), nil
}
