package translation

import (
	"context"
	"database/sql"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

// AutoTranslate 给开了“自动翻译”的语言补译：公开树里每篇文档的当前发布修订，只要这门语言还没有它的译文就创建一份。
// 管理员打开自动翻译即视为批准这类费用；失败过的修订不自动重试，避免反复花钱。每轮最多开 3 份。
func AutoTranslate(ctx context.Context, op OperationContext) error {
	var languages []string
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT language_tag FROM languages WHERE enabled=1 AND default_language=0 AND automatic_translation=1 AND data_epoch=? ORDER BY language_tag`, op.DataEpoch)
		if err != nil {
			return language.StorageError(err)
		}
		defer rows.Close()
		for rows.Next() {
			var tag string
			if err := rows.Scan(&tag); err != nil {
				return language.StorageError(err)
			}
			languages = append(languages, tag)
		}
		return language.StorageError(rows.Err())
	})
	if err != nil || len(languages) == 0 {
		return err
	}
	tree, err := localizationruntime.ReadPublicTree(ctx, op)
	if err != nil {
		return err
	}
	started := 0
	for _, document := range tree.Documents {
		for _, tag := range languages {
			if started >= 3 {
				return nil
			}
			var exists int
			err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
				return language.StorageError(tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM translation_manifests WHERE target_language=? AND data_epoch=? AND json_extract(publication_ref,'$.documentId')=? AND json_extract(publication_ref,'$.publishedRevisionId')=? AND state NOT IN('cancelled','superseded')`, tag, op.DataEpoch, document.DocumentID, document.Reference.PublishedRevisionId).Scan(&exists))
			})
			if err != nil {
				return err
			}
			if exists > 0 {
				continue
			}
			if _, err := createTranslation(ctx, op, gen.CreateTranslationInput{Document: document.Reference, TargetLanguage: tag, IdempotencyKey: "auto." + platform.HashBytes([]byte(tag+":"+document.Reference.PublishedRevisionId))}); err != nil {
				return err
			}
			started++
		}
	}
	return nil
}
