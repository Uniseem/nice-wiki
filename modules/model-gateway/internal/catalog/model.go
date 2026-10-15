package catalog

import (
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
	operationCtx, cancel, err := operationContext(ctx, op, 5*time.Second)
	if err != nil {
		return PutModelResult{}, err
	}
	defer cancel()
	if err := requireSuperadmin(op); err != nil {
		return PutModelResult{}, err
	}
	validation, err := ValidateModel(operationCtx, op, ValidateModelInput{Model: input.Model})
	if err != nil {
		return PutModelResult{}, err
	}
	if input.ExpectedVersion < 0 || input.IdempotencyKey == "" || !validBoundedString(input.IdempotencyKey, 65536) {
		return PutModelResult{}, platform.NewError(platform.CodeValidation, false, "模型保存幂等参数不合法")
	}
	if input.Model.Version != input.ExpectedVersion+1 {
		return PutModelResult{}, platform.NewError(platform.CodeVersionMismatch, false, "模型版本必须为预期版本加一")
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return PutModelResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型网关数据库尚未装配")
	}
	if err := store.CheckDataEpoch(operationCtx, op.DataEpoch); err != nil {
		return PutModelResult{}, err
	}
	tx, err := store.BeginTx(operationCtx)
	if err != nil {
		return PutModelResult{}, err
	}
	rollback := func(cause error) (PutModelResult, error) {
		_ = tx.Rollback()
		return PutModelResult{}, cause
	}
	if err := rejectDrainingBarrier(operationCtx, tx, op.DataEpoch); err != nil {
		return rollback(err)
	}
	inputHash, err := hashJSON(struct {
		Model           Model `json:"model"`
		ExpectedVersion int   `json:"expectedVersion"`
	}{input.Model, input.ExpectedVersion})
	if err != nil {
		return rollback(platform.NewError(platform.CodeInternal, false, "模型输入摘要计算失败"))
	}
	if result, found, err := readModelReceipt(operationCtx, tx, op, input.IdempotencyKey, inputHash); err != nil {
		return rollback(err)
	} else if found {
		if err := tx.Commit(); err != nil {
			return PutModelResult{}, platform.NewError(platform.CodeInternal, true, "提交模型幂等读取失败")
		}
		return result, nil
	}
	var providerExists int
	if err := tx.QueryRowContext(operationCtx, `SELECT COUNT(*) FROM providers WHERE provider_id = ?`, string(input.Model.ProviderId)).Scan(&providerExists); err != nil {
		return rollback(platform.NewError(platform.CodeInternal, true, "读取模型供应方失败"))
	}
	if providerExists != 1 {
		return rollback(platform.NewError(platform.CodeNotFound, false, "模型供应方不存在"))
	}
	var currentVersion int
	err = tx.QueryRowContext(operationCtx, `SELECT version FROM models WHERE model_id = ?`, string(input.Model.ModelId)).Scan(&currentVersion)
	switch {
	case errors.Is(err, sql.ErrNoRows) && input.ExpectedVersion != 0:
		return rollback(platform.NewError(platform.CodeNotFound, false, "模型不存在"))
	case err == nil && currentVersion != input.ExpectedVersion:
		return rollback(platform.NewError(platform.CodeVersionMismatch, false, "模型版本已变化"))
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return rollback(platform.NewError(platform.CodeInternal, true, "读取模型版本失败"))
	}

	model := input.Model
	capabilities, _ := json.Marshal(model.Capabilities)
	if _, err := tx.ExecContext(operationCtx, `
		INSERT INTO models (
			model_id, version, provider_id, provider_model_version, capabilities_json,
			dimension, preprocessor_version, max_input_tokens, max_output_tokens,
			catalog_origin, manual_override, enabled, data_epoch, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(model_id) DO UPDATE SET
			version = excluded.version, provider_id = excluded.provider_id,
			provider_model_version = excluded.provider_model_version,
			capabilities_json = excluded.capabilities_json, dimension = excluded.dimension,
			preprocessor_version = excluded.preprocessor_version, max_input_tokens = excluded.max_input_tokens,
			max_output_tokens = excluded.max_output_tokens, catalog_origin = excluded.catalog_origin,
			manual_override = excluded.manual_override, enabled = excluded.enabled,
			data_epoch = excluded.data_epoch, updated_at = excluded.updated_at`,
		string(model.ModelId), model.Version, string(model.ProviderId), model.ProviderModelVersion,
		capabilities, model.Dimension, model.PreprocessorVersion, model.MaxInputTokens,
		model.MaxOutputTokens, string(model.CatalogOrigin), boolInt(model.ManualOverride),
		boolInt(model.Enabled), string(op.DataEpoch), string(platform.Now())); err != nil {
		return rollback(platform.NewError(platform.CodeInternal, true, "保存模型配置失败"))
	}
	payload := struct {
		ModelID   string          `json:"modelId"`
		Version   int             `json:"version"`
		Space     *EmbeddingSpace `json:"space,omitempty"`
		DataEpoch string          `json:"dataEpoch"`
	}{string(model.ModelId), model.Version, validation.EmbeddingSpace, string(op.DataEpoch)}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return rollback(platform.NewError(platform.CodeInternal, false, "模型事件编码失败"))
	}
	now := string(platform.Now())
	if err := outbox.EnqueueEvent(tx, platform.EventEnvelope{
		EventID:      platform.NewIdentifier(),
		EventType:    modelEventType,
		EventVersion: modelEventVersion,
		Producer:     "model-gateway",
		SubjectID:    platform.Identifier(model.ModelId),
		Revision:     platform.Revision(model.Version),
		OccurredAt:   platform.Timestamp(now),
		DataEpoch:    op.DataEpoch,
		Payload:      payloadJSON,
	}); err != nil {
		return rollback(platform.NewError(platform.CodeInternal, true, "保存模型事件失败"))
	}
	result := PutModelResult{Model: model}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return rollback(platform.NewError(platform.CodeInternal, false, "模型回执编码失败"))
	}
	if _, err := tx.ExecContext(operationCtx, `
		INSERT INTO operation_receipts
			(site_id, actor_id, function_id, idempotency_key, input_hash, result_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, string(op.Actor.SiteID), string(op.Actor.SubjectID), modelFunctionID,
		input.IdempotencyKey, inputHash, resultJSON, now); err != nil {
		return rollback(platform.NewError(platform.CodeInternal, true, "保存模型幂等回执失败"))
	}
	if err := tx.Commit(); err != nil {
		return PutModelResult{}, platform.NewError(platform.CodeInternal, true, "提交模型配置失败")
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

func readModelReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, key string, inputHash platform.Hash) (PutModelResult, bool, error) {
	var storedHash string
	var resultJSON []byte
	err := tx.QueryRowContext(ctx, `
		SELECT input_hash, result_json FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND function_id = ? AND idempotency_key = ?`,
		string(op.Actor.SiteID), string(op.Actor.SubjectID), modelFunctionID, key).Scan(&storedHash, &resultJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return PutModelResult{}, false, nil
	}
	if err != nil {
		return PutModelResult{}, false, platform.NewError(platform.CodeInternal, true, "读取模型幂等回执失败")
	}
	if storedHash != string(inputHash) {
		return PutModelResult{}, false, platform.NewError(platform.CodeConflict, false, "幂等键已绑定不同模型配置")
	}
	var result PutModelResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return PutModelResult{}, false, platform.NewError(platform.CodeIntegrityError, false, "模型幂等回执损坏")
	}
	return result, true, nil
}
