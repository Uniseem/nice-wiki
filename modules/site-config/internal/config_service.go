package siteconfig

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

type ReadSiteConfigInput struct{}

// ReadSiteConfig 只返回已完成初始化且head/COMMITTED/SQL一致的权威文件。
func ReadSiteConfig(ctx context.Context, op platform.OperationContext, input ReadSiteConfigInput) (gen.ReadSiteConfigResult, error) {
	if err := authorizeConfigRead(op, false); err != nil {
		return gen.ReadSiteConfigResult{}, err
	}
	store := currentActiveStore()
	if store == nil {
		return gen.ReadSiteConfigResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	config, err := store.loadCurrentConfig(ctx, op)
	return gen.ReadSiteConfigResult{Config: config}, err
}

// UpdateSiteConfig 将稳定候选材料与原始请求身份持久化后复用文件提交链路。
// 同操作已成功时读原修订，后续更新和维护不会改写原始结果。
func UpdateSiteConfig(ctx context.Context, op platform.OperationContext, input gen.UpdateSiteConfigInput) (gen.UpdateSiteConfigResult, error) {
	if err := authorizeConfigWrite(op); err != nil {
		return gen.UpdateSiteConfigResult{}, err
	}
	store := currentActiveStore()
	if store == nil {
		return gen.UpdateSiteConfigResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	store.settingsMu.Lock()
	defer store.settingsMu.Unlock()
	store.stateMu.RLock()
	localHTTP := store.allowLocalHTTP
	store.stateMu.RUnlock()
	requestHash, err := hashInput(struct {
		Kind  string                    `json:"kind"`
		Input gen.UpdateSiteConfigInput `json:"input"`
	}{"settings", input})
	if err != nil {
		return gen.UpdateSiteConfigResult{}, err
	}
	receipt, err := ResolveConfigOperation(ctx, op, ResolveConfigOperationInput{OperationId: op.OperationID, RequestHash: requestHash})
	if err != nil {
		return gen.UpdateSiteConfigResult{}, err
	}
	if receipt.State == "committed" {
		config, err := store.readConfigOperationResult(ctx, op, requestHash, receipt.ResultRevision)
		return gen.UpdateSiteConfigResult{Config: config}, err
	}
	// 维护白名单只为旧成功回执读取开放；任何候选读取/持久化和
	// PREPARED文件IO之前，新操作与未完成操作先失败关闭。
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return gen.UpdateSiteConfigResult{}, err
	}
	normalized, err := NormalizeBaseURL(NormalizeBaseURLInput{URL: input.PublicBaseURL, AllowLocalHTTP: localHTTP})
	if err != nil {
		return gen.UpdateSiteConfigResult{}, err
	}
	input.PublicBaseURL = normalized.Normalized
	var candidate configCandidate
	if receipt.State == "in_progress" {
		candidate, err = store.readConfigCandidate(ctx, store.db, op.OperationID, "settings")
		if err == nil {
			materialInput := gen.UpdateSiteConfigInput{PublicBaseURL: candidate.Config.PublicBaseURL, TitleTemplate: candidate.Config.TitleTemplate, Description: candidate.Config.Description,
				IndexingEnabled: candidate.Config.IndexingEnabled, ResourceLimits: candidate.Config.ResourceLimits, SecurityPolicy: candidate.Config.SecurityPolicy, ExpectedRevision: candidate.Config.Revision - 1}
			if materialInput != input {
				return gen.UpdateSiteConfigResult{}, platform.NewError(platform.CodeIntegrityError, false, "配置候选材料与原始设置字段不一致")
			}
			if _, err := ValidateSiteConfig(ValidateSiteConfigInput{Config: candidate.Config, AllowLocalHTTP: localHTTP}); err != nil {
				return gen.UpdateSiteConfigResult{}, err
			}
		}
	} else {
		current, readErr := store.loadCurrentConfig(ctx, op)
		if readErr != nil {
			return gen.UpdateSiteConfigResult{}, readErr
		}
		config := current
		if config.Revision != input.ExpectedRevision {
			return gen.UpdateSiteConfigResult{}, platform.NewError(platform.CodeVersionMismatch, false, "配置expectedRevision已变化")
		}
		config.PublicBaseURL, config.TitleTemplate, config.Description = input.PublicBaseURL, input.TitleTemplate, input.Description
		config.IndexingEnabled, config.ResourceLimits, config.SecurityPolicy = input.IndexingEnabled, input.ResourceLimits, input.SecurityPolicy
		config.Revision, config.UpdatedAt = config.Revision+1, time.Now().UTC()
		if _, err := ValidateSiteConfig(ValidateSiteConfigInput{Config: config, AllowLocalHTTP: localHTTP}); err != nil {
			return gen.UpdateSiteConfigResult{}, err
		}
		candidate, err = store.reserveConfigCandidate(ctx, op, "settings", requestHash, config)
	}
	if err != nil {
		return gen.UpdateSiteConfigResult{}, err
	}
	if candidate.Config.SiteId != op.Actor.SiteID || candidate.Config.Revision != input.ExpectedRevision+1 {
		return gen.UpdateSiteConfigResult{}, platform.NewError(platform.CodeIntegrityError, false, "配置候选材料与原始请求不一致")
	}
	written, err := WriteConfigRevision(ctx, op, WriteConfigRevisionInput{Config: candidate.Config, RevisionId: candidate.RevisionID})
	if err != nil {
		return gen.UpdateSiteConfigResult{}, err
	}
	committed, err := CommitConfigHead(ctx, op, CommitConfigHeadInput{Object: written.Object, ExpectedRevision: input.ExpectedRevision, RequestHash: requestHash,
		ChangedSections: []string{"identity", "seo", "limits", "security"}})
	if err != nil {
		return gen.UpdateSiteConfigResult{}, err
	}
	return gen.UpdateSiteConfigResult{Config: committed.Config}, nil
}

func authorizeConfigWrite(op platform.OperationContext) error {
	if op.Actor.Kind != "admin" || op.Actor.SessionID == "" || op.Actor.SiteID == "" || op.Actor.SubjectID == "" || (!slices.Contains(op.Actor.Grants, "site.manage") && !slices.Contains(op.Actor.Grants, "superadmin")) {
		return platform.NewError(platform.CodeForbidden, false, "没有管理站点配置的会话权限")
	}
	return nil
}

func authorizeConfigRead(op platform.OperationContext, object bool) error {
	if op.Actor.Kind == "service" {
		allowed := []string{"portal", "public-site", "content", "media", "localization", "identity", "knowledge", "model-gateway", "memory", "qa", "analytics", "traffic-guard", "jobs", "data-protection"}
		if object {
			allowed = []string{"public-site", "data-protection"}
		}
		if slices.Contains(allowed, op.CallerService) {
			return nil
		}
	} else if op.Actor.Kind == "admin" && op.Actor.SessionID != "" && op.Actor.SiteID != "" && op.Actor.SubjectID != "" &&
		(slices.Contains(op.Actor.Grants, "site.read") || slices.Contains(op.Actor.Grants, "superadmin")) {
		return nil
	}
	return platform.NewError(platform.CodeForbidden, false, "没有读取站点配置的权限")
}

func (s *ActiveStore) loadCurrentConfig(ctx context.Context, op platform.OperationContext) (gen.SiteConfig, error) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return gen.SiteConfig{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开配置读取事务失败")
	}
	defer tx.Rollback()
	config, _, err := s.readCurrentConfig(ctx, op, tx)
	return config, err
}

// readCurrentConfig 共享读取聚合权威边界；调用方持有configMu及只读事务。
func (s *ActiveStore) readCurrentConfig(ctx context.Context, op platform.OperationContext, tx *sql.Tx) (gen.SiteConfig, configJournalRecord, error) {
	siteID, revision, revisionID, err := s.readConfigWatermark(ctx, op, tx)
	if err != nil {
		return gen.SiteConfig{}, configJournalRecord{}, err
	}
	return s.readConfigAtWatermark(ctx, op, tx, siteID, revision, revisionID)
}

// readConfigAtWatermark 复用同一文件权威边界；调用方已按私有或公开入口核准水位。
func (s *ActiveStore) readConfigAtWatermark(ctx context.Context, op platform.OperationContext, tx *sql.Tx, siteID string, revision gen.Revision, revisionID string) (gen.SiteConfig, configJournalRecord, error) {
	config, record, _, err := s.readIndexedConfig(ctx, tx, revisionID)
	if err != nil {
		var failure *platform.Error
		if errors.As(err, &failure) && failure.Code == platform.CodeNotFound {
			return config, record, platform.NewError(platform.CodeIntegrityError, false, "配置当前水位对应的索引缺失")
		}
		return config, record, err
	}
	if record.SiteID != siteID || record.Object.Version != revision || record.DataEpoch != op.DataEpoch {
		return config, record, platform.NewError(platform.CodeIntegrityError, false, "配置当前索引与水位不一致")
	}
	head, err := readConfigRecord(configHeadPath(s.activeRoot, siteID))
	if err != nil {
		return config, record, err
	}
	left, _ := canonicalConfigJSON(head)
	right, _ := canonicalConfigJSON(record)
	if string(left) != string(right) {
		return config, record, platform.NewError(platform.CodeIntegrityError, false, "配置head与已完成权威水位不一致")
	}
	return config, record, nil
}

func (s *ActiveStore) readConfigWatermark(ctx context.Context, op platform.OperationContext, tx *sql.Tx) (string, gen.Revision, string, error) {
	var epoch string
	var initialized, revision int
	var siteID, revisionID sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch,initialized,site_id,config_revision,config_revision_id FROM site_config_state WHERE singleton=1`).Scan(&epoch, &initialized, &siteID, &revision, &revisionID); err != nil {
		return "", 0, "", platform.NewError(platform.CodeIntegrityError, false, "站点配置水位不可读")
	}
	if epoch != op.DataEpoch {
		return "", 0, "", platform.NewError(platform.CodeVersionMismatch, false, "配置dataEpoch已变化")
	}
	if initialized != 1 {
		return "", 0, "", platform.NewError(platform.CodeNotFound, false, "站点尚未完成初始化")
	}
	if !siteID.Valid || !revisionID.Valid || revision < 1 {
		return "", 0, "", platform.NewError(platform.CodeIntegrityError, false, "已初始化配置缺少权威引用")
	}
	if op.Actor.Kind != "service" && op.Actor.SiteID != siteID.String {
		return "", 0, "", platform.NewError(platform.CodeForbidden, false, "配置站点与委托主体不一致")
	}
	return siteID.String, revision, revisionID.String, nil
}

func (s *ActiveStore) readConfigOperationResult(ctx context.Context, op platform.OperationContext, requestHash string, revision *gen.Revision) (gen.SiteConfig, error) {
	if revision == nil {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置成功回执缺少原始修订")
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return gen.SiteConfig{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开配置原始结果事务失败")
	}
	defer tx.Rollback()
	siteID, current, _, err := s.readConfigWatermark(ctx, op, tx)
	if err != nil {
		return gen.SiteConfig{}, err
	}
	if current < *revision {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置回执超出已完成水位")
	}
	var revisionID string
	if err := tx.QueryRowContext(ctx, `SELECT revision_id FROM config_revision_index WHERE site_id=? AND revision=?`, siteID, *revision).Scan(&revisionID); err != nil {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置原始结果权威索引缺失")
	}
	config, record, _, err := s.readIndexedConfig(ctx, tx, revisionID)
	if err != nil {
		return config, err
	}
	if record.OperationID != op.OperationID || record.ActorID != op.Actor.SubjectID || record.RequestHash != requestHash || record.DataEpoch != op.DataEpoch {
		return config, platform.NewError(platform.CodeIntegrityError, false, "配置原始结果与操作回执不一致")
	}
	return config, nil
}

// loadPublicHomeConfig 是独立匿名首页入口，不改变私有readConfigWatermark的站点授权。
// 仅本域ReadPublicHome使用返回的内部配置，HTTP结果由明确公开字段投影构造。
func (s *ActiveStore) loadPublicHomeConfig(ctx context.Context, op platform.OperationContext) (gen.SiteConfig, error) {
	if op.CallerService != "portal" && op.CallerService != "public-site" {
		return gen.SiteConfig{}, platform.NewError(platform.CodeForbidden, false, "公开首页只允许登记入口读取")
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return gen.SiteConfig{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开公开首页读取事务失败")
	}
	defer tx.Rollback()
	var epoch string
	var initialized, revision int
	var siteID, revisionID sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch,initialized,site_id,config_revision,config_revision_id FROM site_config_state WHERE singleton=1`).Scan(&epoch, &initialized, &siteID, &revision, &revisionID); err != nil {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "公开首页水位不可读")
	}
	if epoch != op.DataEpoch {
		return gen.SiteConfig{}, platform.NewError(platform.CodeVersionMismatch, false, "公开首页不属于活动dataEpoch")
	}
	if initialized != 1 {
		return gen.SiteConfig{}, platform.NewError(platform.CodeNotFound, false, "站点尚未完成初始化")
	}
	if !siteID.Valid || !revisionID.Valid || revision < 1 {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "公开首页缺少正式配置引用")
	}
	config, _, err := s.readConfigAtWatermark(ctx, op, tx, siteID.String, revision, revisionID.String)
	return config, err
}
