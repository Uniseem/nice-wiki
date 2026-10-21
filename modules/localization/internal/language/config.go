package language

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
)

// PutLanguageInput 与生成类型一致。
type PutLanguageInput = gen.PutLanguageInput

// PutLanguageResult 与生成类型一致。
type PutLanguageResult = gen.PutLanguageResult

// SetLanguageEnabledInput 与生成类型一致。
type SetLanguageEnabledInput = gen.SetLanguageEnabledInput

// SetLanguageEnabledResult 与生成类型一致。
type SetLanguageEnabledResult = gen.SetLanguageEnabledResult

// PutLanguage 按原文保存语言配置。它不改写标签，不探测模型，也不创建翻译任务。
func PutLanguage(ctx context.Context, op OperationContext, input PutLanguageInput) (PutLanguageResult, error) {
	if op.Actor.Kind != "admin" {
		return PutLanguageResult{}, platform.NewError(platform.CodeForbidden, false, "缺少语言配置权限")
	}
	if err := requireIdempotencyKey(input.IdempotencyKey); err != nil {
		return PutLanguageResult{}, err
	}
	if err := acceptStoredTag(input.Language.Tag, input.Language.Direction); err != nil {
		return PutLanguageResult{}, err
	}
	if input.Language.IsDefault && !input.Language.Enabled {
		return PutLanguageResult{}, platform.NewError(platform.CodeValidation, false, "默认语言不能停用")
	}
	if err := requireEnglishKept(input.Language.Tag, input.Language.IsDefault, input.Language.Enabled, input.Language.AutomaticTranslation); err != nil {
		return PutLanguageResult{}, err
	}
	if input.Language.Version != input.ExpectedVersion+1 {
		return PutLanguageResult{}, platform.NewError(platform.CodeVersionMismatch, false, "语言版本必须为预期版本加一")
	}
	if input.Language.ProfileId != nil && !platform.ValidIdentifier(string(*input.Language.ProfileId)) {
		return PutLanguageResult{}, platform.NewError(platform.CodeValidation, false, "翻译档位标识不合法")
	}
	tx, err := openLanguage(ctx, op)
	if err != nil {
		return PutLanguageResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	inputHash, err := digestInput(struct {
		CallerService   string              `json:"callerService"`
		DataEpoch       platform.Identifier `json:"dataEpoch"`
		Language        gen.Language        `json:"language"`
		ExpectedVersion int                 `json:"expectedVersion"`
	}{op.CallerService, op.DataEpoch, input.Language, input.ExpectedVersion})
	if err != nil {
		return PutLanguageResult{}, err
	}
	if raw, found, err := readReceipt(ctx, tx, op, putLanguageFunctionID, input.IdempotencyKey, inputHash); err != nil {
		return PutLanguageResult{}, err
	} else if found {
		var result PutLanguageResult
		if json.Unmarshal(raw, &result) != nil {
			return PutLanguageResult{}, platform.NewError(platform.CodeInternal, false, "语言配置原回执损坏")
		}
		return result, nil
	}
	if err := rejectHeldBarrier(ctx, tx); err != nil {
		return PutLanguageResult{}, err
	}
	if err := rejectLanguageAlias(ctx, tx, input.Language.Tag); err != nil {
		return PutLanguageResult{}, err
	}
	if err := rejectOtherDefault(ctx, tx, input.Language.Tag, input.Language.IsDefault); err != nil {
		return PutLanguageResult{}, err
	}
	_, epoch, found, err := loadLanguage(ctx, tx, input.Language.Tag)
	if err != nil {
		return PutLanguageResult{}, err
	}
	now := string(platform.Now())
	if !found {
		if input.ExpectedVersion != 0 {
			return PutLanguageResult{}, platform.NewError(platform.CodeVersionMismatch, false, "语言配置尚未创建")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO languages (
			language_tag, version, enabled, default_language, translation_profile, automatic_translation,
			direction, reflection_enabled, batch_output_tokens, translation_memory_minimum_characters,
			data_epoch, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			input.Language.Tag, input.Language.Version, boolInt(input.Language.Enabled), boolInt(input.Language.IsDefault),
			profileValue(input.Language.ProfileId), boolInt(input.Language.AutomaticTranslation), string(input.Language.Direction),
			boolInt(input.Language.ReflectionEnabled), input.Language.BatchOutputTokens, input.Language.TranslationMemoryMinimumCharacters,
			string(op.DataEpoch), now, now); err != nil {
			return PutLanguageResult{}, mapWriteErr(err)
		}
	} else if epoch != string(op.DataEpoch) {
		return PutLanguageResult{}, platform.NewError(platform.CodeVersionMismatch, false, "语言代次已变化")
	} else if _, err = updateLanguage(ctx, tx, input, now, op.DataEpoch); err != nil {
		return PutLanguageResult{}, err
	}
	if err := enqueueLanguageChanged(tx, input.Language.Tag, input.Language.Version, input.Language.Enabled, op.DataEpoch); err != nil {
		return PutLanguageResult{}, err
	}
	saved := PutLanguageResult{Language: input.Language}
	if err := writeReceipt(ctx, tx, op, putLanguageFunctionID, input.IdempotencyKey, inputHash, saved); err != nil {
		return PutLanguageResult{}, err
	}
	if err := finishTx(tx); err != nil {
		return PutLanguageResult{}, err
	}
	return saved, nil
}

func updateLanguage(ctx context.Context, tx *sql.Tx, input PutLanguageInput, now string, epoch platform.Identifier) (sql.Result, error) {
	result, err := tx.ExecContext(ctx, `UPDATE languages SET version = ?, enabled = ?, default_language = ?,
		translation_profile = ?, automatic_translation = ?, direction = ?, reflection_enabled = ?,
		batch_output_tokens = ?, translation_memory_minimum_characters = ?, updated_at = ?
		WHERE language_tag = ? AND version = ? AND data_epoch = ?`,
		input.Language.Version, boolInt(input.Language.Enabled), boolInt(input.Language.IsDefault),
		profileValue(input.Language.ProfileId), boolInt(input.Language.AutomaticTranslation), string(input.Language.Direction),
		boolInt(input.Language.ReflectionEnabled), input.Language.BatchOutputTokens, input.Language.TranslationMemoryMinimumCharacters,
		now, input.Language.Tag, input.ExpectedVersion, string(epoch))
	if err != nil {
		return nil, mapWriteErr(err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return nil, mapWriteErr(err)
	}
	if changed != 1 {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "语言版本已变化")
	}
	return result, nil
}

// SetLanguageEnabled 启停已有语言。直接启用不会打开自动翻译，也不删除人工译文。
func SetLanguageEnabled(ctx context.Context, op OperationContext, input SetLanguageEnabledInput) (SetLanguageEnabledResult, error) {
	if op.Actor.Kind != "admin" {
		return SetLanguageEnabledResult{}, platform.NewError(platform.CodeForbidden, false, "缺少语言配置权限")
	}
	if err := requireIdempotencyKey(input.IdempotencyKey); err != nil {
		return SetLanguageEnabledResult{}, err
	}
	if input.Tag == "" {
		return SetLanguageEnabledResult{}, platform.NewError(platform.CodeValidation, false, "语言标签为空")
	}
	tx, err := openLanguage(ctx, op)
	if err != nil {
		return SetLanguageEnabledResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	inputHash, err := digestInput(struct {
		CallerService   string              `json:"callerService"`
		DataEpoch       platform.Identifier `json:"dataEpoch"`
		Tag             string              `json:"tag"`
		Enabled         bool                `json:"enabled"`
		ExpectedVersion int                 `json:"expectedVersion"`
	}{op.CallerService, op.DataEpoch, input.Tag, input.Enabled, input.ExpectedVersion})
	if err != nil {
		return SetLanguageEnabledResult{}, err
	}
	if raw, found, err := readReceipt(ctx, tx, op, setLanguageEnabledFunctionID, input.IdempotencyKey, inputHash); err != nil {
		return SetLanguageEnabledResult{}, err
	} else if found {
		var result SetLanguageEnabledResult
		if json.Unmarshal(raw, &result) != nil {
			return SetLanguageEnabledResult{}, platform.NewError(platform.CodeInternal, false, "语言启停原回执损坏")
		}
		return result, nil
	}
	if err := rejectHeldBarrier(ctx, tx); err != nil {
		return SetLanguageEnabledResult{}, err
	}
	existing, epoch, found, err := loadLanguage(ctx, tx, input.Tag)
	if err != nil {
		return SetLanguageEnabledResult{}, err
	}
	if !found {
		return SetLanguageEnabledResult{}, platform.NewError(platform.CodeConflict, false, "语言配置不存在")
	}
	if epoch != string(op.DataEpoch) || existing.Version != input.ExpectedVersion {
		return SetLanguageEnabledResult{}, platform.NewError(platform.CodeVersionMismatch, false, "语言版本或代次已变化")
	}
	if !input.Enabled && existing.IsDefault {
		return SetLanguageEnabledResult{}, platform.NewError(platform.CodeConflict, false, "默认语言不能停用")
	}
	if err := requireEnglishKept(existing.Tag, existing.IsDefault, input.Enabled, true); err != nil {
		return SetLanguageEnabledResult{}, err
	}
	if input.Enabled && existing.AutomaticTranslation {
		return SetLanguageEnabledResult{}, platform.NewError(platform.CodeConflict, false, "直接启停不能启用自动翻译")
	}
	if !input.Enabled {
		// 只结束本域未完成任务。不删除 translation_units，也不调用模型或 jobs。
		if _, err := tx.ExecContext(ctx, `UPDATE translation_jobs SET state = 'cancelled', updated_at = ?
			WHERE target_language = ? AND data_epoch = ? AND state IN ('queued', 'running', 'review')`,
			string(platform.Now()), input.Tag, string(op.DataEpoch)); err != nil {
			return SetLanguageEnabledResult{}, mapWriteErr(err)
		}
	}
	now := string(platform.Now())
	result, err := tx.ExecContext(ctx, `UPDATE languages SET enabled = ?, version = ?, updated_at = ?
		WHERE language_tag = ? AND version = ? AND data_epoch = ?`,
		boolInt(input.Enabled), input.ExpectedVersion+1, now, input.Tag, input.ExpectedVersion, string(op.DataEpoch))
	if err != nil {
		return SetLanguageEnabledResult{}, mapWriteErr(err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return SetLanguageEnabledResult{}, mapWriteErr(err)
	}
	if changed != 1 {
		return SetLanguageEnabledResult{}, platform.NewError(platform.CodeVersionMismatch, false, "语言版本已变化")
	}
	existing.Enabled = input.Enabled
	existing.Version = input.ExpectedVersion + 1
	if err := enqueueLanguageChanged(tx, existing.Tag, existing.Version, existing.Enabled, op.DataEpoch); err != nil {
		return SetLanguageEnabledResult{}, err
	}
	saved := SetLanguageEnabledResult{Language: existing}
	if err := writeReceipt(ctx, tx, op, setLanguageEnabledFunctionID, input.IdempotencyKey, inputHash, saved); err != nil {
		return SetLanguageEnabledResult{}, err
	}
	if err := finishTx(tx); err != nil {
		return SetLanguageEnabledResult{}, err
	}
	return saved, nil
}

func rejectOtherDefault(ctx context.Context, tx *sql.Tx, tag string, isDefault bool) error {
	if !isDefault {
		return nil
	}
	var other string
	err := tx.QueryRowContext(ctx, `SELECT language_tag FROM languages WHERE default_language = 1 AND language_tag <> ?`, tag).Scan(&other)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "默认语言不可读")
	}
	return platform.NewError(platform.CodeConflict, false, "已存在其他默认语言")
}

// rejectLanguageAlias 保留原文标签显示，但不允许两个配置代表同一规范语言。
func rejectLanguageAlias(ctx context.Context, tx *sql.Tx, tag string) error {
	wanted, err := NormalizeLanguageTag(ctx, OperationContext{}, NormalizeLanguageTagInput{Tag: tag})
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT language_tag FROM languages WHERE language_tag<>?`, tag)
	if err != nil {
		return mapWriteErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var other string
		if err = rows.Scan(&other); err != nil {
			return mapWriteErr(err)
		}
		normalized, err := NormalizeLanguageTag(ctx, OperationContext{}, NormalizeLanguageTagInput{Tag: other})
		if err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "现有语言标签不能规范解析")
		}
		if normalized.Tag == wanted.Tag {
			return platform.NewError(platform.CodeConflict, false, "规范语言已存在其他原文标签配置")
		}
	}
	return mapWriteErr(rows.Err())
}
