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
