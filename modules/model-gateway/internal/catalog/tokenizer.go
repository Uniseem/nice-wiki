package catalog

import (
 "context"
 "database/sql"
 "errors"

 "nice-wiki/libs/platform"
 gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

// ReadTokenizerModel 只读取已启用的同代嵌入模型与其不可变预处理声明。
func ReadTokenizerModel(ctx context.Context, op OperationContext, modelID string) (Model, error) {
 store := gatewayruntime.CurrentStore()
 if store == nil { return Model{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型目录尚未装配") }
 var model Model
 var modelEpoch, currentEpoch string
 err := store.DB().QueryRowContext(ctx, `SELECT m.model_id, m.provider_id, m.provider_model_version, m.dimension, m.preprocessor_version, m.data_epoch, r.data_epoch
 FROM models m CROSS JOIN runtime_meta r WHERE m.model_id=? AND m.enabled=1 AND r.singleton=1`, modelID).Scan(&model.ModelId, &model.ProviderId, &model.ProviderModelVersion, &model.Dimension, &model.PreprocessorVersion, &modelEpoch, &currentEpoch)
 if errors.Is(err, sql.ErrNoRows) { return Model{}, platform.NewError(platform.CodeNotFound, false, "可用嵌入模型不存在") }
 if err != nil { return Model{}, availabilityReadError(err) }
 if modelEpoch != op.DataEpoch || currentEpoch != op.DataEpoch { return Model{}, platform.NewError(platform.CodeVersionMismatch, false, "模型目录代次不一致") }
 if model.Dimension == 0 { return Model{}, platform.NewError(platform.CodeValidation, false, "分词只支持明确的嵌入模型") }
 return model, nil
}
