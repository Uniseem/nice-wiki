package translation

import (
	"context"
	"database/sql"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

// 同时在翻的译文最多几份（含管理员手动发起的）。
const maximumRunningTranslations = 3

// AutoTranslate 给开了“自动翻译”的语言补译：公开树里每篇文档的当前发布修订，只要这门语言还没有它的译文就创建一份。
// 管理员打开自动翻译即视为批准这类费用；失败或被取消过的修订不自动重来，避免反复花钱。
// 库里还在排队或翻译中、但本进程没在翻的（进程重启时中断的）先接着翻；同时在翻的不超过 3 份。
func AutoTranslate(ctx context.Context, op OperationContext) error {
	var languages, unfinished []string
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		var err error
		if languages, err = queryStrings(ctx, tx, `SELECT language_tag FROM languages WHERE enabled=1 AND default_language=0 AND automatic_translation=1 AND data_epoch=? ORDER BY language_tag`, op.DataEpoch); err != nil {
			return err
		}
		unfinished, err = queryStrings(ctx, tx, `SELECT translation_id FROM translation_manifests WHERE state IN('queued','running') AND data_epoch=? ORDER BY updated_at`, op.DataEpoch)
		return err
	})
	if err != nil {
		return err
	}
	running := 0
	for _, id := range unfinished {
		if running >= maximumRunningTranslations {
			return nil
		}
		if _, busy := executing.Load(id); !busy {
			go ExecuteTranslation(context.Background(), executionOperation(op, id), id)
		}
		running++
	}
	if len(languages) == 0 || running >= maximumRunningTranslations {
		return nil
	}
	tree, err := localizationruntime.ReadPublicTree(ctx, op)
	if err != nil {
		return err
	}
	// 一篇文档建不了译文（例如正文结构切不出翻译单元）不挡其他文档；第一个错误留给恢复循环记日志。
	var firstErr error
	for _, document := range tree.Documents {
		for _, tag := range languages {
			if running >= maximumRunningTranslations {
				return firstErr
			}
			var exists int
			err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
				return language.StorageError(tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM translation_manifests WHERE target_language=? AND data_epoch=? AND json_extract(publication_ref,'$.documentId')=? AND json_extract(publication_ref,'$.publishedRevisionId')=?`, tag, op.DataEpoch, document.DocumentID, document.Reference.PublishedRevisionId).Scan(&exists))
			})
			if err != nil {
				return err
			}
			if exists > 0 {
				continue
			}
			if _, err := createTranslation(ctx, op, gen.CreateTranslationInput{Document: document.Reference, TargetLanguage: tag, IdempotencyKey: "auto." + platform.HashBytes([]byte(tag+":"+document.Reference.PublishedRevisionId))}); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			running++
		}
	}
	return firstErr
}

func queryStrings(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, language.StorageError(err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, language.StorageError(err)
		}
		values = append(values, value)
	}
	return values, language.StorageError(rows.Err())
}
