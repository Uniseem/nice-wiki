package language

import (
	"context"
	"database/sql"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
)

// ListLanguagesInput 对应本域 types.json。列表请求没有正文，
// OpenAPI 生成物只有查询参数，没有这个业务输入类型。
type ListLanguagesInput struct {
	IncludeDisabled bool `json:"includeDisabled"`
}

// ListLanguagesResult 与生成类型一致。
type ListLanguagesResult = gen.ListLanguagesResult

// LanguageForTag 在同一只读快照内按规范 BCP47 查找配置并返回展示原文。
func LanguageForTag(ctx context.Context, op OperationContext, tx *sql.Tx, tag string) (gen.Language, error) {
	wanted, err := NormalizeLanguageTag(ctx, OperationContext{}, NormalizeLanguageTagInput{Tag: tag})
	if err != nil {
		return gen.Language{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT language_tag, version, enabled, default_language, translation_profile,
		automatic_translation, direction, reflection_enabled, batch_output_tokens,
		translation_memory_minimum_characters, data_epoch
		FROM languages WHERE data_epoch = ? ORDER BY language_tag`, string(op.DataEpoch))
	if err != nil {
		return gen.Language{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "语言配置不可读")
	}
	defer rows.Close()
	var found gen.Language
	for rows.Next() {
		item, _, scanErr := scanLanguage(rows)
		if scanErr != nil {
			return gen.Language{}, platform.NormalizeIOError(scanErr, platform.CodeInternal, false, "语言配置记录损坏")
		}
		normalized, normalizeErr := NormalizeLanguageTag(ctx, OperationContext{}, NormalizeLanguageTagInput{Tag: item.Tag})
		if normalizeErr != nil {
			return gen.Language{}, platform.NewError(platform.CodeIntegrityError, false, "既有语言配置不是有效BCP47")
		}
		if normalized.Tag == wanted.Tag {
			if found.Tag != "" {
				return gen.Language{}, platform.NewError(platform.CodeIntegrityError, false, "规范语言存在多个原文配置")
			}
			found = item
		}
	}
	if err = rows.Err(); err != nil {
		return gen.Language{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "语言配置不可读")
	}
	if found.Tag == "" {
		return gen.Language{}, platform.NewError(platform.CodeNotFound, false, "语言配置不存在")
	}
	return found, nil
}

// ListLanguages 从本域语言表读取当前代次的配置。
// 缺译不改变启用状态；响应只含语言配置，不含模型密钥。
func ListLanguages(ctx context.Context, op OperationContext, input ListLanguagesInput) (ListLanguagesResult, error) {
	if input.IncludeDisabled {
		if op.CallerService == "public-site" && op.Actor.Kind == "service" {
			if err := requireActor(op); err != nil {
				return ListLanguagesResult{}, err
			}
			return ListLanguagesResult{}, platform.NewError(platform.CodeForbidden, false, "公开站点服务只能读取已启用语言")
		}
		if op.Actor.Kind != "admin" {
			return ListLanguagesResult{}, platform.NewError(platform.CodeForbidden, false, "停用语言列表需要语言管理权限")
		}
	}
	tx, err := openLanguage(ctx, op)
	if err != nil {
		return ListLanguagesResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := rejectHeldBarrier(ctx, tx); err != nil {
		return ListLanguagesResult{}, err
	}
	query := `SELECT language_tag, version, enabled, default_language, translation_profile, automatic_translation,
		direction, reflection_enabled, batch_output_tokens, translation_memory_minimum_characters, data_epoch
		FROM languages WHERE data_epoch = ?`
	args := []any{string(op.DataEpoch)}
	if !input.IncludeDisabled {
		query += ` AND enabled = 1`
	}
	query += ` ORDER BY default_language DESC, language_tag ASC LIMIT ?`
	args = append(args, maxLanguages+1)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return ListLanguagesResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "语言列表不可读")
	}
	defer rows.Close()
	languages := make([]gen.Language, 0)
	for rows.Next() {
		item, _, err := scanLanguage(rows)
		if err != nil {
			return ListLanguagesResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "语言列表记录损坏")
		}
		languages = append(languages, item)
		if len(languages) > maxLanguages {
			return ListLanguagesResult{}, platform.NewError(platform.CodeInternal, false, "语言数量超过登记上限")
		}
	}
	if err := rows.Err(); err != nil {
		return ListLanguagesResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "语言列表不可读")
	}
	return ListLanguagesResult{Languages: languages}, nil
}
