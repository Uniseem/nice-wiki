package siteconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform/problem"
	"slices"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/pgdb"
	"nice-wiki/modules/site-config/internal/gen"
)

type ReadInitializationStatusInput struct{}

// ReadInitializationStatus 返回匿名可安全读取的阶段；私有协调标识只
// 附给identity/site-config或同站点超管，不能靠读取状态重新开放注册。
func ReadInitializationStatus(ctx context.Context, op platform.OperationContext, input ReadInitializationStatusInput) (gen.ReadInitializationStatusResult, error) {
	store := currentActiveStore()
	if store == nil {
		return gen.ReadInitializationStatusResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	tx, err := store.db.BeginTx(store.scopeSite(ctx, op), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return gen.ReadInitializationStatusResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开初始化读取事务失败")
	}
	defer tx.Rollback()
	state, initialized, err := store.ReadInitializationRecord(ctx, tx)
	if err != nil {
		return gen.ReadInitializationStatusResult{}, err
	}
	result := gen.ReadInitializationStatusResult{
		Initialized: initialized,
		State: gen.PublicInitializationState{
			Phase:     gen.PublicInitializationStatePhase(state.Phase),
			SafeError: state.SafeError,
		},
	}
	trustedService := op.CallerService == "identity" || op.CallerService == "site-config"
	sameSiteAdmin := op.Actor.Kind == "admin" && (slices.Contains(op.Actor.Grants, "superadmin") || slices.Contains(op.Actor.Grants, "site.manage")) && op.Actor.SiteID != "" && (state.SiteId == nil || op.Actor.SiteID == *state.SiteId)
	if trustedService || sameSiteAdmin {
		result.Coordination = &state
	}
	return result, nil
}

// scopeSite 把不带站点的服务调用（启动绑定、单站点时代的读取）定位到平台自己的站点：最早提交初始化的那个；
// 还没有任何站点完成初始化时只看模板行（即“未初始化”）。
func (s *ActiveStore) scopeSite(ctx context.Context, op platform.OperationContext) context.Context {
	if op.Actor.SiteID != "" && op.Actor.SiteID != "system" {
		return ctx
	}
	if site, ok := s.platformSite.Load().(string); ok {
		return pgdb.WithSite(ctx, site)
	}
	var site string
	err := s.db.QueryRowContext(pgdb.WithSite(ctx, ""), `SELECT site_id FROM initialization_decisions WHERE decision='commit' ORDER BY created_at, operation_id LIMIT 1`).Scan(&site)
	if err != nil {
		return pgdb.WithTemplateView(pgdb.WithSite(ctx, ""))
	}
	s.platformSite.Store(site)
	return pgdb.WithSite(ctx, site)
}

// ConfigureInitialization 仅接收进程配置边界已校验的部署公开地址。
// 空地址允许既有读取启动，新的初始化在使用时失败关闭。
func (s *ActiveStore) ConfigureInitialization(publicBaseURL string, allowLocalHTTP bool) {
	s.stateMu.Lock()
	s.publicBaseURL = publicBaseURL
	s.allowLocalHTTP = allowLocalHTTP
	s.stateMu.Unlock()
}

// InitializeSite 以唯一超管的已验证会话和siteId启动无AI初始化。
func InitializeSite(ctx context.Context, op platform.OperationContext, input gen.InitializeSiteInput) (gen.InitializeSiteResult, error) {
	if err := requireInitializationAdmin(op); err != nil {
		return gen.InitializeSiteResult{}, err
	}
	store := currentActiveStore()
	if store == nil {
		return gen.InitializeSiteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	store.initializationMu.Lock()
	defer store.initializationMu.Unlock()
	language, err := ValidateLanguageTag(ValidateLanguageTagInput{Language: input.BaseLanguage})
	if err != nil {
		return gen.InitializeSiteResult{}, err
	}
	if _, ok := platform.FindSiteLanguage(language.Canonical); !ok {
		return gen.InitializeSiteResult{}, platform.NewError(platform.CodeValidation, false, "基准语言只能从预设的站点语言里选")
	}
	requestHash, err := initializationRequestHash(input.Name, language.Canonical)
	if err != nil {
		return gen.InitializeSiteResult{}, err
	}
	receipt, err := ResolveConfigOperation(ctx, op, ResolveConfigOperationInput{OperationId: op.OperationID, RequestHash: requestHash})
	if err != nil {
		return gen.InitializeSiteResult{}, err
	}
	state, initialized, err := store.readCurrentInitialization(ctx)
	if err != nil {
		return gen.InitializeSiteResult{}, err
	}
	if receipt.State != "absent" {
		if state.OperationId == nil || *state.OperationId != op.OperationID || state.Decision != gen.Commit {
			return gen.InitializeSiteResult{}, platform.NewError(platform.CodeConflict, false, "此operationId不是可继续的初始化")
		}
		resumed, err := store.resumeInitializationLocked(ctx, op, op.OperationID)
		return gen.InitializeSiteResult{State: resumed}, err
	}
	if initialized || state.Decision == gen.Commit {
		return gen.InitializeSiteResult{}, platform.NewError(platform.CodeConflict, false, "站点已有初始化commit决定")
	}
	store.stateMu.RLock()
	publicBaseURL, allowLocalHTTP := store.publicBaseURL, store.allowLocalHTTP
	store.stateMu.RUnlock()
	if input.PublicBaseURL != nil && *input.PublicBaseURL != "" {
		normalized, err := NormalizeBaseURL(NormalizeBaseURLInput{URL: *input.PublicBaseURL, AllowLocalHTTP: allowLocalHTTP})
		if err != nil {
			return gen.InitializeSiteResult{}, err
		}
		publicBaseURL = normalized.Normalized
	}
	if publicBaseURL == "" {
		return gen.InitializeSiteResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "初始化尚未配置可信公开地址")
	}
	prepared, err := PrepareDefaultConfig(PrepareDefaultConfigInput{SiteID: op.Actor.SiteID, Name: input.Name, BaseLanguage: language.Canonical, PublicBaseURL: publicBaseURL, PreparedAt: time.Now().UTC()})
	if err != nil {
		return gen.InitializeSiteResult{}, err
	}
	operationID, siteID := op.OperationID, op.Actor.SiteID
	preparing := gen.InitializationState{OperationId: &operationID, SiteId: &siteID, Phase: gen.InitializationStatePhasePreparing,
		Decision: gen.None, Revision: state.Revision, UpdatedAt: prepared.Config.UpdatedAt}
	if _, err := PersistInitializationDecision(ctx, op, PersistInitializationDecisionInput{State: preparing, Decision: "commit", ExpectedRevision: state.Revision, PreparedConfig: &prepared.Config, RequestHash: requestHash}); err != nil {
		return gen.InitializeSiteResult{}, err
	}
	resumed, err := store.resumeInitializationLocked(ctx, op, op.OperationID)
	return gen.InitializeSiteResult{State: resumed}, err
}

// ResumeInitialization 只接受同站点管理权限或本域受控启动；原始业务
// 摘要和准备时钟均从持久决定读取，不把恢复请求当成新初始化。
func ResumeInitialization(ctx context.Context, op platform.OperationContext, input gen.ResumeInitializationInput) (gen.ResumeInitializationResult, error) {
	store := currentActiveStore()
	if store == nil {
		return gen.ResumeInitializationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	store.initializationMu.Lock()
	defer store.initializationMu.Unlock()
	state, err := store.resumeInitializationLocked(ctx, op, input.OperationId)
	return gen.ResumeInitializationResult{State: state}, err
}

func (s *ActiveStore) resumeInitializationLocked(ctx context.Context, op platform.OperationContext, operationID string) (result gen.InitializationState, resultErr error) {
	state, _, err := s.readCurrentInitialization(ctx)
	if err != nil {
		return state, err
	}
	if state.OperationId == nil || *state.OperationId != operationID || state.SiteId == nil || state.Decision != gen.Commit {
		return state, platform.NewError(platform.CodeConflict, false, "没有可恢复的初始化commit决定")
	}
	startup := op.CallerService == "site-config" && op.Actor.Kind == "service"
	allowed := op.Actor.Kind == "admin" && op.Actor.SessionID != "" && op.Actor.SiteID == *state.SiteId && (slices.Contains(op.Actor.Grants, "superadmin") || slices.Contains(op.Actor.Grants, "site.manage"))
	if !startup && !allowed {
		return state, platform.NewError(platform.CodeForbidden, false, "没有恢复该站点初始化的权限")
	}
	defer func() {
		if resultErr != nil {
			s.blockInitialization(ctx, operationID, resultErr)
		}
	}()
	var configRaw, hash string
	if err := s.db.QueryRowContext(ctx, `SELECT prepared_config_json,prepared_config_hash FROM initialization_decisions WHERE operation_id=? AND decision='commit'`, operationID).Scan(&configRaw, &hash); err != nil {
		return state, platform.NewError(platform.CodeIntegrityError, false, "初始化完整准备材料缺失")
	}
	if platform.HashBytes([]byte(configRaw)) != hash {
		return state, platform.NewError(platform.CodeIntegrityError, false, "初始化准备材料hash不一致")
	}
	var prepared gen.SiteConfig
	if err := decodeConfigJSON([]byte(configRaw), &prepared); err != nil {
		return state, err
	}
	originalHash, err := initializationRequestHash(prepared.Name, prepared.BaseLanguage)
	if err != nil {
		return state, err
	}
	var actorID, siteID, epoch, requestHash, revisionID string
	err = s.db.QueryRowContext(ctx, `SELECT actor_id,site_id,data_epoch,request_hash,revision_id FROM config_operations WHERE operation_id=?`, operationID).Scan(&actorID, &siteID, &epoch, &requestHash, &revisionID)
	if err != nil || siteID != *state.SiteId || requestHash != originalHash {
		return state, platform.NewError(platform.CodeIntegrityError, false, "初始化原始业务请求回执不一致")
	}
	if epoch != op.DataEpoch {
		return state, platform.NewError(platform.CodeVersionMismatch, false, "初始化属于其他dataEpoch")
	}
	if state.Phase == gen.InitializationStatePhaseCompleted {
		if err := s.verifyConfigHead(ctx); err != nil {
			return state, err
		}
		return state, nil
	}
	originalOp := op
	originalOp.OperationID = operationID
	originalOp.Actor.SubjectID, originalOp.Actor.SiteID = actorID, siteID
	config := prepared
	config.Initialized, config.Revision, config.Home.Revision = true, 1, 1
	written, err := WriteConfigRevision(ctx, originalOp, WriteConfigRevisionInput{Config: config, RevisionId: revisionID})
	if err != nil {
		return state, err
	}
	if _, err := CommitConfigHead(ctx, originalOp, CommitConfigHeadInput{Object: written.Object, ExpectedRevision: 0, RequestHash: requestHash, Initialization: true,
		ChangedSections: []string{"identity", "home", "seo", "security", "limits", "language"}}); err != nil {
		return state, err
	}
	return s.completeInitialization(ctx, operationID, epoch)
}

// AbortInitialization 不撤销commit、不删除身份账号、不重新开放注册。
func AbortInitialization(ctx context.Context, op platform.OperationContext, input gen.AbortInitializationInput) (gen.AbortInitializationResult, error) {
	if err := requireInitializationAdmin(op); err != nil {
		return gen.AbortInitializationResult{}, err
	}
	store := currentActiveStore()
	if store == nil {
		return gen.AbortInitializationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	store.initializationMu.Lock()
	defer store.initializationMu.Unlock()
	requestHash, err := hashInput(struct {
		Kind             string       `json:"kind"`
		OperationID      string       `json:"operationId"`
		ExpectedRevision gen.Revision `json:"expectedRevision"`
	}{"abort", input.OperationId, input.ExpectedRevision})
	if err != nil {
		return gen.AbortInitializationResult{}, err
	}
	receipt, err := ResolveConfigOperation(ctx, op, ResolveConfigOperationInput{OperationId: input.OperationId, RequestHash: requestHash})
	if err != nil {
		return gen.AbortInitializationResult{}, err
	}
	if receipt.State == "committed" {
		var raw string
		if err := store.db.QueryRowContext(ctx, `SELECT result_state_json FROM config_operations WHERE operation_id=?`, input.OperationId).Scan(&raw); err != nil {
			return gen.AbortInitializationResult{}, platform.NewError(platform.CodeIntegrityError, false, "取消初始化原始结果缺失")
		}
		state, err := decodeInitializationState(raw)
		return gen.AbortInitializationResult{State: state}, err
	}
	state, _, err := store.readCurrentInitialization(ctx)
	if err != nil {
		return gen.AbortInitializationResult{}, err
	}
	if state.Decision == gen.Commit {
		return gen.AbortInitializationResult{}, platform.NewError(platform.CodeConflict, false, "初始化commit决定不可撤销，须恢复完成")
	}
	operationID, siteID := input.OperationId, op.Actor.SiteID
	preparing := gen.InitializationState{OperationId: &operationID, SiteId: &siteID, Phase: gen.InitializationStatePhasePreparing, Decision: gen.None, Revision: input.ExpectedRevision, UpdatedAt: time.Now().UTC()}
	decision, err := PersistInitializationDecision(ctx, op, PersistInitializationDecisionInput{State: preparing, Decision: "abort", ExpectedRevision: input.ExpectedRevision, RequestHash: requestHash})
	if err != nil {
		return gen.AbortInitializationResult{}, err
	}
	return gen.AbortInitializationResult{State: decision.State}, nil
}

func requireInitializationAdmin(op platform.OperationContext) error {
	if op.Actor.Kind != "admin" || op.Actor.SessionID == "" || op.Actor.SubjectID == "" || op.Actor.SiteID == "" || !(slices.Contains(op.Actor.Grants, "superadmin") || slices.Contains(op.Actor.Grants, "site.manage")) {
		return platform.NewError(platform.CodeForbidden, false, "初始化要求站点所有者或超管会话")
	}
	return nil
}

func initializationRequestHash(name, language string) (string, error) {
	return hashInput(struct {
		Kind         string `json:"kind"`
		Name         string `json:"name"`
		BaseLanguage string `json:"baseLanguage"`
	}{"initialize", name, language})
}
func configRevisionID(epoch, siteID, operationID string) string {
	return platform.HashBytes([]byte(epoch + "\x00" + siteID + "\x00" + operationID))
}

func (s *ActiveStore) readCurrentInitialization(ctx context.Context) (gen.InitializationState, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return gen.InitializationState{}, false, platform.NewError(platform.CodeDependencyUnavailable, true, "打开初始化读取事务失败")
	}
	defer tx.Rollback()
	return s.ReadInitializationRecord(ctx, tx)
}

func (s *ActiveStore) completeInitialization(ctx context.Context, operationID, epoch string) (gen.InitializationState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return gen.InitializationState{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开初始化完成事务失败")
	}
	defer tx.Rollback()
	state, _, err := s.ReadInitializationRecord(ctx, tx)
	if err != nil {
		return state, err
	}
	if state.OperationId == nil || *state.OperationId != operationID || state.Decision != gen.Commit || (state.Phase != gen.InitializationStatePhaseConfigCommitted && state.Phase != gen.InitializationStatePhaseBlocked) || state.ConfigRevision != 1 {
		return state, platform.NewError(platform.CodeIntegrityError, false, "配置权威文件尚未完成提交")
	}
	state.Phase, state.SafeError, state.Revision, state.UpdatedAt = gen.InitializationStatePhaseCompleted, nil, state.Revision+1, time.Now().UTC()
	raw, err := json.Marshal(state)
	if err != nil {
		return state, platform.NewError(platform.CodeIntegrityError, false, "编码初始化完成状态失败")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE initialization_state SET revision=?,state_json=? WHERE singleton=1 AND operation_id=?`, state.Revision, string(raw), operationID); err != nil {
		return state, platform.NewError(platform.CodeDependencyUnavailable, true, "保存初始化完成状态失败")
	}
	result, err := tx.ExecContext(ctx, `UPDATE site_config_state SET initialized=1 WHERE singleton=1 AND data_epoch=? AND config_revision=1 AND initialized=0`, epoch)
	if err != nil {
		return state, platform.NewError(platform.CodeDependencyUnavailable, true, "保存初始化完成水位失败")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return state, platform.NewError(platform.CodeVersionMismatch, false, "初始化完成CAS失败")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE config_operations SET result_state_json=? WHERE operation_id=?`, string(raw), operationID); err != nil {
		return state, platform.NewError(platform.CodeDependencyUnavailable, true, "保存初始化原始结果失败")
	}
	if err := tx.Commit(); err != nil {
		return state, platform.NewError(platform.CodeDependencyUnavailable, true, "提交初始化完成状态失败")
	}
	return state, nil
}

func (s *ActiveStore) blockInitialization(ctx context.Context, operationID string, cause error) {
	// 已取消的请求不脱离ctx继续写；下次同operation恢复会再次核对决定。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	state, initialized, err := s.ReadInitializationRecord(ctx, tx)
	if err != nil || initialized || state.OperationId == nil || *state.OperationId != operationID {
		return
	}
	safe := problem.FromError(cause, operationID)
	state.Phase, state.Revision, state.UpdatedAt = gen.InitializationStatePhaseBlocked, state.Revision+1, time.Now().UTC()
	state.SafeError = &gen.Problem{Type: safe.Type, Title: safe.Title, Status: safe.Status, Code: gen.ErrorCode(safe.Code), Detail: safe.Detail, RequestId: safe.RequestID, Retryable: safe.Retryable}
	raw, err := json.Marshal(state)
	if err != nil {
		return
	}
	if _, err := tx.ExecContext(ctx, `UPDATE initialization_state SET revision=?,state_json=? WHERE singleton=1 AND operation_id=?`, state.Revision, string(raw), operationID); err != nil {
		return
	}
	_ = tx.Commit()
}
