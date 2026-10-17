package catalog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

// Model 复用OpenAPI生成的模型类型。
type Model = gen.Model

// EmbeddingSpace 是公共schema中用于跨域比较边界的最小本域表示。
// 其spaceId只由供应方、不可变模型版本、维度和预处理版本计算。
type EmbeddingSpace struct {
	SpaceID              platform.Hash `json:"spaceId"`
	Provider             string        `json:"provider"`
	ModelVersion         string        `json:"modelVersion"`
	Dimensions           int           `json:"dimensions"`
	PreprocessingVersion string        `json:"preprocessingVersion"`
}

type ValidateModelInput struct {
	Model Model `json:"model"`
}

type ValidateModelResult struct {
	Valid          bool            `json:"valid"`
	EmbeddingSpace *EmbeddingSpace `json:"embeddingSpace,omitempty"`
}

type PutModelInput struct {
	Model           Model  `json:"model"`
	ExpectedVersion int    `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

type PutModelResult struct {
	Model Model `json:"model"`
}

const (
	modelFunctionID       = "model-gateway.PutModel"
	modelEventType        = "model.changed"
	modelEventVersion     = 1
	maxModelStringRunes   = 65536
	maxCapabilityRunes    = 4096
	maxCapabilities       = 256
	maxEmbeddingDimension = 65536
)

// ValidateModel 是无网络、无收费的模型能力与嵌入空间身份校验。
func ValidateModel(ctx context.Context, op OperationContext, input ValidateModelInput) (ValidateModelResult, error) {
	operationCtx, cancel, err := operationContext(ctx, op, 5*time.Second)
	if err != nil {
		return ValidateModelResult{}, err
	}
	defer cancel()
	if err := contextError(operationCtx); err != nil {
		return ValidateModelResult{}, err
	}
	model := input.Model
	if !platform.ValidIdentifier(model.ModelId) || !platform.ValidIdentifier(model.ProviderId) || model.Version < 0 ||
		!validModelString(model.ProviderModelVersion) || !validModelString(model.PreprocessorVersion) ||
		model.MaxInputTokens < 0 || model.MaxOutputTokens < 0 || len(model.Capabilities) == 0 || len(model.Capabilities) > maxCapabilities {
		return ValidateModelResult{}, platform.NewError(platform.CodeValidation, false, "模型声明基础字段不合法")
	}
	if !validCatalogOrigin(model.CatalogOrigin) {
		return ValidateModelResult{}, platform.NewError(platform.CodeValidation, false, "模型目录来源不合法")
	}
	capabilities := make(map[string]struct{}, len(model.Capabilities))
	for _, capability := range model.Capabilities {
		if !validCapability(capability) || !validBoundedString(capability, maxCapabilityRunes) {
			return ValidateModelResult{}, platform.NewError(platform.CodeValidation, false, "模型能力不在允许集合")
		}
		capabilities[capability] = struct{}{}
	}
	_, isEmbedding := capabilities["embedding"]
	if isEmbedding {
		if model.Dimension < 1 || model.Dimension > maxEmbeddingDimension {
			return ValidateModelResult{}, platform.NewError(platform.CodeValidation, false, "嵌入维度不合法")
		}
	} else if model.Dimension != 0 {
		return ValidateModelResult{}, platform.NewError(platform.CodeValidation, false, "非嵌入模型维度必须为零")
	}
	if _, isChat := capabilities["chat"]; isChat && model.MaxOutputTokens < 1 {
		return ValidateModelResult{}, platform.NewError(platform.CodeValidation, false, "chat模型输出上限必须为正")
	}
	if _, isTranslation := capabilities["translation"]; isTranslation && model.MaxOutputTokens < 1 {
		return ValidateModelResult{}, platform.NewError(platform.CodeValidation, false, "翻译模型输出上限必须为正")
	}
	result := ValidateModelResult{Valid: true}
	if isEmbedding {
		spaceID, err := embeddingSpaceID(model)
		if err != nil {
			return ValidateModelResult{}, platform.NewError(platform.CodeInternal, false, "嵌入空间摘要计算失败")
		}
		result.EmbeddingSpace = &EmbeddingSpace{
			SpaceID:              spaceID,
			Provider:             string(model.ProviderId),
			ModelVersion:         model.ProviderModelVersion,
			Dimensions:           model.Dimension,
			PreprocessingVersion: model.PreprocessorVersion,
		}
	}
	return result, nil
}

// PutModel 在单个SQLite事务内保存模型身份和model.changed outbox。
// 真实模型目录刷新、连接探测、调用及费用预留不在本切片内。
func PutModel(ctx context.Context, op OperationContext, input PutModelInput) (PutModelResult, error) {
	if err := requireSuperadmin(op); err != nil {
		return PutModelResult{}, err
	}
	if input.ExpectedVersion >= 9007199254740991 || input.Model.Version != input.ExpectedVersion+1 {
		return PutModelResult{}, platform.NewError(platform.CodeVersionMismatch, false, "模型版本必须为预期版本加一")
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return PutModelResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型网关数据库尚未装配")
	}
	snapshot, err := beginAvailabilitySnapshot(ctx, op, store)
	if err != nil {
		return PutModelResult{}, err
	}
	tx := snapshot.tx
	defer tx.Rollback()
	inputHash, err := hashJSON(struct {
		CallerService   string              `json:"callerService"`
		DataEpoch       platform.Identifier `json:"dataEpoch"`
		Model           Model               `json:"model"`
		ExpectedVersion int                 `json:"expectedVersion"`
	}{op.CallerService, op.DataEpoch, input.Model, input.ExpectedVersion})
	if err != nil {
		return PutModelResult{}, platform.NewError(platform.CodeInternal, false, "模型输入摘要计算失败")
	}
	if result, found, err := readModelReceipt(ctx, op, tx, input.IdempotencyKey, inputHash, input.Model); err != nil {
		return PutModelResult{}, err
	} else if found {
		return result, nil
	}
	var blocked bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM barriers WHERE state <> 'released')`).Scan(&blocked); err != nil {
		return PutModelResult{}, availabilityReadError(err)
	}
	if blocked {
		return PutModelResult{}, platform.NewError(platform.CodeLocked, false, "模型配置屏障尚未释放")
	}
	if hasGrant(input.Model.Capabilities, "embedding") {
		return PutModelResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "嵌入空间归一化身份尚未确认，暂不能保存嵌入模型配置")
	}
	validation, err := ValidateModel(ctx, op, ValidateModelInput{Model: input.Model})
	if err != nil {
		return PutModelResult{}, err
	}
	var providerEpoch string
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch FROM providers WHERE provider_id = ?`, input.Model.ProviderId).Scan(&providerEpoch); errors.Is(err, sql.ErrNoRows) {
		return PutModelResult{}, platform.NewError(platform.CodeNotFound, false, "模型供应方不存在")
	} else if err != nil {
		return PutModelResult{}, availabilityReadError(err)
	}
	if providerEpoch != op.DataEpoch {
		return PutModelResult{}, platform.NewError(platform.CodeVersionMismatch, false, "模型供应方代次已变化")
	}
	var currentVersion, dimension int
	var currentEpoch, providerID, modelVersion, preprocessorVersion string
	err = tx.QueryRowContext(ctx, `SELECT version, data_epoch, provider_id, provider_model_version, dimension, preprocessor_version
		FROM models WHERE model_id = ?`, input.Model.ModelId).Scan(&currentVersion, &currentEpoch, &providerID, &modelVersion, &dimension, &preprocessorVersion)
	switch {
	case errors.Is(err, sql.ErrNoRows) && input.ExpectedVersion != 0:
		return PutModelResult{}, platform.NewError(platform.CodeNotFound, false, "模型不存在")
	case err == nil && (currentEpoch != op.DataEpoch || currentVersion != input.ExpectedVersion):
		return PutModelResult{}, platform.NewError(platform.CodeVersionMismatch, false, "模型版本或代次已变化")
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return PutModelResult{}, availabilityReadError(err)
	}
	if err == nil && (providerID != input.Model.ProviderId || modelVersion != input.Model.ProviderModelVersion ||
		dimension != input.Model.Dimension || preprocessorVersion != input.Model.PreprocessorVersion) {
		return PutModelResult{}, platform.NewError(platform.CodeConflict, false, "模型身份变化必须使用新的模型ID")
	}
	model := input.Model
	capabilities, _ := json.Marshal(model.Capabilities)
	saved, err := tx.ExecContext(ctx, `
		INSERT INTO models (
			model_id, version, provider_id, provider_model_version, capabilities_json,
			dimension, preprocessor_version, max_input_tokens, max_output_tokens,
			catalog_origin, manual_override, enabled, data_epoch, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(model_id) DO UPDATE SET
			version = excluded.version, capabilities_json = excluded.capabilities_json,
			max_input_tokens = excluded.max_input_tokens, max_output_tokens = excluded.max_output_tokens,
			catalog_origin = excluded.catalog_origin, manual_override = excluded.manual_override,
			enabled = excluded.enabled, updated_at = excluded.updated_at
		WHERE models.version = ? AND models.data_epoch = ?`,
		model.ModelId, model.Version, model.ProviderId, model.ProviderModelVersion, capabilities,
		model.Dimension, model.PreprocessorVersion, model.MaxInputTokens, model.MaxOutputTokens,
		model.CatalogOrigin, model.ManualOverride, model.Enabled, op.DataEpoch, platform.Now(), input.ExpectedVersion, op.DataEpoch)
	if err != nil {
		return PutModelResult{}, availabilityReadError(err)
	}
	if count, err := saved.RowsAffected(); err != nil {
		return PutModelResult{}, availabilityReadError(err)
	} else if count != 1 {
		return PutModelResult{}, platform.NewError(platform.CodeVersionMismatch, false, "模型保存条件已变化")
	}
	payload, err := json.Marshal(struct {
		ModelID   string          `json:"modelId"`
		Version   int             `json:"version"`
		Space     *EmbeddingSpace `json:"space,omitempty"`
		DataEpoch string          `json:"dataEpoch"`
	}{model.ModelId, model.Version, validation.EmbeddingSpace, op.DataEpoch})
	if err != nil {
		return PutModelResult{}, platform.NewError(platform.CodeInternal, false, "模型事件编码失败")
	}
	now := platform.Now()
	if err := outbox.EnqueueEvent(tx, platform.EventEnvelope{
		EventID: platform.NewIdentifier(), EventType: modelEventType, EventVersion: modelEventVersion,
		Producer: "model-gateway", SubjectID: model.ModelId, Revision: platform.Revision(model.Version),
		OccurredAt: now, DataEpoch: op.DataEpoch, Payload: payload,
	}); err != nil {
		return PutModelResult{}, availabilityReadError(err)
	}
	result := PutModelResult{Model: model}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return PutModelResult{}, platform.NewError(platform.CodeInternal, false, "模型回执编码失败")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM operation_receipts WHERE site_id=? AND actor_id=? AND function_id=? AND idempotency_key=?`,
		op.Actor.SiteID, op.Actor.SubjectID, modelFunctionID, input.IdempotencyKey); err != nil {
		return PutModelResult{}, availabilityReadError(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO operation_receipts
		(site_id, actor_id, function_id, idempotency_key, input_hash, result_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		op.Actor.SiteID, op.Actor.SubjectID, modelFunctionID, input.IdempotencyKey, inputHash, resultJSON, now); err != nil {
		return PutModelResult{}, availabilityReadError(err)
	}
	if err := tx.Commit(); err != nil {
		return PutModelResult{}, availabilityReadError(err)
	}
	return result, nil
}

func validModelString(value string) bool {
	return validBoundedString(value, maxModelStringRunes)
}

func validCatalogOrigin(origin gen.ModelCatalogOrigin) bool {
	switch origin {
	case gen.Bundled, gen.Remote, gen.Manual:
		return true
	default:
		return false
	}
}

func validCapability(capability string) bool {
	switch capability {
	case "chat", "vision", "structured", "reasoning", "embedding", "rerank", "translation":
		return true
	default:
		return false
	}
}

func embeddingSpaceID(model Model) (platform.Hash, error) {
	canonical := struct {
		Provider             string `json:"provider"`
		ModelVersion         string `json:"modelVersion"`
		Dimensions           int    `json:"dimensions"`
		PreprocessingVersion string `json:"preprocessingVersion"`
	}{string(model.ProviderId), model.ProviderModelVersion, model.Dimension, model.PreprocessorVersion}
	return hashJSON(canonical)
}

func readModelReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, key string, inputHash platform.Hash, expected Model) (PutModelResult, bool, error) {
	var storedHash, createdAt string
	var resultJSON []byte
	err := tx.QueryRowContext(ctx, `SELECT input_hash, result_json, created_at FROM operation_receipts
		WHERE site_id=? AND actor_id=? AND function_id=? AND idempotency_key=?`,
		op.Actor.SiteID, op.Actor.SubjectID, modelFunctionID, key).Scan(&storedHash, &resultJSON, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PutModelResult{}, false, nil
	}
	if err != nil {
		return PutModelResult{}, false, availabilityReadError(err)
	}
	created, err := platform.ParseTimestamp(createdAt)
	now := time.Now().UTC()
	if err != nil || created.After(now) {
		return PutModelResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "模型回执时间损坏")
	}
	if now.Sub(created) > 7*24*time.Hour {
		return PutModelResult{}, false, nil
	}
	if storedHash != inputHash {
		return PutModelResult{}, false, platform.NewError(platform.CodeConflict, false, "幂等键已绑定不同模型输入或代次")
	}
	expectedJSON, _ := json.Marshal(PutModelResult{Model: expected})
	if !bytes.Equal(resultJSON, expectedJSON) {
		return PutModelResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "模型原结果与摘要不一致")
	}
	return PutModelResult{Model: expected}, true, nil
}
