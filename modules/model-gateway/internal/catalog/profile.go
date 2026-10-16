package catalog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

type Profile = gen.Profile
type PutProfileInput = gen.PutProfileInput
type PutProfileResult = gen.PutProfileResult

// 这些内部类型不增加HTTP入口，调用者不能自由指定任务档位或能力。
type ResolveProfileInput struct {
	TaskId    string `json:"taskId"`
	CostClass string `json:"costClass"`
}

type ResolveRouteInput = ResolveProfileInput

type ResolveProfileResult struct {
	Profile              Profile        `json:"profile"`
	Assignment           TaskAssignment `json:"assignment"`
	OrderedModelIds      []string       `json:"orderedModelIds"`
	RequiredCapabilities []string       `json:"requiredCapabilities"`
	Fallback             bool           `json:"fallback"`
}

type RouteCandidate struct {
	Model             Model `json:"model"`
	ProviderVersion   int   `json:"providerVersion"`
	CredentialVersion int   `json:"credentialVersion"`
}

type ResolveRouteResult struct {
	Resolved     ResolveProfileResult `json:"resolved"`
	Candidates   []RouteCandidate     `json:"candidates"`
	RouteVersion platform.Hash        `json:"routeVersion"`
}

const profileFunctionID = "model-gateway.PutProfile"

// PutProfile 保存有序档位与费用配置，不调用供应方或重算既有付费结果。
func PutProfile(ctx context.Context, op OperationContext, input PutProfileInput) (PutProfileResult, error) {
	if op.Actor.Kind != "admin" || !hasGrant(op.Actor.Grants, "superadmin") {
		return PutProfileResult{}, platform.NewError(platform.CodeForbidden, false, "仅超管可修改模型档位")
	}
	if input.Profile.Version != input.ExpectedVersion+1 {
		return PutProfileResult{}, platform.NewError(platform.CodeVersionMismatch, false, "档位版本必须为预期版本加一")
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return PutProfileResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型配置存储尚未装配")
	}
	snapshot, err := beginAvailabilitySnapshot(ctx, op, store)
	if err != nil {
		return PutProfileResult{}, err
	}
	defer snapshot.tx.Rollback()
	inputHash, err := hashJSON(struct {
		CallerService   string              `json:"callerService"`
		DataEpoch       platform.Identifier `json:"dataEpoch"`
		Profile         Profile             `json:"profile"`
		ExpectedVersion int                 `json:"expectedVersion"`
	}{op.CallerService, op.DataEpoch, input.Profile, input.ExpectedVersion})
	if err != nil {
		return PutProfileResult{}, platform.NewError(platform.CodeInternal, false, "档位配置摘要编码失败")
	}
	if result, found, err := readProfileReceipt(ctx, op, snapshot, input, inputHash); err != nil {
		return PutProfileResult{}, err
	} else if found {
		return result, nil
	}
	var blocked bool
	if err := snapshot.tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM barriers WHERE state <> 'released')`).Scan(&blocked); err != nil {
		return PutProfileResult{}, availabilityReadError(err)
	}
	if blocked {
		return PutProfileResult{}, platform.NewError(platform.CodeLocked, false, "模型配置屏障尚未释放")
	}

	var currentVersion int
	var currentEpoch, previousTier string
	err = snapshot.tx.QueryRowContext(ctx, `SELECT version, data_epoch, tier FROM profiles WHERE profile_id = ?`, input.Profile.ProfileId).
		Scan(&currentVersion, &currentEpoch, &previousTier)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return PutProfileResult{}, availabilityReadError(err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		if input.ExpectedVersion != 0 {
			return PutProfileResult{}, platform.NewError(platform.CodeNotFound, false, "档位配置尚未创建")
		}
	} else if currentEpoch != op.DataEpoch || currentVersion != input.ExpectedVersion {
		return PutProfileResult{}, platform.NewError(platform.CodeVersionMismatch, false, "档位版本或代次已变化")
	}
	var occupied bool
	if err := snapshot.tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM profiles
		WHERE data_epoch = ? AND cost_class = ? AND tier = ? AND profile_id <> ?)`,
		op.DataEpoch, input.Profile.CostClass, input.Profile.Tier, input.Profile.ProfileId).Scan(&occupied); err != nil {
		return PutProfileResult{}, availabilityReadError(err)
	}
	if occupied {
		return PutProfileResult{}, platform.NewError(platform.CodeConflict, false, "该费用类别的档位已由其他配置占用")
	}
	if err := validateProfileModels(ctx, snapshot, input.Profile); err != nil {
		return PutProfileResult{}, err
	}
	orderedJSON, err := json.Marshal(input.Profile.OrderedModelIds)
	if err != nil {
		return PutProfileResult{}, platform.NewError(platform.CodeInternal, false, "档位候选编码失败")
	}
	now := platform.Now()
	var changed sql.Result
	if input.ExpectedVersion == 0 {
		changed, err = snapshot.tx.ExecContext(ctx, `INSERT INTO profiles
			(profile_id, version, cost_class, tier, ordered_model_ids_json, maximum_cost_micro_usd, concurrency, daily_limit_cost_micro_usd, data_epoch, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
			input.Profile.ProfileId, input.Profile.Version, input.Profile.CostClass, input.Profile.Tier, orderedJSON,
			input.Profile.MaximumCost.Micros, input.Profile.Concurrency, input.Profile.DailyLimitCost.Micros, op.DataEpoch, now)
	} else {
		changed, err = snapshot.tx.ExecContext(ctx, `UPDATE profiles SET version = ?, cost_class = ?, tier = ?, ordered_model_ids_json = ?,
			maximum_cost_micro_usd = ?, concurrency = ?, daily_limit_cost_micro_usd = ?, updated_at = ?
			WHERE profile_id = ? AND version = ? AND data_epoch = ?`,
			input.Profile.Version, input.Profile.CostClass, input.Profile.Tier, orderedJSON, input.Profile.MaximumCost.Micros,
			input.Profile.Concurrency, input.Profile.DailyLimitCost.Micros, now, input.Profile.ProfileId, input.ExpectedVersion, op.DataEpoch)
	}
	if err != nil {
		return PutProfileResult{}, availabilityReadError(err)
	}
	affected, err := changed.RowsAffected()
	if err != nil {
		return PutProfileResult{}, availabilityReadError(err)
	}
	if affected != 1 {
		return PutProfileResult{}, platform.NewError(platform.CodeVersionMismatch, false, "档位版本已变化")
	}
	taskIDs := make([]string, 0)
	fastChanged := previousTier == "fast" || input.Profile.Tier == "fast"
	for taskID, task := range gen.AITaskCatalog {
		if task.DefaultTier == previousTier || task.DefaultTier == string(input.Profile.Tier) ||
			(fastChanged && (task.DefaultTier == "vision" || task.DefaultTier == "rerank")) {
			taskIDs = append(taskIDs, taskID)
		}
	}
	sort.Strings(taskIDs)
	payload, err := json.Marshal(struct {
		ProfileID string              `json:"profileId"`
		Version   int                 `json:"version"`
		TaskIDs   []string            `json:"taskIds"`
		DataEpoch platform.Identifier `json:"dataEpoch"`
	}{input.Profile.ProfileId, input.Profile.Version, taskIDs, op.DataEpoch})
	if err != nil {
		return PutProfileResult{}, platform.NewError(platform.CodeInternal, false, "档位变更事件编码失败")
	}
	if err := outbox.EnqueueEvent(snapshot.tx, platform.EventEnvelope{
		EventID: platform.NewIdentifier(), EventType: "profile.changed", EventVersion: 1,
		Producer: "model-gateway", SubjectID: input.Profile.ProfileId, Revision: platform.Revision(input.Profile.Version),
		OccurredAt: now, DataEpoch: op.DataEpoch, Payload: payload,
	}); err != nil {
		return PutProfileResult{}, err
	}
	result := PutProfileResult{Profile: input.Profile}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return PutProfileResult{}, platform.NewError(platform.CodeInternal, false, "档位原结果回执编码失败")
	}
	// 有效回执已原样返回，过期键只随成功的新配置事务一起替换。
	if _, err := snapshot.tx.ExecContext(ctx, `DELETE FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`,
		op.Actor.SiteID, op.Actor.SubjectID, profileFunctionID, input.IdempotencyKey); err != nil {
		return PutProfileResult{}, availabilityReadError(err)
	}
	if _, err := snapshot.tx.ExecContext(ctx, `INSERT INTO operation_receipts
		(site_id, actor_id, function_id, idempotency_key, input_hash, result_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		op.Actor.SiteID, op.Actor.SubjectID, profileFunctionID, input.IdempotencyKey, inputHash, resultJSON, now); err != nil {
		return PutProfileResult{}, availabilityReadError(err)
	}
	if err := snapshot.tx.Commit(); err != nil {
		return PutProfileResult{}, availabilityReadError(err)
	}
	return result, nil
}

func readProfileReceipt(ctx context.Context, op OperationContext, snapshot *availabilitySnapshot, input PutProfileInput, inputHash platform.Hash) (PutProfileResult, bool, error) {
	var storedHash, createdAt string
	var resultJSON []byte
	err := snapshot.tx.QueryRowContext(ctx, `SELECT input_hash, result_json, created_at FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`,
		op.Actor.SiteID, op.Actor.SubjectID, profileFunctionID, input.IdempotencyKey).Scan(&storedHash, &resultJSON, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PutProfileResult{}, false, nil
	}
	if err != nil {
		return PutProfileResult{}, false, availabilityReadError(err)
	}
	created, err := platform.ParseTimestamp(createdAt)
	now := time.Now().UTC()
	if err != nil || created.After(now) {
		return PutProfileResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "档位回执时间损坏")
	}
	if now.Sub(created) > 7*24*time.Hour {
		return PutProfileResult{}, false, nil
	}
	if storedHash != inputHash {
		return PutProfileResult{}, false, platform.NewError(platform.CodeConflict, false, "幂等键已绑定不同档位配置或代次")
	}
	var result PutProfileResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return PutProfileResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "档位原始回执损坏")
	}
	storedResult, _ := json.Marshal(result)
	expectedResult, _ := json.Marshal(PutProfileResult{Profile: input.Profile})
	if !bytes.Equal(storedResult, expectedResult) {
		return PutProfileResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "档位原始结果与摘要不一致")
	}
	return result, true, nil
}

func validateProfileModels(ctx context.Context, snapshot *availabilitySnapshot, profile Profile) error {
	requiredCapabilities := gen.AITierCapabilities[string(profile.Tier)]
	for _, modelID := range profile.OrderedModelIds {
		if err := validateTaskOverride(ctx, snapshot, modelID, requiredCapabilities); err != nil {
			return err
		}
	}
	return nil
}

// ResolveProfile 在调用者的同代快照内解析真实配置，不保证供应方或费用可用。
func ResolveProfile(ctx context.Context, op OperationContext, snapshot *availabilitySnapshot, input ResolveProfileInput) (ResolveProfileResult, error) {
	task, ok := gen.AITaskCatalog[input.TaskId]
	if !ok {
		return ResolveProfileResult{}, platform.NewError(platform.CodeValidation, false, "任务未在目录登记")
	}
	if task.Owner != op.CallerService || input.CostClass != op.CostClass {
		return ResolveProfileResult{}, platform.NewError(platform.CodeForbidden, false, "任务拥有者或费用类别不符")
	}
	var result ResolveProfileResult
	var override sql.NullString
	var epoch string
	err := snapshot.tx.QueryRowContext(ctx, `SELECT task_id, tier, override_model_id, version, data_epoch
		FROM task_assignments WHERE task_id = ?`, input.TaskId).Scan(&result.Assignment.TaskId,
		&result.Assignment.Tier, &override, &result.Assignment.Version, &epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return ResolveProfileResult{}, platform.NewError(platform.CodeNotFound, false, "任务模型分配尚未配置")
	}
	if err != nil {
		return ResolveProfileResult{}, availabilityReadError(err)
	}
	if epoch != snapshot.epoch {
		return ResolveProfileResult{}, platform.NewError(platform.CodeVersionMismatch, false, "任务模型分配代次已变化")
	}
	if string(result.Assignment.Tier) != task.DefaultTier || result.Assignment.Version < 1 ||
		result.Assignment.Version > 9007199254740991 || (override.Valid && !platform.ValidIdentifier(override.String)) {
		return ResolveProfileResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务模型分配损坏")
	}
	if override.Valid {
		result.Assignment.OverrideModelId = &override.String
	}
	profile, found, err := readRouteProfile(ctx, snapshot, op.CostClass, task.DefaultTier)
	if err != nil {
		return ResolveProfileResult{}, err
	}
	result.RequiredCapabilities = append([]string{}, task.RequiredCapabilities...)
	if found {
		result.OrderedModelIds = applyTaskOverride(profile.OrderedModelIds, result.Assignment.OverrideModelId)
	}
	if (!found || len(result.OrderedModelIds) == 0) && (task.DefaultTier == "vision" || task.DefaultTier == "rerank") {
		profile, found, err = readRouteProfile(ctx, snapshot, op.CostClass, "fast")
		if err != nil {
			return ResolveProfileResult{}, err
		}
		if found {
			result.Fallback = true
			result.OrderedModelIds = applyTaskOverride(profile.OrderedModelIds, result.Assignment.OverrideModelId)
			if task.DefaultTier == "rerank" && result.Assignment.OverrideModelId == nil {
				result.RequiredCapabilities = append([]string{}, gen.AITierCapabilities["fast"]...)
			}
		}
	}
	if !found {
		return ResolveProfileResult{}, platform.NewError(platform.CodeNotFound, false, "任务所需档位尚未配置")
	}
	result.Profile = profile
	return result, nil
}

func readRouteProfile(ctx context.Context, snapshot *availabilitySnapshot, costClass string, tier string) (Profile, bool, error) {
	var profile Profile
	var orderedJSON []byte
	err := snapshot.tx.QueryRowContext(ctx, `SELECT profile_id, version, cost_class, tier, ordered_model_ids_json,
		maximum_cost_micro_usd, concurrency, daily_limit_cost_micro_usd FROM profiles
		WHERE data_epoch = ? AND cost_class = ? AND tier = ?`, snapshot.epoch, costClass, tier).
		Scan(&profile.ProfileId, &profile.Version, &profile.CostClass, &profile.Tier, &orderedJSON,
			&profile.MaximumCost.Micros, &profile.Concurrency, &profile.DailyLimitCost.Micros)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, false, nil
	}
	if err != nil {
		return Profile{}, false, availabilityReadError(err)
	}
	if !platform.ValidIdentifier(profile.ProfileId) || profile.Version < 1 || profile.Version > 9007199254740991 ||
		profile.MaximumCost.Micros < 0 || profile.MaximumCost.Micros > 9007199254740991 ||
		profile.DailyLimitCost.Micros < 0 || profile.DailyLimitCost.Micros > 9007199254740991 ||
		profile.Concurrency < 0 || profile.Concurrency > 9007199254740991 ||
		json.Unmarshal(orderedJSON, &profile.OrderedModelIds) != nil || profile.OrderedModelIds == nil || len(profile.OrderedModelIds) > 256 {
		return Profile{}, false, platform.NewError(platform.CodeIntegrityError, false, "档位持久配置损坏")
	}
	seen := make(map[string]bool, len(profile.OrderedModelIds))
	for _, modelID := range profile.OrderedModelIds {
		if !platform.ValidIdentifier(modelID) || seen[modelID] {
			return Profile{}, false, platform.NewError(platform.CodeIntegrityError, false, "档位候选引用损坏")
		}
		seen[modelID] = true
	}
	profile.MaximumCost.Currency = "USD"
	profile.DailyLimitCost.Currency = "USD"
	return profile, true, nil
}

func applyTaskOverride(ordered []string, override *string) []string {
	result := make([]string, 0, len(ordered)+1)
	seen := make(map[string]bool, len(ordered)+1)
	if override != nil {
		result = append(result, *override)
		seen[*override] = true
		if len(ordered) > 0 {
			ordered = ordered[1:]
		}
	}
	for _, modelID := range ordered {
		if !seen[modelID] {
			result = append(result, modelID)
			seen[modelID] = true
		}
	}
	return result
}

// SelectCandidates 只解析安全元数据；持久派发引用与费用准入留在执行链。
func SelectCandidates(ctx context.Context, op OperationContext, input ResolveRouteInput) (ResolveRouteResult, error) {
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return ResolveRouteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型配置存储尚未装配")
	}
	snapshot, err := beginAvailabilitySnapshot(ctx, op, store)
	if err != nil {
		return ResolveRouteResult{}, err
	}
	defer snapshot.tx.Rollback()
	resolved, err := ResolveProfile(ctx, op, snapshot, input)
	if err != nil {
		return ResolveRouteResult{}, err
	}
	result := ResolveRouteResult{Resolved: resolved, Candidates: make([]RouteCandidate, 0, 16)}
	var spaceID platform.Hash
	for _, modelID := range resolved.OrderedModelIds {
		candidate, eligible, err := readRouteCandidate(ctx, snapshot, modelID, resolved.RequiredCapabilities)
		if err != nil {
			return ResolveRouteResult{}, err
		}
		if !eligible {
			continue
		}
		if hasGrant(resolved.RequiredCapabilities, "embedding") {
			candidateSpace, err := embeddingSpaceID(candidate.Model)
			if err != nil {
				return ResolveRouteResult{}, platform.NewError(platform.CodeInternal, false, "嵌入空间摘要编码失败")
			}
			if spaceID != "" && spaceID != candidateSpace {
				return ResolveRouteResult{}, platform.NewError(platform.CodeIntegrityError, false, "嵌入备用模型空间不一致")
			}
			spaceID = candidateSpace
		}
		// 后续候选仍须完成同代/完整性与嵌入空间检查，不能先截断配置。
		if len(result.Candidates) < 16 {
			result.Candidates = append(result.Candidates, candidate)
		}
	}
	if len(result.Candidates) == 0 {
		return ResolveRouteResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "当前配置没有可用模型候选")
	}
	result.RouteVersion, err = hashJSON(struct {
		DataEpoch  platform.Identifier  `json:"dataEpoch"`
		Resolved   ResolveProfileResult `json:"resolved"`
		Candidates []RouteCandidate     `json:"candidates"`
	}{snapshot.epoch, resolved, result.Candidates})
	if err != nil {
		return ResolveRouteResult{}, platform.NewError(platform.CodeInternal, false, "路线版本摘要编码失败")
	}
	return result, nil
}

func readRouteCandidate(ctx context.Context, snapshot *availabilitySnapshot, modelID string, requiredCapabilities []string) (RouteCandidate, bool, error) {
	var candidate RouteCandidate
	var capabilitiesJSON, headerNamesJSON, headerRefsJSON []byte
	var modelEpoch string
	var providerEnabled sql.NullBool
	var providerVersion, credentialVersion sql.NullInt64
	var providerEpoch sql.NullString
	model := &candidate.Model
	err := snapshot.tx.QueryRowContext(ctx, `SELECT m.model_id, m.version, m.provider_id, m.provider_model_version,
		m.capabilities_json, m.dimension, m.preprocessor_version, m.max_input_tokens, m.max_output_tokens,
		m.catalog_origin, m.manual_override, m.enabled, m.data_epoch,
		p.enabled, p.version, p.credential_version, p.data_epoch, p.extra_header_names_json, p.extra_header_secret_refs_json
		FROM models m LEFT JOIN providers p ON p.provider_id = m.provider_id WHERE m.model_id = ?`, modelID).
		Scan(&model.ModelId, &model.Version, &model.ProviderId, &model.ProviderModelVersion,
			&capabilitiesJSON, &model.Dimension, &model.PreprocessorVersion, &model.MaxInputTokens, &model.MaxOutputTokens,
			&model.CatalogOrigin, &model.ManualOverride, &model.Enabled, &modelEpoch,
			&providerEnabled, &providerVersion, &credentialVersion, &providerEpoch, &headerNamesJSON, &headerRefsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return RouteCandidate{}, false, nil
	}
	if err != nil {
		return RouteCandidate{}, false, availabilityReadError(err)
	}
	if !providerEnabled.Valid || !providerVersion.Valid || !credentialVersion.Valid || !providerEpoch.Valid {
		return RouteCandidate{}, false, platform.NewError(platform.CodeIntegrityError, false, "模型供应方引用损坏")
	}
	if modelEpoch != snapshot.epoch || providerEpoch.String != snapshot.epoch {
		return RouteCandidate{}, false, platform.NewError(platform.CodeVersionMismatch, false, "模型或供应方代次已变化")
	}
	var names, refs []string
	if !platform.ValidIdentifier(model.ProviderId) || model.Version < 1 || model.Version > 9007199254740991 ||
		!validModelString(model.ProviderModelVersion) || !validModelString(model.PreprocessorVersion) || !validCatalogOrigin(model.CatalogOrigin) ||
		model.MaxInputTokens < 0 || model.MaxInputTokens > 9007199254740991 || model.MaxOutputTokens < 0 || model.MaxOutputTokens > 9007199254740991 ||
		providerVersion.Int64 < 1 || providerVersion.Int64 > 9007199254740991 || credentialVersion.Int64 < 0 || credentialVersion.Int64 > 9007199254740991 ||
		json.Unmarshal(capabilitiesJSON, &model.Capabilities) != nil || len(model.Capabilities) == 0 || len(model.Capabilities) > 256 ||
		json.Unmarshal(headerNamesJSON, &names) != nil || names == nil || len(names) > 256 ||
		json.Unmarshal(headerRefsJSON, &refs) != nil || refs == nil || len(names) != len(refs) {
		return RouteCandidate{}, false, platform.NewError(platform.CodeIntegrityError, false, "模型或供应方元数据损坏")
	}
	for _, capability := range model.Capabilities {
		if !validCapability(capability) {
			return RouteCandidate{}, false, platform.NewError(platform.CodeIntegrityError, false, "模型能力损坏")
		}
	}
	if (hasGrant(model.Capabilities, "embedding") && (model.Dimension < 1 || model.Dimension > maxEmbeddingDimension)) ||
		(!hasGrant(model.Capabilities, "embedding") && model.Dimension != 0) ||
		((hasGrant(model.Capabilities, "chat") || hasGrant(model.Capabilities, "translation")) && model.MaxOutputTokens < 1) {
		return RouteCandidate{}, false, platform.NewError(platform.CodeIntegrityError, false, "模型能力边界损坏")
	}
	for i, name := range names {
		if name == "" || refs[i] == "" {
			return RouteCandidate{}, false, platform.NewError(platform.CodeIntegrityError, false, "供应方附加头引用不完整")
		}
	}
	if !model.Enabled || !providerEnabled.Bool || credentialVersion.Int64 == 0 {
		return RouteCandidate{}, false, nil
	}
	for _, required := range requiredCapabilities {
		if !hasGrant(model.Capabilities, required) {
			return RouteCandidate{}, false, nil
		}
	}
	var credentialEpoch string
	var nonceBytes, ciphertextBytes int64
	err = snapshot.tx.QueryRowContext(ctx, `SELECT data_epoch, length(nonce), length(ciphertext) FROM credentials
		WHERE provider_id = ? AND credential_version = ?`, model.ProviderId, credentialVersion.Int64).
		Scan(&credentialEpoch, &nonceBytes, &ciphertextBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return RouteCandidate{}, false, nil
	}
	if err != nil {
		return RouteCandidate{}, false, availabilityReadError(err)
	}
	if credentialEpoch != snapshot.epoch {
		return RouteCandidate{}, false, platform.NewError(platform.CodeVersionMismatch, false, "凭据引用代次已变化")
	}
	if nonceBytes != 12 || ciphertextBytes <= 16 {
		return RouteCandidate{}, false, platform.NewError(platform.CodeIntegrityError, false, "凭据引用材料不完整")
	}
	candidate.ProviderVersion = int(providerVersion.Int64)
	candidate.CredentialVersion = int(credentialVersion.Int64)
	return candidate, true, nil
}

// NewProfileHandlers 只绑定真实档位写入，统一授权与严格schema由公共入口完成。
func NewProfileHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"putProfile": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, ok := platform.OperationContextFrom(r.Context())
			if !ok {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeUnauthenticated, false, "缺少可信服务操作上下文"), platform.NewIdentifier()))
				return
			}
			var input PutProfileInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "档位配置输入不可读"), op.RequestID))
				return
			}
			if r.PathValue("profileId") != input.Profile.ProfileId || (r.Header.Get("Idempotency-Key") != "" && r.Header.Get("Idempotency-Key") != input.IdempotencyKey) {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "档位路径或幂等头与正文不一致"), op.RequestID))
				return
			}
			result, err := PutProfile(r.Context(), op, input)
			if err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_ = json.NewEncoder(w).Encode(result)
		}),
	}
}
