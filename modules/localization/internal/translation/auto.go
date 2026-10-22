package translation

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

// 同时在翻的译文最多几份（含管理员手动发起的）。
const maximumRunningTranslations = 3

// AutoTranslate 让启用的每门非基准语言自动跟上：公开树里每篇文档的当前发布修订，这门语言还没有可用译文就创建一份。
// 全自动、后台不用管：模型还没配好时先不建（配好后下一轮自动开始）；失败或取消过的修订按退避自动重来。
// 库里还在排队或翻译中、但本进程没在翻的（进程重启时中断的）先接着翻；同时在翻的不超过 3 份。
func AutoTranslate(ctx context.Context, op OperationContext) error {
	var languages, unfinished []string
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		var err error
		if languages, err = queryStrings(ctx, tx, `SELECT language_tag FROM languages WHERE enabled=1 AND default_language=0 AND data_epoch=? ORDER BY language_tag`, op.DataEpoch); err != nil {
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
	routeChecked := false
	// 一篇文档建不了译文（例如正文结构切不出翻译单元）不挡其他文档；第一个错误留给恢复循环记日志。
	var firstErr error
	for _, document := range tree.Documents {
		for _, tag := range languages {
			if running >= maximumRunningTranslations {
				return firstErr
			}
			attempt, due, err := nextAttempt(ctx, op, document.DocumentID, document.Reference.PublishedRevisionId, tag)
			if err != nil {
				return err
			}
			if !due {
				continue
			}
			if !routeChecked {
				// 文档翻译任务还解析不出档位（没配模型）时，建了也只会失败：等配好再建。
				if _, err := localizationruntime.ResolvePlanTaskRoute(ctx, op, "localization.document"); err != nil {
					return nil
				}
				routeChecked = true
			}
			key := "auto." + platform.HashBytes([]byte(tag+":"+document.Reference.PublishedRevisionId+":"+strconv.Itoa(attempt)))
			if _, err := createTranslation(ctx, op, gen.CreateTranslationInput{Document: document.Reference, TargetLanguage: tag, IdempotencyKey: key}); err != nil {
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

// nextAttempt 判断这门语言是否该为这次发布修订（再）建译文：已有可用或在翻的译文就不用；
// 失败或取消过的按次数退避（5 分钟起翻倍，最长 1 小时，换好密钥或模型后不用等太久）。返回第几次尝试，用于幂等键。
func nextAttempt(ctx context.Context, op OperationContext, documentID, revisionID, tag string) (int, bool, error) {
	var live, failed int
	var last sql.NullString
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		return language.StorageError(tx.QueryRowContext(ctx, `SELECT COUNT(CASE WHEN state NOT IN('failed','cancelled') THEN 1 END),COUNT(CASE WHEN state IN('failed','cancelled') THEN 1 END),MAX(CASE WHEN state IN('failed','cancelled') THEN updated_at END) FROM translation_manifests WHERE target_language=? AND data_epoch=? AND json_extract(publication_ref,'$.documentId')=? AND json_extract(publication_ref,'$.publishedRevisionId')=?`, tag, op.DataEpoch, documentID, revisionID).Scan(&live, &failed, &last))
	})
	if err != nil || live > 0 {
		return 0, false, err
	}
	if failed > 0 {
		at, parseErr := time.Parse(time.RFC3339Nano, last.String)
		if parseErr == nil && time.Since(at) < retryDelay(failed) {
			return 0, false, nil
		}
	}
	return failed, true, nil
}

func retryDelay(failures int) time.Duration {
	return min(5*time.Minute<<min(failures-1, 10), time.Hour)
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
