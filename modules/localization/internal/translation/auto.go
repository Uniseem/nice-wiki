package translation

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

// 同时在翻的译文最多几份（含管理员手动发起的）。
const maximumRunningTranslations = 3

// AutoTranslate 让启用的每门非基准语言自动跟上：公开树里每篇文档的当前发布修订，这门语言还没有可用译文就创建一份。
// 全自动、后台不用管：模型还没配好时先不建（配好后下一轮自动开始）；失败的修订按退避自动重来，见 nextAttempt。
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
	if len(unfinished) == 0 && len(languages) == 0 {
		return nil
	}
	// 先确认正文（content）和模型网关都能用了再开始或接着翻：进程刚重启时它们可能还在启动，
	// 这时接着翻会立刻失败，并按失败次数退避，最长要等一小时才重来。没配好模型时同样先不动。
	tree, err := localizationruntime.ReadPublicTree(ctx, op)
	if err != nil {
		return err
	}
	if _, err := localizationruntime.ResolvePlanTaskRoute(ctx, op, "localization.document"); err != nil {
		return nil
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
	// 搜索摘要补译：早期译文没有翻译摘要（公开页的 meta description 还是原文）。补译时正文单元沿用上一份译文，
	// 只把标题和摘要交给模型；本进程内每个（发布修订, 语言）最多补一次，补不成也不反复重来。
	for _, document := range tree.Documents {
		for _, tag := range languages {
			if running >= maximumRunningTranslations {
				return firstErr
			}
			key := tag + ":" + document.Reference.PublishedRevisionId
			if _, seen := describedOnce.LoadOrStore(key, true); seen {
				continue
			}
			if missing, err := missingDescription(ctx, op, document, tag); err != nil || !missing {
				continue
			}
			if _, err := createTranslation(ctx, op, gen.CreateTranslationInput{Document: document.Reference, TargetLanguage: tag, IdempotencyKey: "describe." + platform.HashBytes([]byte(key))}); err == nil {
				running++
			}
		}
	}
	return firstErr
}

var describedOnce sync.Map

// missingDescription 判断这门语言当前修订的已完成译文是否缺摘要译文，而原文其实有摘要；有在排队或在翻的译文时不算缺。
func missingDescription(ctx context.Context, op OperationContext, document localizationruntime.PublicTreeDocument, tag string) (bool, error) {
	var object TranslationObject
	found := false
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		var live int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM translation_manifests WHERE state IN('queued','running') AND target_language=? AND data_epoch=? AND json_extract(publication_ref,'$.documentId')=?`, tag, op.DataEpoch, document.DocumentID).Scan(&live); err != nil || live > 0 {
			return language.StorageError(err)
		}
		latest, ok, err := LatestManifest(ctx, tx, op, document.DocumentID, tag, true)
		if err != nil || !ok {
			return err
		}
		object, err = ReadObject(ctx, tx, op, latest.ID)
		found = err == nil
		return err
	})
	if err != nil || !found || object.Description != "" || object.Translation.Document.PublishedRevisionId != document.Reference.PublishedRevisionId {
		return false, err
	}
	source, err := localizationruntime.FetchPublishedSnapshot(ctx, op, document.Reference)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(source.Description) != "", nil
}

// maxAttempts 是同一发布修订、同一语言自动建译文的总份数上限（首次加 3 次重试）。到了就停，等下次发布或站点人员手动重试：
// 自动翻译花的是站点所有者的钱，不能无限重来。
const maxAttempts = 4

// nextAttempt 判断这门语言是否该为这次发布修订（再）建译文：在排队、在翻或术语变更后待更新（stale）就不用；已完成且没有单元退回原文也不用。
// 完成了但有单元退回原文（模型几次都没按格式译出）的，再建一份：已译单元沿用，只把退回的交给模型。
// 失败或补译按次数退避（10 分钟、1 小时、6 小时，供应方或密钥短暂出问题撑得过去），总份数到 maxAttempts 为止；有人取消过的不再自动重建。
// 返回本修订已有的译文份数作为本次尝试序号，用于幂等键（每建一份就加一，不会和之前的键重复）。
func nextAttempt(ctx context.Context, op OperationContext, documentID, revisionID, tag string) (int, bool, error) {
	const scope = `target_language=? AND data_epoch=? AND json_extract(publication_ref,'$.documentId')=? AND json_extract(publication_ref,'$.publishedRevisionId')=?`
	var total, busy, done, failed, cancelled, fallback int
	var lastDone, lastFailed sql.NullString
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(CASE WHEN state NOT IN('ready','review','failed','cancelled') THEN 1 END),COUNT(CASE WHEN state IN('ready','review') THEN 1 END),COUNT(CASE WHEN state='failed' THEN 1 END),COUNT(CASE WHEN state='cancelled' THEN 1 END),MAX(CASE WHEN state IN('ready','review') THEN updated_at END),MAX(CASE WHEN state='failed' THEN updated_at END) FROM translation_manifests WHERE `+scope, tag, op.DataEpoch, documentID, revisionID).Scan(&total, &busy, &done, &failed, &cancelled, &lastDone, &lastFailed); err != nil || done == 0 {
			return language.StorageError(err)
		}
		return language.StorageError(tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM translation_units WHERE status='fallback' AND translation_id=(SELECT translation_id FROM translation_manifests WHERE state IN('ready','review') AND `+scope+` ORDER BY updated_at DESC LIMIT 1)`, tag, op.DataEpoch, documentID, revisionID).Scan(&fallback))
	})
	if err != nil || busy > 0 || cancelled > 0 || done > 0 && fallback == 0 || done+failed >= maxAttempts {
		return 0, false, err
	}
	var last time.Time
	for _, value := range []sql.NullString{lastDone, lastFailed} {
		if at, parseErr := time.Parse(time.RFC3339Nano, value.String); parseErr == nil && at.After(last) {
			last = at
		}
	}
	if done+failed > 0 && time.Since(last) < retryDelay(done+failed) {
		return 0, false, nil
	}
	return total, true, nil
}

func retryDelay(failures int) time.Duration {
	return [...]time.Duration{10 * time.Minute, time.Hour, 6 * time.Hour}[min(failures, 3)-1]
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
