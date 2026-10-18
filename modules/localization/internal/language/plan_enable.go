package language

import (
	"context"
	"database/sql"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
)

// EnableLanguageForPlan 在批准事务内把预期代次的语言切换为自动翻译。
func EnableLanguageForPlan(ctx context.Context, tx *sql.Tx, op OperationContext, tag string, expectedVersion int) (gen.Language, error) {
	existing, epoch, found, err := loadLanguage(ctx, tx, tag)
	if err != nil {
		return gen.Language{}, err
	}
	if !found {
		return gen.Language{}, platform.NewError(platform.CodeValidation, false, "语言配置不存在")
	}
	if epoch != string(op.DataEpoch) || existing.Version != expectedVersion {
		return gen.Language{}, platform.NewError(platform.CodeVersionMismatch, false, "批准期间语言版本或代次已变化")
	}
	if existing.AutomaticTranslation {
		return gen.Language{}, platform.NewError(platform.CodeConflict, false, "语言计划已被批准")
	}
	if existing.ProfileId == nil || !platform.ValidIdentifier(string(*existing.ProfileId)) {
		return gen.Language{}, platform.NewError(platform.CodeValidation, false, "语言没有可用翻译档位")
	}
	result, err := tx.ExecContext(ctx, `UPDATE languages SET enabled = 1, automatic_translation = 1,
		version = ?, updated_at = ? WHERE language_tag = ? AND version = ? AND data_epoch = ?`,
		expectedVersion+1, string(platform.Now()), tag, expectedVersion, string(op.DataEpoch))
	if err != nil {
		return gen.Language{}, mapWriteErr(err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return gen.Language{}, mapWriteErr(err)
	}
	if changed != 1 {
		return gen.Language{}, platform.NewError(platform.CodeVersionMismatch, false, "语言计划批准前配置已变化")
	}
	existing.Enabled = true
	existing.AutomaticTranslation = true
	existing.Version = expectedVersion + 1
	if err = enqueueLanguageChanged(tx, existing.Tag, existing.Version, true, op.DataEpoch); err != nil {
		return gen.Language{}, err
	}
	return existing, nil
}
