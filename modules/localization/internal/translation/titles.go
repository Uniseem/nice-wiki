package translation

import (
	"context"
	"database/sql"
	"encoding/json"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	interfacecatalog "nice-wiki/modules/localization/internal/interface"
	"nice-wiki/modules/localization/internal/language"
)

type ListTranslationTitlesInput struct {
	TargetLanguage string `json:"targetLanguage"`
}
type ListTranslationTitlesResult = gen.ListTranslationTitlesResult

// ListTranslationTitles 列出某语言各文档最新已完成译文的标题和搜索摘要（连同它对应的发布修订），
// 供公开站点在导航、面包屑和首页卡片里显示译文。只返回有标题译文的文档；调用方要核对发布修订，修订不一致就沿用原文。
func ListTranslationTitles(ctx context.Context, op OperationContext, input ListTranslationTitlesInput) (ListTranslationTitlesResult, error) {
	if op.CallerService != "public-site" || op.Actor.Kind != "service" || op.Actor.SubjectID != "public-site" {
		return ListTranslationTitlesResult{}, platform.NewError(platform.CodeForbidden, false, "译文标题只接受公开站点服务读取")
	}
	type titleItem = struct {
		Description         *string        `json:"description,omitempty"`
		DocumentId          gen.Identifier `json:"documentId"`
		PublishedRevisionId gen.Identifier `json:"publishedRevisionId"`
		Title               string         `json:"title"`
	}
	result := ListTranslationTitlesResult{Titles: []titleItem{}}
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		target, err := interfacecatalog.EnabledLanguage(ctx, tx, input.TargetLanguage)
		if err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT translation_id,publication_ref FROM translation_manifests WHERE target_language=? AND data_epoch=? AND file_ref IS NOT NULL AND file_ref<>'' AND state IN('ready','review','stale') ORDER BY generation DESC,translation_id DESC LIMIT 2000`, target, op.DataEpoch)
		if err != nil {
			return language.StorageError(err)
		}
		type candidate struct{ id, ref string }
		var list []candidate
		for rows.Next() {
			var c candidate
			if err := rows.Scan(&c.id, &c.ref); err != nil {
				rows.Close()
				return language.StorageError(err)
			}
			list = append(list, c)
		}
		rows.Close()
		seen := map[string]bool{}
		for _, c := range list {
			var ref gen.PublishedRevisionRef
			if json.Unmarshal([]byte(c.ref), &ref) != nil || seen[ref.DocumentId] {
				continue
			}
			seen[ref.DocumentId] = true
			object, err := ReadObject(ctx, tx, op, c.id)
			if err != nil || object.Title == "" {
				continue
			}
			item := titleItem{DocumentId: ref.DocumentId, PublishedRevisionId: ref.PublishedRevisionId, Title: object.Title}
			if object.Description != "" {
				description := object.Description
				item.Description = &description
			}
			result.Titles = append(result.Titles, item)
			if len(result.Titles) >= 1000 {
				break
			}
		}
		return nil
	})
	return result, err
}
