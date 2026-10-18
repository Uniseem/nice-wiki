// Package admin 读取本域管理配置并保存费用策略；不连接供应方或创建费用任务。
package admin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/billing"
	"nice-wiki/modules/model-gateway/internal/catalog"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

type OperationContext = platform.OperationContext
type ReadModelsInput struct {
	Capability *string `json:"capability,omitempty"`
}
type ModelWithSpace struct {
	gen.Model
	ModelSnapshotHash *string                 `json:"modelSnapshotHash,omitempty"`
	EmbeddingSpace    *catalog.EmbeddingSpace `json:"embeddingSpace,omitempty"`
}
type ReadModelsResult struct {
	Models []ModelWithSpace `json:"models"`
}
type ReadProfilesInput struct {
	ProfileId *string `json:"profileId,omitempty"`
}
type ReadProfilesResult = gen.ReadProfilesResult

// ReadModels 返回真实模型声明，停用模型仍保留enabled=false；读取不探测供应方。
func ReadModels(ctx context.Context, op OperationContext, input ReadModelsInput) (ReadModelsResult, error) {
	if err := requireCatalogReader(ctx, op); err != nil {
		return ReadModelsResult{}, err
	}
	tx, err := beginAdminSnapshot(ctx, op)
	if err != nil {
		return ReadModelsResult{}, err
	}
	defer tx.Rollback()
	models, err := readCatalogModels(ctx, op, tx, input.Capability, 256)
	if err != nil {
		return ReadModelsResult{}, err
	}
	result := ReadModelsResult{Models: make([]ModelWithSpace, 0, len(models))}
	for _, model := range models {
		hash, err := gatewayruntime.ModelSnapshotHash(model)
		if err != nil {
			return ReadModelsResult{}, err
		}
		view := ModelWithSpace{Model: model, ModelSnapshotHash: &hash}
		if slices.Contains(model.Capabilities, "embedding") {
			space, err := catalog.ModelEmbeddingSpace(model)
			if err != nil {
				return ReadModelsResult{}, err
			}
			view.EmbeddingSpace = &space
		}
		result.Models = append(result.Models, view)
	}
	return result, nil
}

// ReadProfiles 对纯服务只返回可信费用类别；后台管理读取通过即时model.manage授权。
func ReadProfiles(ctx context.Context, op OperationContext, input ReadProfilesInput) (ReadProfilesResult, error) {
	if err := requireCatalogReader(ctx, op); err != nil {
		return ReadProfilesResult{}, err
	}
	tx, err := beginAdminSnapshot(ctx, op)
	if err != nil {
		return ReadProfilesResult{}, err
	}
	defer tx.Rollback()
	var costClass *string
	if op.Actor.Kind == "service" {
		if op.CostClass != platform.CostClassQA && op.CostClass != platform.CostClassBackground && op.CostClass != platform.CostClassAdminProbe {
			return ReadProfilesResult{}, platform.NewError(platform.CodeForbidden, false, "费用类别尚未通过可信调用链建立")
		}
		costClass = &op.CostClass
	}
	profiles, err := readCatalogProfiles(ctx, op, tx, input.ProfileId, costClass, 256)
	if err != nil {
		return ReadProfilesResult{}, err
	}
	return ReadProfilesResult{Profiles: profiles}, nil
}

func requireCatalogReader(ctx context.Context, op OperationContext) error {
	if op.CallerService == "portal" && op.Actor.Kind == "admin" && (slices.Contains(op.Actor.Grants, "model.manage") || slices.Contains(op.Actor.Grants, "superadmin")) {
		return nil
	}
	if op.Actor.Kind == "service" {
		for _, task := range gen.AITaskCatalog {
			if task.Owner == op.CallerService {
				return nil
			}
		}
	}
	return platform.NewError(platform.CodeForbidden, false, "调用主体不能读取模型配置目录")
}

func beginAdminSnapshot(ctx context.Context, op OperationContext) (*sql.Tx, error) {
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "模型配置存储尚未装配")
	}
	tx, err := store.BeginTx(ctx)
	if err != nil {
		return nil, adminStorageError(err)
	}
	var epoch string
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch FROM runtime_meta WHERE singleton = 1`).Scan(&epoch); err != nil {
		_ = tx.Rollback()
		return nil, adminStorageError(err)
	}
	if epoch == "" {
		_ = tx.Rollback()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "模型配置当前代次尚未知")
	}
	if epoch != op.DataEpoch {
		_ = tx.Rollback()
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "模型配置当前代次已变化")
	}
	return tx, nil
}

func readCatalogModels(ctx context.Context, op OperationContext, tx *sql.Tx, capability *string, limit int) ([]gen.Model, error) {
	rows, err := tx.QueryContext(ctx, `SELECT model_id, version, provider_id, provider_model_version, capabilities_json,
		dimension, preprocessor_version, max_input_tokens, max_output_tokens, catalog_origin, manual_override, enabled, data_epoch, catalog_provider_id, catalog_model_id
		FROM models ORDER BY model_id`)
	if err != nil {
		return nil, adminStorageError(err)
	}
	defer rows.Close()
	result := make([]gen.Model, 0)
	for rows.Next() {
		var model gen.Model
		var capabilities []byte
		var epoch string
		var catalogProvider, catalogModel sql.NullString
		if err := rows.Scan(&model.ModelId, &model.Version, &model.ProviderId, &model.ProviderModelVersion, &capabilities,
			&model.Dimension, &model.PreprocessorVersion, &model.MaxInputTokens, &model.MaxOutputTokens, &model.CatalogOrigin, &model.ManualOverride, &model.Enabled, &epoch, &catalogProvider, &catalogModel); err != nil {
			return nil, adminStorageError(err)
		}
		if epoch != op.DataEpoch {
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "模型目录代次已变化")
		}
		if json.Unmarshal(capabilities, &model.Capabilities) != nil || model.Capabilities == nil || len(model.Capabilities) == 0 || len(model.Capabilities) > 256 || model.Version < 1 || model.Version > 9007199254740991 {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "模型目录持久声明损坏")
		}
		if catalogProvider.Valid != catalogModel.Valid || (catalogProvider.Valid && (!utf8.ValidString(catalogProvider.String) || !utf8.ValidString(catalogModel.String) || catalogProvider.String == "" || catalogModel.String == "" || utf8.RuneCountInString(catalogProvider.String) > 65536 || utf8.RuneCountInString(catalogModel.String) > 65536)) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "公开目录来源身份不完整")
		}
		if catalogProvider.Valid {
			model.CatalogRef = &gen.ModelCatalogRef{ProviderId: catalogProvider.String, ModelId: catalogModel.String}
		}
		if capability != nil && !slices.Contains(model.Capabilities, *capability) {
			continue
		}
		if len(result) == limit {
			return nil, platform.NewError(platform.CodeBudgetExceeded, false, "模型目录超过响应条目上限")
		}
		result = append(result, model)
	}
	if err := rows.Err(); err != nil {
		return nil, adminStorageError(err)
	}
	return result, nil
}

func readCatalogProfiles(ctx context.Context, op OperationContext, tx *sql.Tx, profileID, costClass *string, limit int) ([]gen.Profile, error) {
	query := `SELECT profile_id, version, cost_class, tier, ordered_model_ids_json, maximum_cost_micro_usd, concurrency,
		daily_limit_cost_micro_usd, data_epoch FROM profiles WHERE 1 = 1`
	args := []any{}
	if profileID != nil {
		query += ` AND profile_id = ?`
		args = append(args, *profileID)
	}
	if costClass != nil {
		query += ` AND cost_class = ?`
		args = append(args, *costClass)
	}
	rows, err := tx.QueryContext(ctx, query+` ORDER BY profile_id`, args...)
	if err != nil {
		return nil, adminStorageError(err)
	}
	defer rows.Close()
	result := make([]gen.Profile, 0)
	for rows.Next() {
		var profile gen.Profile
		var ordered []byte
		var epoch string
		if err := rows.Scan(&profile.ProfileId, &profile.Version, &profile.CostClass, &profile.Tier, &ordered, &profile.MaximumCost.Micros,
			&profile.Concurrency, &profile.DailyLimitCost.Micros, &epoch); err != nil {
			return nil, adminStorageError(err)
		}
		if epoch != op.DataEpoch {
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "模型档位代次已变化")
		}
		if json.Unmarshal(ordered, &profile.OrderedModelIds) != nil || profile.OrderedModelIds == nil || len(profile.OrderedModelIds) > 256 || profile.Version < 1 || profile.Version > 9007199254740991 {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "模型档位持久声明损坏")
		}
		profile.MaximumCost.Currency, profile.DailyLimitCost.Currency = "USD", "USD"
		if len(result) == limit {
			return nil, platform.NewError(platform.CodeBudgetExceeded, false, "档位目录超过响应条目上限")
		}
		result = append(result, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, adminStorageError(err)
	}
	return result, nil
}

func adminStorageError(err error) error {
	return platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, true, "模型管理权威存储暂不可读")
}

type ReadAdminCatalogInput struct{}
type ReadAdminCatalogResult = gen.ReadAdminCatalogResult
type FeeBudgetPolicy = gen.FeeBudgetPolicy
type ReadFeeBudgetPolicyInput struct{}
type ReadFeeBudgetPolicyResult = gen.ReadFeeBudgetPolicyResult
type PutFeeBudgetPolicyInput = gen.PutFeeBudgetPolicyInput
type PutFeeBudgetPolicyResult = gen.PutFeeBudgetPolicyResult

// AdminAuthorizer 复用平台identity即时授权，不缓存账户权限或复制身份权威。
type AdminAuthorizer func(context.Context, platform.OperationContext, string, string) error
type feePolicyAuthorizationKey struct{}

const feePolicyFunctionID = "model-gateway.PutFeeBudgetPolicy"

// ReadAdminCatalog 以同一快照读取真实配置与最近诊断的原冻结记录。
func ReadAdminCatalog(ctx context.Context, op OperationContext, input ReadAdminCatalogInput) (ReadAdminCatalogResult, error) {
	if op.CallerService != "portal" || op.Actor.Kind != "admin" {
		return ReadAdminCatalogResult{}, platform.NewError(platform.CodeForbidden, false, "管理目录要求后台账户委托")
	}
	tx, err := beginAdminSnapshot(ctx, op)
	if err != nil {
		return ReadAdminCatalogResult{}, err
	}
	defer tx.Rollback()
	providers, err := readCatalogProviders(ctx, op, tx)
	if err != nil {
		return ReadAdminCatalogResult{}, err
	}
	models, err := readCatalogModels(ctx, op, tx, nil, 1000)
	if err != nil {
		return ReadAdminCatalogResult{}, err
	}
	profiles, err := readCatalogProfiles(ctx, op, tx, nil, nil, 64)
	if err != nil {
		return ReadAdminCatalogResult{}, err
	}
	assignments, err := readCatalogAssignments(ctx, op, tx)
	if err != nil {
		return ReadAdminCatalogResult{}, err
	}
	if err:=ReadLatestProbes(ctx,op,tx,providers);err!=nil{return ReadAdminCatalogResult{},err}
	return ReadAdminCatalogResult{Providers: providers, Models: models, Profiles: profiles, TaskAssignments: assignments}, nil
}

func readCatalogProviders(ctx context.Context, op OperationContext, tx *sql.Tx) ([]gen.ProviderSummary, error) {
	// 私有附加头secret引用、nonce和ciphertext正文均不选入查询。
	rows, err := tx.QueryContext(ctx, `SELECT p.provider_id, p.version, p.adapter, p.base_url, p.enabled,
		p.credential_version, p.connect_timeout_ms, p.request_timeout_ms, p.display_name, p.extra_header_names_json,
		p.concurrency_limit, p.stream_idle_timeout_ms, p.data_epoch, c.data_epoch, length(c.nonce), length(c.ciphertext),r.source_database_hash,r.source_material_hash
		FROM providers p LEFT JOIN credentials c ON c.provider_id = p.provider_id AND c.credential_version = p.credential_version
		LEFT JOIN snapshot_material_redactions r ON r.kind='credential' AND r.record_key=p.provider_id||'|'||p.credential_version
		ORDER BY p.provider_id LIMIT 65`)
	if err != nil {
		return nil, adminStorageError(err)
	}
	defer rows.Close()
	result := make([]gen.ProviderSummary, 0)
	for rows.Next() {
		var provider gen.ProviderSummary
		var names []byte
		var epoch string
		var credentialEpoch,redactionDatabase,redactionMaterial sql.NullString
		var nonceLength, ciphertextLength sql.NullInt64
		if err := rows.Scan(&provider.ProviderId, &provider.Version, &provider.Adapter, &provider.BaseUrl, &provider.Enabled,
			&provider.CredentialVersion, &provider.ConnectTimeoutMs, &provider.RequestTimeoutMs, &provider.DisplayName,
			&names, &provider.ConcurrencyLimit, &provider.StreamIdleTimeoutMs, &epoch, &credentialEpoch, &nonceLength, &ciphertextLength,&redactionDatabase,&redactionMaterial); err != nil {
			return nil, adminStorageError(err)
		}
		if epoch != op.DataEpoch || (credentialEpoch.Valid && credentialEpoch.String != op.DataEpoch) {
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "供应方或凭据代次已变化")
		}
		redacted:=redactionDatabase.Valid&&redactionMaterial.Valid&&platform.ValidHash(redactionDatabase.String)&&platform.ValidHash(redactionMaterial.String)&&nonceLength.Int64==0&&ciphertextLength.Int64==0
 if (redactionDatabase.Valid||redactionMaterial.Valid)&&!redacted{return nil,platform.NewError(platform.CodeIntegrityError,false,"明确脱密材料的来源摘要损坏")}
		if provider.Version < 1 || provider.Version > 9007199254740991 || json.Unmarshal(names, &provider.ExtraHeaderNames) != nil || provider.ExtraHeaderNames == nil || len(provider.ExtraHeaderNames) > 256 || (credentialEpoch.Valid && !redacted && (nonceLength.Int64 != 12 || ciphertextLength.Int64 <= 16)) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "供应方目录或凭据元数据损坏")
		}
		provider.CredentialConfigured = credentialEpoch.Valid&&!redacted
		if len(result) == 64 {
			return nil, platform.NewError(platform.CodeBudgetExceeded, false, "供应方目录超过响应条目上限")
		}
		result = append(result, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, adminStorageError(err)
	}
	return result, nil
}

func readCatalogAssignments(ctx context.Context, op OperationContext, tx *sql.Tx) ([]gen.TaskAssignment, error) {
	rows, err := tx.QueryContext(ctx, `SELECT task_id, tier, override_model_id, version, data_epoch FROM task_assignments ORDER BY task_id LIMIT 257`)
	if err != nil {
		return nil, adminStorageError(err)
	}
	defer rows.Close()
	result := make([]gen.TaskAssignment, 0)
	for rows.Next() {
		var assignment gen.TaskAssignment
		var override sql.NullString
		var epoch string
		if err := rows.Scan(&assignment.TaskId, &assignment.Tier, &override, &assignment.Version, &epoch); err != nil {
			return nil, adminStorageError(err)
		}
		if epoch != op.DataEpoch {
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "任务模型分配代次已变化")
		}
		task, found := gen.AITaskCatalog[assignment.TaskId]
		if !found || string(assignment.Tier) != task.DefaultTier || assignment.Version < 1 || assignment.Version > 9007199254740991 || (override.Valid && !platform.ValidIdentifier(override.String)) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "任务模型分配持久声明损坏")
		}
		if override.Valid {
			assignment.OverrideModelId = &override.String
		}
		if len(result) == 256 {
			return nil, platform.NewError(platform.CodeBudgetExceeded, false, "任务分配超过响应条目上限")
		}
		result = append(result, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, adminStorageError(err)
	}
	return result, nil
}

// ReadFeeBudgetPolicy 只返回已经明确保存的费用策略；缺配置不合成默认美元金额。
func ReadFeeBudgetPolicy(ctx context.Context, op OperationContext, input ReadFeeBudgetPolicyInput) (ReadFeeBudgetPolicyResult, error) {
	if err := requireFeePolicyActor(ctx, op); err != nil {
		return ReadFeeBudgetPolicyResult{}, err
	}
	tx, err := beginAdminSnapshot(ctx, op)
	if err != nil {
		return ReadFeeBudgetPolicyResult{}, err
	}
	defer tx.Rollback()
	policy, err := readFeePolicy(ctx, op, tx)
	if err != nil {
		return ReadFeeBudgetPolicyResult{}, err
	}
	return ReadFeeBudgetPolicyResult{Policy: policy}, nil
}

// PutFeeBudgetPolicy 保存完整原结果、版本CAS和脱敏审计，不修改既有费用记录。
func PutFeeBudgetPolicy(ctx context.Context, op OperationContext, input PutFeeBudgetPolicyInput) (PutFeeBudgetPolicyResult, error) {
	if err := requireFeePolicyActor(ctx, op); err != nil {
		return PutFeeBudgetPolicyResult{}, err
	}
	if input.ExpectedVersion >= 9007199254740991 || input.Policy.Version != input.ExpectedVersion+1 {
		return PutFeeBudgetPolicyResult{}, platform.NewError(platform.CodeVersionMismatch, false, "费用策略版本必须为预期版本加一")
	}
	tx, err := beginAdminSnapshot(ctx, op)
	if err != nil {
		return PutFeeBudgetPolicyResult{}, err
	}
	defer tx.Rollback()
	raw, err := json.Marshal(struct {
		CallerService   string              `json:"callerService"`
		DataEpoch       platform.Identifier `json:"dataEpoch"`
		Policy          FeeBudgetPolicy     `json:"policy"`
		ExpectedVersion int                 `json:"expectedVersion"`
	}{op.CallerService, op.DataEpoch, input.Policy, input.ExpectedVersion})
	if err != nil {
		return PutFeeBudgetPolicyResult{}, platform.NewError(platform.CodeInternal, false, "费用策略摘要编码失败")
	}
	sum := sha256.Sum256(raw)
	inputHash := hex.EncodeToString(sum[:])
	if result, found, err := readFeePolicyReceipt(ctx, op, tx, input, inputHash); err != nil {
		return PutFeeBudgetPolicyResult{}, err
	} else if found {
		return result, nil
	}
	current, err := readFeePolicy(ctx, op, tx)
	var public *platform.Error
	missing := errors.As(err, &public) && public.Code == platform.CodeNotFound
	if err != nil && !missing {
		return PutFeeBudgetPolicyResult{}, err
	}
	if current.Version != input.ExpectedVersion {
		return PutFeeBudgetPolicyResult{}, platform.NewError(platform.CodeVersionMismatch, false, "费用策略版本已变化")
	}
	needsSuperadmin := missing || (current.DailyQaLimit.Micros != input.Policy.DailyQaLimit.Micros||FeeProbeLimit(current)!=FeeProbeLimit(input.Policy))
	// 即时身份RPC不得持有本域事务；写事务重新核原回执、屏障和CAS。
	_ = tx.Rollback()
	if needsSuperadmin {
		authorize, ok := ctx.Value(feePolicyAuthorizationKey{}).(AdminAuthorizer)
		if !ok || authorize == nil {
			return PutFeeBudgetPolicyResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "费用策略当前超管授权尚未装配")
		}
		if err := authorize(ctx, op, "superadmin", "model-gateway:putFeeBudgetPolicy"); err != nil {
			return PutFeeBudgetPolicyResult{}, err
		}
	}
	tx, err = beginAdminSnapshot(ctx, op)
	if err != nil {
		return PutFeeBudgetPolicyResult{}, err
	}
	defer tx.Rollback()
	if result, found, err := readFeePolicyReceipt(ctx, op, tx, input, inputHash); err != nil {
		return PutFeeBudgetPolicyResult{}, err
	} else if found {
		return result, nil
	}
	var blocked bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM barriers WHERE state <> 'released')`).Scan(&blocked); err != nil {
		return PutFeeBudgetPolicyResult{}, adminStorageError(err)
	}
	if blocked {
		return PutFeeBudgetPolicyResult{}, platform.NewError(platform.CodeLocked, false, "费用策略屏障尚未释放")
	}
	current, err = readFeePolicy(ctx, op, tx)
	missing = errors.As(err, &public) && public.Code == platform.CodeNotFound
	if err != nil && !missing {
		return PutFeeBudgetPolicyResult{}, err
	}
	if current.Version != input.ExpectedVersion || (!needsSuperadmin && (missing || (current.DailyQaLimit.Micros != input.Policy.DailyQaLimit.Micros||FeeProbeLimit(current)!=FeeProbeLimit(input.Policy)))) {
		return PutFeeBudgetPolicyResult{}, platform.NewError(platform.CodeVersionMismatch, false, "费用策略或日额度授权条件已变化")
	}
	now := platform.Now()
	var changed sql.Result
	if missing {
		changed, err = tx.ExecContext(ctx, `INSERT INTO fee_budget_policy
			(singleton, site_id, version, hourly_limits_enabled, session_hourly_limit_micro_usd, network_multiplier, daily_qa_limit_micro_usd, daily_probe_limit_micro_usd,data_epoch, updated_at)
			VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(singleton) DO NOTHING`, op.Actor.SiteID, input.Policy.Version,
			input.Policy.HourlyLimitsEnabled, input.Policy.SessionHourlyLimit.Micros, input.Policy.NetworkMultiplier, input.Policy.DailyQaLimit.Micros,FeeProbeLimit(input.Policy), op.DataEpoch, now)
	} else {
		changed, err = tx.ExecContext(ctx, `UPDATE fee_budget_policy SET version = ?, hourly_limits_enabled = ?,
			session_hourly_limit_micro_usd = ?, network_multiplier = ?, daily_qa_limit_micro_usd = ?,daily_probe_limit_micro_usd = ?, updated_at = ?
			WHERE singleton = 1 AND site_id = ? AND version = ? AND data_epoch = ?`, input.Policy.Version,
			input.Policy.HourlyLimitsEnabled, input.Policy.SessionHourlyLimit.Micros, input.Policy.NetworkMultiplier,
			input.Policy.DailyQaLimit.Micros,FeeProbeLimit(input.Policy), now, op.Actor.SiteID, input.ExpectedVersion, op.DataEpoch)
	}
	if err != nil {
		return PutFeeBudgetPolicyResult{}, adminStorageError(err)
	}
	affected, err := changed.RowsAffected()
	if err != nil {
		return PutFeeBudgetPolicyResult{}, adminStorageError(err)
	}
	if affected != 1 {
		return PutFeeBudgetPolicyResult{}, platform.NewError(platform.CodeVersionMismatch, false, "费用策略版本已变化")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO fee_policy_audit
		(site_id, data_epoch, policy_version, actor_id, root_operation_id, created_at, daily_limit_changed)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, op.Actor.SiteID, op.DataEpoch, input.Policy.Version, op.Actor.SubjectID,
		op.RootOperationID, now, missing || (current.DailyQaLimit.Micros != input.Policy.DailyQaLimit.Micros||FeeProbeLimit(current)!=FeeProbeLimit(input.Policy))); err != nil {
		return PutFeeBudgetPolicyResult{}, adminStorageError(err)
	}
	result := PutFeeBudgetPolicyResult{Policy: input.Policy}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return PutFeeBudgetPolicyResult{}, platform.NewError(platform.CodeInternal, false, "费用策略回执编码失败")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM operation_receipts WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`,
		op.Actor.SiteID, op.Actor.SubjectID, feePolicyFunctionID, input.IdempotencyKey); err != nil {
		return PutFeeBudgetPolicyResult{}, adminStorageError(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO operation_receipts (site_id, actor_id, function_id, idempotency_key, input_hash, result_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, op.Actor.SiteID, op.Actor.SubjectID, feePolicyFunctionID, input.IdempotencyKey, inputHash, resultJSON, now); err != nil {
		return PutFeeBudgetPolicyResult{}, adminStorageError(err)
	}
	if err := tx.Commit(); err != nil {
		return PutFeeBudgetPolicyResult{}, adminStorageError(err)
	}
	return result, nil
}

func requireFeePolicyActor(ctx context.Context, op OperationContext) error {
	if op.CallerService != "portal" || op.Actor.Kind != "admin" {
		return platform.NewError(platform.CodeForbidden, false, "费用策略要求后台账户委托")
	}
	return nil
}

func readFeePolicy(ctx context.Context, op OperationContext, tx *sql.Tx) (FeeBudgetPolicy, error) {
	var policy FeeBudgetPolicy
	var siteID, epoch string
 var probe int
	err := tx.QueryRowContext(ctx, `SELECT site_id, version, hourly_limits_enabled, session_hourly_limit_micro_usd,
		network_multiplier, daily_qa_limit_micro_usd,daily_probe_limit_micro_usd, data_epoch FROM fee_budget_policy WHERE singleton = 1`).Scan(
		&siteID, &policy.Version, &policy.HourlyLimitsEnabled, &policy.SessionHourlyLimit.Micros, &policy.NetworkMultiplier, &policy.DailyQaLimit.Micros,&probe, &epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return FeeBudgetPolicy{}, platform.NewError(platform.CodeNotFound, false, "费用策略尚未明确配置")
	}
	if err != nil {
		return FeeBudgetPolicy{}, adminStorageError(err)
	}
	if siteID != op.Actor.SiteID {
		return FeeBudgetPolicy{}, platform.NewError(platform.CodeForbidden, false, "费用策略不属于当前站点")
	}
	if epoch != op.DataEpoch {
		return FeeBudgetPolicy{}, platform.NewError(platform.CodeVersionMismatch, false, "费用策略代次已变化")
	}
	if policy.Version < 1 || policy.Version > 9007199254740991 || policy.NetworkMultiplier < 1 || policy.NetworkMultiplier > 100 || policy.SessionHourlyLimit.Micros < 0 || policy.DailyQaLimit.Micros < 0 {
		return FeeBudgetPolicy{}, platform.NewError(platform.CodeIntegrityError, false, "费用策略持久记录损坏")
	}
	policy.SessionHourlyLimit.Currency, policy.DailyQaLimit.Currency = "USD", "USD"
 policy.DailyProbeLimit=&gen.Money{Currency:"USD",Micros:probe}
	return policy, nil
}

func readFeePolicyReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, input PutFeeBudgetPolicyInput, inputHash string) (PutFeeBudgetPolicyResult, bool, error) {
	var storedHash, createdAt string
	var resultJSON []byte
	err := tx.QueryRowContext(ctx, `SELECT input_hash, result_json, created_at FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`, op.Actor.SiteID,
		op.Actor.SubjectID, feePolicyFunctionID, input.IdempotencyKey).Scan(&storedHash, &resultJSON, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PutFeeBudgetPolicyResult{}, false, nil
	}
	if err != nil {
		return PutFeeBudgetPolicyResult{}, false, adminStorageError(err)
	}
	created, err := platform.ParseTimestamp(createdAt)
	now := time.Now().UTC()
	if err != nil || created.After(now) {
		return PutFeeBudgetPolicyResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "费用策略回执时间损坏")
	}
	if now.Sub(created) > 7*24*time.Hour {
		return PutFeeBudgetPolicyResult{}, false, nil
	}
	if storedHash != inputHash {
		return PutFeeBudgetPolicyResult{}, false, platform.NewError(platform.CodeConflict, false, "幂等键已绑定不同费用策略或代次")
	}
	var result PutFeeBudgetPolicyResult
	if json.Unmarshal(resultJSON, &result) != nil {
		return PutFeeBudgetPolicyResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "费用策略原结果损坏")
	}
	expected, _ := json.Marshal(PutFeeBudgetPolicyResult{Policy: input.Policy})
	if !bytes.Equal(resultJSON, expected) {
		return PutFeeBudgetPolicyResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "费用策略原结果与摘要不一致")
	}
	return result, true, nil
}

type GetPriceBookInput struct {
	ModelId     string `json:"modelId"`
	EffectiveAt string `json:"effectiveAt"`
}
type GetPriceBookResult = gen.GetPriceBookResult

// GetPriceBook 只读取不晚于指定时刻的真实不可变价目，不触发报价或模型。
func GetPriceBook(ctx context.Context, op OperationContext, input GetPriceBookInput) (GetPriceBookResult, error) {
	if op.CallerService != "portal" || op.Actor.Kind != "admin" {
		return GetPriceBookResult{}, platform.NewError(platform.CodeForbidden, false, "价目管理读取要求当前后台账户")
	}
	effectiveAt, err := platform.ParseTimestamp(input.EffectiveAt)
	if err != nil {
		return GetPriceBookResult{}, platform.NewError(platform.CodeValidation, false, "价目读取时刻不可用")
	}
	snapshot, err := billing.ReadPriceSnapshot(ctx, op, input.ModelId, effectiveAt, false)
	if err != nil {
		return GetPriceBookResult{}, err
	}
	return GetPriceBookResult{PriceSnapshot: snapshot}, nil
}

// NewAdminHandlers 绑定真实HTTP输入与平台已验证上下文；费用权限只在入口核一次。
func NewAdminHandlers(authorize AdminAuthorizer) map[string]http.Handler {
	handlers := make(map[string]http.Handler)
	for _, operationID := range []string{"readModels", "readProfiles", "readAdminCatalog", "readFeeBudgetPolicy", "putFeeBudgetPolicy", "getPriceBook","createConnectionProbe"} {
		handlers[operationID] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			if (operationID == "readModels" || operationID == "readProfiles") && op.Actor.Kind == "admin" {
				if authorize == nil {
					_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeDependencyUnavailable, true, "模型配置当前授权尚未装配"), op.RequestID))
					return
				}
				if err := authorize(r.Context(), op, "model.manage", "model-gateway:"+operationID); err != nil {
					_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
					return
				}
			}
			var result any
			var err error
			switch operationID {
   case "createConnectionProbe":
    var input gen.CreateConnectionProbeInput
    if json.NewDecoder(r.Body).Decode(&input)!=nil{err=platform.NewError(platform.CodeValidation,false,"诊断输入不可读")}else if input.ProviderId!=r.PathValue("providerId"){err=platform.NewError(platform.CodeValidation,false,"诊断渠道路径不一致")}else if key:=r.Header.Get("Idempotency-Key");key!=""&&key!=input.IdempotencyKey{err=platform.NewError(platform.CodeValidation,false,"诊断幂等头不一致")}else{result,err=CreateConnectionProbe(r.Context(),op,input)}
			case "getPriceBook":
				result, err = GetPriceBook(r.Context(), op, GetPriceBookInput{ModelId: r.PathValue("modelId"), EffectiveAt: r.URL.Query().Get("effectiveAt")})
			case "readModels":
				var input ReadModelsInput
				if query := r.URL.Query(); query.Has("capability") {
					value := query.Get("capability")
					input.Capability = &value
				}
				result, err = ReadModels(r.Context(), op, input)
			case "readProfiles":
				var input ReadProfilesInput
				if query := r.URL.Query(); query.Has("profileId") {
					value := query.Get("profileId")
					input.ProfileId = &value
				}
				result, err = ReadProfiles(r.Context(), op, input)
			case "readAdminCatalog":
				result, err = ReadAdminCatalog(r.Context(), op, ReadAdminCatalogInput{})
			case "readFeeBudgetPolicy":
				result, err = ReadFeeBudgetPolicy(r.Context(), op, ReadFeeBudgetPolicyInput{})
			case "putFeeBudgetPolicy":
				var input PutFeeBudgetPolicyInput
				if decodeErr := json.NewDecoder(r.Body).Decode(&input); decodeErr != nil {
					err = platform.NewError(platform.CodeValidation, false, "费用策略输入不可读")
				} else if key := r.Header.Get("Idempotency-Key"); key != "" && key != input.IdempotencyKey {
					err = platform.NewError(platform.CodeValidation, false, "费用策略幂等头与正文不一致")
				} else {
					ctx := context.WithValue(r.Context(), feePolicyAuthorizationKey{}, authorize)
					result, err = PutFeeBudgetPolicy(ctx, op, input)
				}
			}
			if err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_ = json.NewEncoder(w).Encode(result)
		})
	}
	return handlers
}

// GetLedger 读取固定水位下的真实费用事实与调用原价目，不写费用状态。
func GetLedger(ctx context.Context, op OperationContext, input GetLedgerInput) (GetLedgerResult, error) {
	return readLedgerPage(ctx, op, input)
}
