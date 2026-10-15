package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

// 免费配置读取只投影任务就绪元数据，不代表模型执行授权或供应方在线。
type ReadTaskAvailabilityInput = gen.ReadTaskAvailabilityInput
type ReadTaskAvailabilityResult = gen.ReadTaskAvailabilityResult
type TaskAvailability = gen.TaskAvailability
type PrivateStore = gatewayruntime.Store

type availabilitySnapshot struct {
	tx    *sql.Tx
	epoch platform.Identifier
}

// ReadTaskAvailability 对一次请求使用同一当前代次快照，不探测或调用模型。
func ReadTaskAvailability(ctx context.Context, op OperationContext, input ReadTaskAvailabilityInput) (ReadTaskAvailabilityResult, error) {
	if op.CallerService != "qa" || op.Actor.Kind != "service" {
		return ReadTaskAvailabilityResult{}, platform.NewError(platform.CodeForbidden, false, "仅QA纯服务可读取任务配置能力")
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return ReadTaskAvailabilityResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型配置存储尚未装配")
	}
	snapshot, err := beginAvailabilitySnapshot(ctx, op, store)
	if err != nil {
		return ReadTaskAvailabilityResult{}, err
	}
	defer snapshot.tx.Rollback()
	result := ReadTaskAvailabilityResult{Tasks: make([]TaskAvailability, 0, len(input.TaskIds))}
	for _, taskID := range input.TaskIds {
		task, err := readAvailabilityTask(ctx, snapshot, string(taskID))
		if err != nil {
			return ReadTaskAvailabilityResult{}, err
		}
		result.Tasks = append(result.Tasks, task)
	}
	return result, nil
}

func beginAvailabilitySnapshot(ctx context.Context, op OperationContext, store *PrivateStore) (*availabilitySnapshot, error) {
	tx, err := store.BeginTx(ctx)
	if err != nil {
		return nil, availabilityReadError(err)
	}
	var epoch string
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch FROM runtime_meta WHERE singleton = 1`).Scan(&epoch); err != nil {
		_ = tx.Rollback()
		return nil, availabilityReadError(err)
	}
	if epoch == "" {
		_ = tx.Rollback()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "模型配置当前代次尚未知")
	}
	if epoch != op.DataEpoch {
		_ = tx.Rollback()
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "模型配置当前代次已变化")
	}
	return &availabilitySnapshot{tx: tx, epoch: op.DataEpoch}, nil
}

func readAvailabilityTask(ctx context.Context, snapshot *availabilitySnapshot, taskID string) (TaskAvailability, error) {
	result := TaskAvailability{TaskId: gen.TaskAvailabilityTaskId(taskID), Reason: gen.NotConfigured}
	var tier, epoch string
	var override sql.NullString
	err := snapshot.tx.QueryRowContext(ctx, `SELECT tier, override_model_id, data_epoch FROM task_assignments WHERE task_id = ?`, taskID).
		Scan(&tier, &override, &epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return TaskAvailability{}, availabilityReadError(err)
	}
	if epoch != snapshot.epoch {
		return TaskAvailability{}, platform.NewError(platform.CodeVersionMismatch, false, "任务分配代次已变化")
	}
	switch tier {
	case "fast", "accurate", "vision", "embedding", "rerank":
	default:
		return TaskAvailability{}, platform.NewError(platform.CodeIntegrityError, false, "任务分配档位损坏")
	}
	if override.Valid && !platform.ValidIdentifier(override.String) {
		return TaskAvailability{}, platform.NewError(platform.CodeIntegrityError, false, "任务覆盖引用损坏")
	}
	var orderedJSON []byte
	err = snapshot.tx.QueryRowContext(ctx, `SELECT ordered_model_ids_json FROM profiles WHERE data_epoch = ? AND cost_class = 'qa' AND tier = ?`,
		snapshot.epoch, tier).Scan(&orderedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return TaskAvailability{}, availabilityReadError(err)
	}
	var ordered []string
	if json.Unmarshal(orderedJSON, &ordered) != nil || ordered == nil || len(ordered) > 256 {
		return TaskAvailability{}, platform.NewError(platform.CodeIntegrityError, false, "档位候选配置损坏")
	}
	seen := make(map[string]bool, len(ordered))
	for _, modelID := range ordered {
		if !platform.ValidIdentifier(modelID) || seen[modelID] {
			return TaskAvailability{}, platform.NewError(platform.CodeIntegrityError, false, "档位候选引用损坏")
		}
		seen[modelID] = true
	}
	if override.Valid {
		if len(ordered) == 0 {
			ordered = append(ordered, override.String)
		} else {
			ordered[0] = override.String
		}
	}
	clear(seen)
	result.Reason = gen.Unavailable
	for _, modelID := range ordered {
		if seen[modelID] {
			continue
		}
		seen[modelID] = true
		available, err := readAvailabilityCandidate(ctx, snapshot, modelID)
		if err != nil {
			return TaskAvailability{}, err
		}
		if available {
			result.Available = true
			result.Reason = gen.Available
		}
	}
	return result, nil
}

func readAvailabilityCandidate(ctx context.Context, snapshot *availabilitySnapshot, modelID string) (bool, error) {
	var enabled bool
	var capabilitiesJSON, headerNamesJSON, headerRefsJSON []byte
	var modelEpoch, providerID string
	var providerEnabled sql.NullBool
	var credentialVersion sql.NullInt64
	var providerEpoch sql.NullString
	err := snapshot.tx.QueryRowContext(ctx, `SELECT m.enabled, m.capabilities_json, m.data_epoch, m.provider_id,
		p.enabled, p.credential_version, p.data_epoch, p.extra_header_names_json, p.extra_header_secret_refs_json
		FROM models m LEFT JOIN providers p ON p.provider_id = m.provider_id WHERE m.model_id = ?`, modelID).
		Scan(&enabled, &capabilitiesJSON, &modelEpoch, &providerID, &providerEnabled, &credentialVersion, &providerEpoch, &headerNamesJSON, &headerRefsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, availabilityReadError(err)
	}
	if !providerEnabled.Valid || !credentialVersion.Valid || !providerEpoch.Valid {
		return false, platform.NewError(platform.CodeIntegrityError, false, "模型供应方引用损坏")
	}
	if modelEpoch != snapshot.epoch || providerEpoch.String != snapshot.epoch {
		return false, platform.NewError(platform.CodeVersionMismatch, false, "模型或供应方代次已变化")
	}
	var capabilities, names, refs []string
	if json.Unmarshal(capabilitiesJSON, &capabilities) != nil || capabilities == nil || len(capabilities) > 256 ||
		json.Unmarshal(headerNamesJSON, &names) != nil || names == nil || len(names) > 256 ||
		json.Unmarshal(headerRefsJSON, &refs) != nil || refs == nil || len(names) != len(refs) {
		return false, platform.NewError(platform.CodeIntegrityError, false, "模型能力或附加头引用损坏")
	}
	for i, name := range names {
		if name == "" || refs[i] == "" {
			return false, platform.NewError(platform.CodeIntegrityError, false, "供应方附加头引用不完整")
		}
	}
	chat := false
	for _, capability := range capabilities {
		if capability == "chat" {
			chat = true
		}
	}
	if !enabled || !chat || !providerEnabled.Bool || credentialVersion.Int64 == 0 {
		return false, nil
	}
	var credentialEpoch string
	var nonceBytes, ciphertextBytes int64
	err = snapshot.tx.QueryRowContext(ctx, `SELECT data_epoch, length(nonce), length(ciphertext) FROM credentials
		WHERE provider_id = ? AND credential_version = ?`, providerID, credentialVersion.Int64).
		Scan(&credentialEpoch, &nonceBytes, &ciphertextBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, availabilityReadError(err)
	}
	if credentialEpoch != snapshot.epoch {
		return false, platform.NewError(platform.CodeVersionMismatch, false, "凭据引用代次已变化")
	}
	if nonceBytes != 12 || ciphertextBytes <= 16 {
		return false, platform.NewError(platform.CodeIntegrityError, false, "凭据引用材料不完整")
	}
	return true, nil
}

func availabilityReadError(err error) error {
	var public *platform.Error
	_ = errors.As(err, &public)
	if errors.Is(err, context.Canceled) || (public != nil && public.Code == platform.CodeCancelled) {
		return platform.NewError(platform.CodeCancelled, false, "任务配置读取已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) || (public != nil && public.Code == platform.CodeDeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, false, "任务配置读取已到截止")
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "任务配置权威快照暂不可读")
}

// NewTaskAvailabilityHandlers 只提供真实映射，完整服务仍须统一边界核全表。
func NewTaskAvailabilityHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"readTaskAvailability": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, ok := platform.OperationContextFrom(r.Context())
			if !ok {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeUnauthenticated, false, "缺少可信服务操作上下文"), platform.NewIdentifier()))
				return
			}
			var input ReadTaskAvailabilityInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "任务配置读取输入不可读"), op.RequestID))
				return
			}
			result, err := ReadTaskAvailability(r.Context(), op, input)
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
