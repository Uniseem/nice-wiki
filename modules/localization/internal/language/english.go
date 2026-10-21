package language

import (
	"context"
	"database/sql"
	"errors"

	"nice-wiki/libs/platform"
)

// requireEnglishKept 是站点规则：基准语言之外的英文必须启用并打开“发布时自动翻译”。
func requireEnglishKept(tag string, isDefault, enabled, automatic bool) error {
	if platform.IsEnglishTag(tag) && !isDefault && (!enabled || !automatic) {
		return platform.NewError(platform.CodeConflict, false, "英文是站点必需语言，必须启用并自动翻译")
	}
	return nil
}

// EnsureEnglish 保证英文始终是站点语言：基准语言不是英文时，英文必须启用并打开“发布时自动翻译”（决策模型只读英文）。
// 还没设置基准语言时什么也不做；已经满足时不写库。
func EnsureEnglish(ctx context.Context, op OperationContext) error {
	tx, err := openLanguage(ctx, op)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var base string
	err = tx.QueryRowContext(ctx, `SELECT language_tag FROM languages WHERE default_language=1 AND data_epoch=?`, string(op.DataEpoch)).Scan(&base)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "基准语言不可读")
	}
	if platform.IsEnglishTag(base) {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT language_tag FROM languages WHERE data_epoch=?`, string(op.DataEpoch))
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "语言列表不可读")
	}
	tag := ""
	for rows.Next() {
		var current string
		if err := rows.Scan(&current); err != nil {
			rows.Close()
			return platform.NormalizeIOError(err, platform.CodeInternal, false, "语言列表不可读")
		}
		if platform.IsEnglishTag(current) {
			tag = current
		}
	}
	rows.Close()
	now := string(platform.Now())
	version := 1
	if tag == "" {
		tag = "en"
		if _, err = tx.ExecContext(ctx, `INSERT INTO languages (language_tag, version, enabled, default_language, translation_profile, automatic_translation,
			direction, reflection_enabled, batch_output_tokens, translation_memory_minimum_characters, data_epoch, created_at, updated_at)
			VALUES (?, 1, 1, 0, NULL, 1, 'ltr', 0, 1000, 20, ?, ?, ?)`, tag, string(op.DataEpoch), now, now); err != nil {
			return mapWriteErr(err)
		}
	} else {
		existing, _, _, err := loadLanguage(ctx, tx, tag)
		if err != nil {
			return err
		}
		if existing.Enabled && existing.AutomaticTranslation {
			return nil
		}
		version = existing.Version + 1
		if _, err = tx.ExecContext(ctx, `UPDATE languages SET enabled = 1, automatic_translation = 1, version = ?, updated_at = ?
			WHERE language_tag = ? AND version = ? AND data_epoch = ?`, version, now, tag, existing.Version, string(op.DataEpoch)); err != nil {
			return mapWriteErr(err)
		}
	}
	if err := enqueueLanguageChanged(tx, tag, version, true, op.DataEpoch); err != nil {
		return err
	}
	return finishTx(tx)
}

// EnabledEnglish 返回站点启用的英文标签（en、en-US 等）；没有启用英文时返回 NOT_FOUND。
func EnabledEnglish(ctx context.Context, tx *sql.Tx, op OperationContext) (string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT language_tag FROM languages WHERE enabled=1 AND data_epoch=? ORDER BY language_tag`, string(op.DataEpoch))
	if err != nil {
		return "", StorageError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return "", StorageError(err)
		}
		if platform.IsEnglishTag(tag) {
			return tag, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", StorageError(err)
	}
	return "", platform.NewError(platform.CodeNotFound, false, "站点没有启用英文")
}
