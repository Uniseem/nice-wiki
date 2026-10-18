// Package config 管理入库参数，不以换配置自动触发付费重建。
package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/admin"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
	"slices"
)

type OperationContext = platform.OperationContext
type PutIndexConfigInput = gen.PutIndexConfigInput
type PutIndexConfigResult = gen.PutIndexConfigResult
type ReadIndexConfigInput struct{}
type ReadIndexConfigResult = gen.ReadIndexConfigResult

// PutIndexConfig 即时入口已核knowledge.manage；只保存已核MG声明及新版本，不改active。
func PutIndexConfig(ctx context.Context, op OperationContext, input PutIndexConfigInput) (PutIndexConfigResult, error) {
	if err := admin.AuthorizeAdmin(op); err != nil {
		return PutIndexConfigResult{}, err
	}
	if input.Config.OverlapTokens >= input.Config.ChunkTokens || input.Config.Version != input.ExpectedVersion {
		return PutIndexConfigResult{}, platform.NewError(platform.CodeValidation, false, "分块重叠或提交版本不合法")
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return PutIndexConfigResult{}, err
	}
	if runtime.BaseLanguage == "" {
		return PutIndexConfigResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "站点基准语言未装配")
	}
	if input.Config.Language != runtime.BaseLanguage {
		return PutIndexConfigResult{}, platform.NewError(platform.CodeValidation, false, "只允许站点基准语言入库")
	}
	// 原回执可重放，不因之后模型停用而把已提交配置操作改成失败。
	read, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return PutIndexConfigResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "配置原回执读取失败")
	}
	var original PutIndexConfigResult
	found, err := indexer.ReadOperationReceipt(ctx, read, op, "PutIndexConfig", input.IdempotencyKey, input, &original)
	_ = read.Rollback()
	if err != nil {
		return PutIndexConfigResult{}, err
	}
	if found {
		return original, nil
	}
	if input.Config.Enabled {
		if err = validateConfiguredSpace(ctx, op, runtime, input.Config); err != nil {
			return PutIndexConfigResult{}, err
		}
	}
	work, tx, cancel, err := indexer.BeginIndexWrite(ctx, op)
	if err != nil {
		return PutIndexConfigResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	found, err = indexer.ReadOperationReceipt(work, tx, op, "PutIndexConfig", input.IdempotencyKey, input, &original)
	if err != nil {
		return PutIndexConfigResult{}, err
	}
	if found {
		return original, nil
	}
	var version int
	var id string
	err = tx.QueryRowContext(work, `SELECT version,config_id FROM index_config WHERE site_id=? AND data_epoch=?`, op.Actor.SiteID, op.DataEpoch).Scan(&version, &id)
	if err != nil && err != sql.ErrNoRows {
		return PutIndexConfigResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "配置水位读取失败")
	}
	if version != input.ExpectedVersion || (id != "" && id != input.Config.ConfigId) || version == 9007199254740991 {
		return PutIndexConfigResult{}, platform.NewError(platform.CodeVersionMismatch, false, "索引配置版本已变化")
	}
	next := input.Config
	next.Version = version + 1
	raw, _ := json.Marshal(next)
	_, err = tx.ExecContext(work, `INSERT INTO index_config(site_id,data_epoch,config_id,version,config_json) VALUES(?,?,?,?,?) ON CONFLICT(site_id) DO UPDATE SET version=excluded.version,config_json=excluded.config_json WHERE index_config.data_epoch=excluded.data_epoch AND index_config.version=?`, op.Actor.SiteID, op.DataEpoch, next.ConfigId, next.Version, raw, version)
	if err != nil {
		return PutIndexConfigResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "配置写入失败")
	}
	result := PutIndexConfigResult{Config: next}
	if err = indexer.SaveOperationReceipt(work, tx, op, "PutIndexConfig", input.IdempotencyKey, input, result); err != nil {
		return PutIndexConfigResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return PutIndexConfigResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "配置提交失败")
	}
	return result, nil
}

func ReadIndexConfig(ctx context.Context, op OperationContext, input ReadIndexConfigInput) (ReadIndexConfigResult, error) {
	if err := admin.AuthorizeAdmin(op); err != nil {
		return ReadIndexConfigResult{}, err
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return ReadIndexConfigResult{}, err
	}
	tx, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ReadIndexConfigResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "配置读取失败")
	}
	defer tx.Rollback()
	config, err := indexer.ReadConfig(ctx, tx, op)
	if err != nil {
		return ReadIndexConfigResult{}, err
	}
	return ReadIndexConfigResult{Config: config, Version: config.Version}, nil
}

// validateConfiguredSpace 核固定档位的主模型与完整空间，不能只比较模型名或维度。
func validateConfiguredSpace(ctx context.Context, op OperationContext, runtime indexer.Runtime, config gen.IndexConfig) error {
	catalog, err := indexer.ReadModelCatalog(ctx, op, runtime)
	if err != nil {
		return err
	}
	_, model, err := indexer.EmbeddingProfile(catalog, config.EmbeddingProfileId, platform.CostClassBackground, config.Space)
	if err != nil {
		return platform.NewError(platform.CodeValidation, false, "嵌入档位与当前完整空间不兼容")
	}
	switch config.Space.PreprocessingVersion {
	case "none;tiktoken-go@v0.7.0;cl100k_base", "none;tiktoken-go@v0.7.0;o200k_base":
	default:
		return platform.NewError(platform.CodeUnsupportedSchema, false, "嵌入模型未声明本域支持的真实分词器")
	}
	if !slices.Contains(model.Capabilities, "embedding") || model.MaximumInputTokens < config.ChunkTokens {
		return platform.NewError(platform.CodeValidation, false, "嵌入主模型能力或输入预算不兼容")
	}
	if config.RerankProfileId != nil {
		found := false
		for _, profile := range catalog.Profiles {
			if profile.ProfileID != *config.RerankProfileId || profile.CostClass != platform.CostClassQA || profile.Tier != "rerank" || len(profile.Models) == 0 {
				continue
			}
			for _, candidate := range catalog.Models {
				if candidate.ModelID == profile.Models[0] && candidate.Enabled && slices.Contains(candidate.Capabilities, "rerank") {
					found = true
				}
			}
		}
		if !found {
			return platform.NewError(platform.CodeValidation, false, "重排档位没有兼容主模型")
		}
	}
	return nil
}
