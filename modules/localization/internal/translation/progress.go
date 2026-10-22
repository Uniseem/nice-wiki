package translation

import (
	"context"
	"database/sql"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
	"sort"
)

type ListTranslationProgressInput struct {
	Language *string `json:"language,omitempty"`
}
type LanguageProgress struct {
	Language       string `json:"language"`
	CompletedUnits int    `json:"completedUnits"`
	QueuedUnits    int    `json:"queuedUnits"`
	ReviewUnits    int    `json:"reviewUnits"`
}
type ActiveTranslation struct {
	TranslationID string `json:"translationId"`
	DocumentID    string `json:"documentId"`
	Language      string `json:"language"`
	State         string `json:"state"`
}
type DocumentTranslation struct {
	DocumentID string `json:"documentId"`
	Language   string `json:"language"`
	State      string `json:"state"`
	Previous   bool   `json:"previous"`
}
type ListTranslationProgressResult struct {
	Languages  []LanguageProgress    `json:"languages"`
	Active     []ActiveTranslation   `json:"active"`
	Documents  []DocumentTranslation `json:"documents"`
	ModelReady bool                  `json:"modelReady"`
}

func ListTranslationProgress(ctx context.Context, op OperationContext, input ListTranslationProgressInput) (ListTranslationProgressResult, error) {
	if err := language.RequireAdministrator(op); err != nil {
		return ListTranslationProgressResult{}, err
	}
	filter := ""
	if input.Language != nil {
		parsed, err := language.NormalizeLanguageTag(ctx, op, language.NormalizeLanguageTagInput{Tag: *input.Language})
		if err != nil {
			return ListTranslationProgressResult{}, err
		}
		filter = parsed.Tag
	}
	result := ListTranslationProgressResult{Languages: []LanguageProgress{}, Active: []ActiveTranslation{}, Documents: []DocumentTranslation{}}
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT language_tag FROM languages WHERE data_epoch=? ORDER BY language_tag`, op.DataEpoch)
		if err != nil {
			return language.StorageError(err)
		}
		progress := map[string]LanguageProgress{}
		for rows.Next() {
			var tag string
			if err = rows.Scan(&tag); err != nil {
				rows.Close()
				return language.StorageError(err)
			}
			canonical, err := language.NormalizeLanguageTag(ctx, op, language.NormalizeLanguageTagInput{Tag: tag})
			if err != nil {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "语言进度配置的标签不合法")
			}
			progress[canonical.Tag] = LanguageProgress{Language: tag}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return language.StorageError(err)
		}
		// 每个文档/目标仅统计最新manifest，历史结果与旧任务不重复累计。
		rows, err = tx.QueryContext(ctx, `SELECT u.target_language,SUM(CASE WHEN u.status='normal' THEN 1 ELSE 0 END),SUM(CASE WHEN u.status='fallback' AND m.state IN('queued','running') THEN 1 ELSE 0 END),SUM(CASE WHEN u.status='review' THEN 1 ELSE 0 END) FROM translation_units u JOIN translation_manifests m ON m.translation_id=u.translation_id WHERE u.data_epoch=? AND m.data_epoch=? AND NOT EXISTS(SELECT 1 FROM translation_manifests newer WHERE newer.target_language=m.target_language AND newer.data_epoch=m.data_epoch AND json_extract(newer.publication_ref,'$.documentId')=json_extract(m.publication_ref,'$.documentId') AND newer.generation>m.generation) GROUP BY u.target_language`, op.DataEpoch, op.DataEpoch)
		if err != nil {
			return language.StorageError(err)
		}
		defer rows.Close()
		for rows.Next() {
			var tag string
			var done, queued, review int
			if err = rows.Scan(&tag, &done, &queued, &review); err != nil {
				return language.StorageError(err)
			}
			p, exists := progress[tag]
			if !exists {
				return platform.NewError(platform.CodeIntegrityError, false, "译文进度引用未登记语言")
			}
			p.CompletedUnits = done
			p.QueuedUnits = queued
			p.ReviewUnits = review
			progress[tag] = p
		}
		if err = rows.Err(); err != nil {
			return language.StorageError(err)
		}
		for tag, p := range progress {
			if filter == "" || tag == filter {
				result.Languages = append(result.Languages, p)
			}
		}
		sort.Slice(result.Languages, func(i, j int) bool { return result.Languages[i].Language < result.Languages[j].Language })
		active, err := tx.QueryContext(ctx, `SELECT translation_id,json_extract(publication_ref,'$.documentId'),target_language,state FROM translation_manifests WHERE state IN('queued','running') AND data_epoch=? ORDER BY updated_at DESC,translation_id DESC LIMIT 50`, op.DataEpoch)
		if err != nil {
			return language.StorageError(err)
		}
		defer active.Close()
		for active.Next() {
			var item ActiveTranslation
			if err = active.Scan(&item.TranslationID, &item.DocumentID, &item.Language, &item.State); err != nil {
				return language.StorageError(err)
			}
			if filter == "" || item.Language == filter {
				result.Active = append(result.Active, item)
			}
		}
		return language.StorageError(active.Err())
	})
	if err != nil {
		return result, err
	}
	background := op
	background.CostClass = platform.CostClassBackground
	_, routeErr := localizationruntime.ResolvePlanTaskRoute(ctx, background, "localization.document")
	result.ModelReady = routeErr == nil
	result.Documents, err = documentStates(ctx, op, filter)
	return result, err
}

// documentStates 按当前发布修订给出每篇公开文档在每门自动翻译语言下的译文状态，和 AutoTranslate 的判断一致。
func documentStates(ctx context.Context, op OperationContext, filter string) ([]DocumentTranslation, error) {
	tree, err := localizationruntime.ReadPublicTree(ctx, op)
	if err != nil {
		return nil, err
	}
	var languages []string
	states := map[string]string{}
	completed := map[string]bool{}
	err = language.ReadData(ctx, op, func(tx *sql.Tx) error {
		var err error
		if languages, err = queryStrings(ctx, tx, `SELECT language_tag FROM languages WHERE enabled=1 AND default_language=0 AND data_epoch=? ORDER BY language_tag`, op.DataEpoch); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT json_extract(publication_ref,'$.documentId'),json_extract(publication_ref,'$.publishedRevisionId'),target_language,state FROM translation_manifests WHERE data_epoch=?`, op.DataEpoch)
		if err != nil {
			return language.StorageError(err)
		}
		defer rows.Close()
		rank := map[string]int{"retrying": 1, "translating": 2, "translated": 3}
		for rows.Next() {
			var document, revision, tag, state string
			if err = rows.Scan(&document, &revision, &tag, &state); err != nil {
				return language.StorageError(err)
			}
			status := ""
			switch state {
			case "ready", "review", "stale":
				status = "translated"
				completed[document+"\x00"+tag] = true
			case "queued", "running":
				status = "translating"
			case "failed", "cancelled":
				status = "retrying"
			}
			key := document + "\x00" + revision + "\x00" + tag
			if rank[status] > rank[states[key]] {
				states[key] = status
			}
		}
		return language.StorageError(rows.Err())
	})
	if err != nil {
		return nil, err
	}
	result := []DocumentTranslation{}
	for _, document := range tree.Documents {
		for _, tag := range languages {
			if filter != "" && tag != filter {
				continue
			}
			state := states[document.DocumentID+"\x00"+document.Reference.PublishedRevisionId+"\x00"+tag]
			if state == "" {
				state = "waiting"
			}
			result = append(result, DocumentTranslation{DocumentID: document.DocumentID, Language: tag, State: state, Previous: state != "translated" && completed[document.DocumentID+"\x00"+tag]})
		}
	}
	return result, nil
}
