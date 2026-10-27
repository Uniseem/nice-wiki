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
	"nice-wiki/libs/platform/problem"
	"nice-wiki/libs/platform/queue"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

// 免费配置读取只投影任务就绪元数据，不代表模型执行授权或供应方在线。
type ReadTaskAvailabilityInput = gen.ReadTaskAvailabilityInput
type ReadTaskAvailabilityResult = gen.ReadTaskAvailabilityResult
type TaskAvailability = gen.TaskAvailability
type PrivateStore = gatewayruntime.Store
type TaskAssignment = gen.TaskAssignment
type PutTaskAssignmentInput = gen.PutTaskAssignmentInput
type PutTaskAssignmentResult = gen.PutTaskAssignmentResult
type ReadTaskAssignmentsResult = gen.ReadTaskAssignmentsResult
type TaskAssignmentAuthorizer func(context.Context, OperationContext, string, string) error

type ReadTaskAssignmentsInput struct {
	TaskId *string `json:"taskId,omitempty"`
}

const taskAssignmentFunctionID = "model-gateway.PutTaskAssignment"

type availabilitySnapshot struct {
	tx    *sql.Tx
	epoch platform.Identifier
}

// PutTaskAssignment 只修改任务模型覆盖，不调用供应方或创建费用任务。
func PutTaskAssignment(ctx context.Context, op OperationContext, input PutTaskAssignmentInput) (PutTaskAssignmentResult, error) {
	if op.Actor.Kind != "admin" || !hasGrant(op.Actor.Grants, "superadmin") {
		return PutTaskAssignmentResult{}, platform.NewError(platform.CodeForbidden, false, "仅超管可修改任务模型分配")
	}
	task, ok := gen.AITaskCatalog[input.Assignment.TaskId]
	if !ok || string(input.Assignment.Tier) != task.DefaultTier {
		return PutTaskAssignmentResult{}, platform.NewError(platform.CodeValidation, false, "任务或默认档位不符合目录")
	}
	if input.Assignment.Version != input.ExpectedVersion+1 {
		return PutTaskAssignmentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "任务分配版本必须为预期版本加一")
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return PutTaskAssignmentResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型配置存储尚未装配")
	}
	snapshot, err := beginAvailabilitySnapshot(ctx, op, store)
	if err != nil {
		return PutTaskAssignmentResult{}, err
	}
	defer snapshot.tx.Rollback()
	inputHash, err := hashJSON(struct {
		CallerService   string              `json:"callerService"`
		DataEpoch       platform.Identifier `json:"dataEpoch"`
		Assignment      TaskAssignment      `json:"assignment"`
		ExpectedVersion int                 `json:"expectedVersion"`
	}{op.CallerService, op.DataEpoch, input.Assignment, input.ExpectedVersion})
	if err != nil {
		return PutTaskAssignmentResult{}, platform.NewError(platform.CodeInternal, false, "任务分配摘要编码失败")
	}
	if result, found, err := readTaskAssignmentReceipt(ctx, op, snapshot, input, inputHash); err != nil {
		return PutTaskAssignmentResult{}, err
	} else if found {
		return result, nil
	}
	var blocked bool
	if err := snapshot.tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM barriers WHERE state <> 'released')`).Scan(&blocked); err != nil {
		return PutTaskAssignmentResult{}, availabilityReadError(err)
	}
	if blocked {
		return PutTaskAssignmentResult{}, platform.NewError(platform.CodeLocked, false, "模型配置屏障尚未释放")
	}
	if input.Assignment.OverrideModelId != nil {
		if err := validateTaskOverride(ctx, snapshot, *input.Assignment.OverrideModelId, task.RequiredCapabilities); err != nil {
			return PutTaskAssignmentResult{}, err
		}
	}
	var currentVersion int
	var currentEpoch string
	err = snapshot.tx.QueryRowContext(ctx, `SELECT version, data_epoch FROM task_assignments WHERE task_id = ?`, input.Assignment.TaskId).
		Scan(&currentVersion, &currentEpoch)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return PutTaskAssignmentResult{}, availabilityReadError(err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		if input.ExpectedVersion != 0 {
			return PutTaskAssignmentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "任务分配尚未创建")
		}
	} else if currentEpoch != op.DataEpoch || currentVersion != input.ExpectedVersion {
		return PutTaskAssignmentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "任务分配版本或代次已变化")
	}
	var override any
	if input.Assignment.OverrideModelId != nil {
		override = *input.Assignment.OverrideModelId
	}
	now := platform.Now()
	var changed sql.Result
	if input.ExpectedVersion == 0 {
		changed, err = snapshot.tx.ExecContext(ctx, `INSERT INTO task_assignments
			(task_id, tier, override_model_id, version, data_epoch, updated_at) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(task_id) DO NOTHING`, input.Assignment.TaskId, input.Assignment.Tier, override,
			input.Assignment.Version, op.DataEpoch, now)
	} else {
		changed, err = snapshot.tx.ExecContext(ctx, `UPDATE task_assignments SET tier = ?, override_model_id = ?, version = ?, updated_at = ?
			WHERE task_id = ? AND version = ? AND data_epoch = ?`, input.Assignment.Tier, override, input.Assignment.Version,
			now, input.Assignment.TaskId, input.ExpectedVersion, op.DataEpoch)
	}
	if err != nil {
		return PutTaskAssignmentResult{}, availabilityReadError(err)
	}
	affected, err := changed.RowsAffected()
	if err != nil {
		return PutTaskAssignmentResult{}, availabilityReadError(err)
	}
	if affected != 1 {
		return PutTaskAssignmentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "任务分配版本已变化")
	}
	payload, err := json.Marshal(struct {
		TaskID    string              `json:"taskId"`
		Version   int                 `json:"version"`
		DataEpoch platform.Identifier `json:"dataEpoch"`
	}{input.Assignment.TaskId, input.Assignment.Version, op.DataEpoch})
	if err != nil {
		return PutTaskAssignmentResult{}, platform.NewError(platform.CodeInternal, false, "任务分配事件编码失败")
	}
	if err := queue.EnqueueEvent(snapshot.tx, platform.EventEnvelope{
		EventID: platform.NewIdentifier(), EventType: "assignment.changed", EventVersion: 1,
		Producer: "model-gateway", SubjectID: input.Assignment.TaskId, Revision: platform.Revision(input.Assignment.Version),
		OccurredAt: now, DataEpoch: op.DataEpoch, Payload: payload,
	}); err != nil {
		return PutTaskAssignmentResult{}, err
	}
	result := PutTaskAssignmentResult{Assignment: input.Assignment}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return PutTaskAssignmentResult{}, platform.NewError(platform.CodeInternal, false, "任务分配回执编码失败")
	}
	// 有效回执已经原样返回；此处只会替换当前事务先前读到的过期回执。
	if _, err := snapshot.tx.ExecContext(ctx, `DELETE FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`,
		op.Actor.SiteID, op.Actor.SubjectID, taskAssignmentFunctionID, input.IdempotencyKey); err != nil {
		return PutTaskAssignmentResult{}, availabilityReadError(err)
	}
	if _, err := snapshot.tx.ExecContext(ctx, `INSERT INTO operation_receipts
		(site_id, actor_id, function_id, idempotency_key, input_hash, result_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		op.Actor.SiteID, op.Actor.SubjectID, taskAssignmentFunctionID, input.IdempotencyKey, inputHash, resultJSON, now); err != nil {
		return PutTaskAssignmentResult{}, availabilityReadError(err)
	}
	if err := snapshot.tx.Commit(); err != nil {
		return PutTaskAssignmentResult{}, availabilityReadError(err)
	}
	return result, nil
}

func readTaskAssignmentReceipt(ctx context.Context, op OperationContext, snapshot *availabilitySnapshot, input PutTaskAssignmentInput, inputHash platform.Hash) (PutTaskAssignmentResult, bool, error) {
	var storedHash, createdAt string
	var resultJSON []byte
	err := snapshot.tx.QueryRowContext(ctx, `SELECT input_hash, result_json, created_at FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`,
		op.Actor.SiteID, op.Actor.SubjectID, taskAssignmentFunctionID, input.IdempotencyKey).Scan(&storedHash, &resultJSON, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PutTaskAssignmentResult{}, false, nil
	}
	if err != nil {
		return PutTaskAssignmentResult{}, false, availabilityReadError(err)
	}
	created, err := platform.ParseTimestamp(createdAt)
	if err != nil || created.After(time.Now().UTC()) {
		return PutTaskAssignmentResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "任务分配回执时间损坏")
	}
	if time.Since(created) > 7*24*time.Hour {
		return PutTaskAssignmentResult{}, false, nil
	}
	if storedHash != inputHash {
		return PutTaskAssignmentResult{}, false, platform.NewError(platform.CodeConflict, false, "幂等键已绑定不同任务分配或代次")
	}
	var result PutTaskAssignmentResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return PutTaskAssignmentResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "任务分配原始回执损坏")
	}
	storedResult, _ := json.Marshal(result)
	expectedResult, _ := json.Marshal(PutTaskAssignmentResult{Assignment: input.Assignment})
	if !bytes.Equal(storedResult, expectedResult) {
		return PutTaskAssignmentResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "任务分配原始结果与摘要不一致")
	}
	return result, true, nil
}

func validateTaskOverride(ctx context.Context, snapshot *availabilitySnapshot, modelID string, requiredCapabilities []string) error {
	var enabled bool
	var capabilitiesJSON []byte
	var epoch string
	err := snapshot.tx.QueryRowContext(ctx, `SELECT enabled, capabilities_json, data_epoch FROM models WHERE model_id = ?`, modelID).
		Scan(&enabled, &capabilitiesJSON, &epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.NewError(platform.CodeValidation, false, "所选模型不存在")
	}
	if err != nil {
		return availabilityReadError(err)
	}
	if epoch != snapshot.epoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "所选模型代次已变化")
	}
	var capabilities []string
	if json.Unmarshal(capabilitiesJSON, &capabilities) != nil || len(capabilities) == 0 || len(capabilities) > 256 {
		return platform.NewError(platform.CodeIntegrityError, false, "所选模型能力损坏")
	}
	for _, capability := range capabilities {
		if !validCapability(capability) {
			return platform.NewError(platform.CodeIntegrityError, false, "所选模型能力损坏")
		}
	}
	if !enabled {
		return platform.NewError(platform.CodeValidation, false, "所选模型未启用")
	}
	for _, required := range requiredCapabilities {
		if !hasGrant(capabilities, required) {
			return platform.NewError(platform.CodeValidation, false, "所选模型能力不满足目录")
		}
	}
	return nil
}

// ReadTaskAssignments 按任务owner过滤本域真实赋值，不把默认目录当已保存配置。
func ReadTaskAssignments(ctx context.Context, op OperationContext, input ReadTaskAssignmentsInput) (ReadTaskAssignmentsResult, error) {
	admin := op.CallerService == "portal" && op.Actor.Kind == "admin" && hasGrant(op.Actor.Grants, "superadmin")
	allowed := admin
	if op.Actor.Kind == "service" {
		for _, task := range gen.AITaskCatalog {
			allowed = allowed || task.Owner == op.CallerService
		}
	}
	if !allowed {
		return ReadTaskAssignmentsResult{}, platform.NewError(platform.CodeForbidden, false, "调用主体不能读取任务模型分配")
	}
	if input.TaskId != nil {
		task, ok := gen.AITaskCatalog[*input.TaskId]
		if !ok {
			return ReadTaskAssignmentsResult{}, platform.NewError(platform.CodeValidation, false, "任务未在目录登记")
		}
		if !admin && task.Owner != op.CallerService {
			return ReadTaskAssignmentsResult{}, platform.NewError(platform.CodeForbidden, false, "任务不属于调用服务")
		}
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return ReadTaskAssignmentsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型配置存储尚未装配")
	}
	snapshot, err := beginAvailabilitySnapshot(ctx, op, store)
	if err != nil {
		return ReadTaskAssignmentsResult{}, err
	}
	defer snapshot.tx.Rollback()
	taskIDs := make([]string, 0, len(gen.AITaskCatalog))
	for taskID, task := range gen.AITaskCatalog {
		if (input.TaskId == nil || *input.TaskId == taskID) && (admin || task.Owner == op.CallerService) {
			taskIDs = append(taskIDs, taskID)
		}
	}
	sort.Strings(taskIDs)
	result := ReadTaskAssignmentsResult{Assignments: make([]TaskAssignment, 0, len(taskIDs))}
	for _, taskID := range taskIDs {
		var assignment TaskAssignment
		var override sql.NullString
		var epoch string
		err := snapshot.tx.QueryRowContext(ctx, `SELECT task_id, tier, override_model_id, version, data_epoch
			FROM task_assignments WHERE task_id = ?`, taskID).Scan(&assignment.TaskId, &assignment.Tier, &override, &assignment.Version, &epoch)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return ReadTaskAssignmentsResult{}, availabilityReadError(err)
		}
		if epoch != op.DataEpoch {
			return ReadTaskAssignmentsResult{}, platform.NewError(platform.CodeVersionMismatch, false, "任务模型分配代次已变化")
		}
		if string(assignment.Tier) != gen.AITaskCatalog[taskID].DefaultTier || assignment.Version < 1 || assignment.Version > 9007199254740991 || (override.Valid && !platform.ValidIdentifier(override.String)) {
			return ReadTaskAssignmentsResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务模型分配损坏")
		}
		if override.Valid {
			assignment.OverrideModelId = &override.String
		}
		result.Assignments = append(result.Assignments, assignment)
	}
	return result, nil
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
	case "fast", "accurate", "vision", "embedding", "rerank", "decision":
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
	_, eligible, err := readRouteCandidate(ctx, snapshot, modelID, []string{"chat"})
	return eligible, err
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

// NewTaskAvailabilityHandlers 提供真实任务配置映射，完整服务仍须统一边界核全表。
func NewTaskAvailabilityHandlers(authorize TaskAssignmentAuthorizer) map[string]http.Handler {
	return map[string]http.Handler{
		"putTaskAssignment": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, ok := platform.OperationContextFrom(r.Context())
			if !ok {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeUnauthenticated, false, "缺少可信服务操作上下文"), platform.NewIdentifier()))
				return
			}
			var input PutTaskAssignmentInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "任务分配输入不可读"), op.RequestID))
				return
			}
			if r.PathValue("taskId") != input.Assignment.TaskId || (r.Header.Get("Idempotency-Key") != "" && r.Header.Get("Idempotency-Key") != input.IdempotencyKey) {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "任务分配路径或幂等头与正文不一致"), op.RequestID))
				return
			}
			result, err := PutTaskAssignment(r.Context(), op, input)
			if err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_ = json.NewEncoder(w).Encode(result)
		}),
		"readTaskAssignments": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, ok := platform.OperationContextFrom(r.Context())
			if !ok {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeUnauthenticated, false, "缺少可信服务操作上下文"), platform.NewIdentifier()))
				return
			}
			if op.Actor.Kind == "admin" {
				if authorize == nil {
					_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeDependencyUnavailable, false, "任务配置当前授权尚未装配"), op.RequestID))
					return
				}
				if err := authorize(r.Context(), op, "superadmin", "model-gateway:readTaskAssignments"); err != nil {
					_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
					return
				}
			}
			var input ReadTaskAssignmentsInput
			if query := r.URL.Query(); query.Has("taskId") {
				taskID := query.Get("taskId")
				input.TaskId = &taskID
			}
			result, err := ReadTaskAssignments(r.Context(), op, input)
			if err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_ = json.NewEncoder(w).Encode(result)
		}),
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
