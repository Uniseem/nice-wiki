package translation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	interfacecatalog "nice-wiki/modules/localization/internal/interface"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

type ReadTranslationInput = gen.ReadTranslationInput
type ReadTranslationResult struct {
	Translation gen.Translation       `json:"translation"`
	Units       []gen.TranslationUnit `json:"units"`
	Partial     bool                  `json:"partial"`
	NoticeKey   *string               `json:"noticeKey,omitempty"`
	Title       *string               `json:"title,omitempty"`
	Body        json.RawMessage       `json:"body"`
	BodyHash    string                `json:"bodyHash"`
}
type manifestWatermark struct {
	ID         string
	Generation int
	State      string
}

func LatestManifest(ctx context.Context, tx *sql.Tx, op OperationContext, document, target string, completedOnly bool) (manifestWatermark, bool, error) {
	query := `SELECT translation_id,generation,state FROM translation_manifests WHERE json_extract(publication_ref,'$.documentId')=? AND target_language=? AND data_epoch=? AND file_ref IS NOT NULL AND file_ref<>'' AND state IN('ready','review','stale','queued','running')`
	if completedOnly {
		query += ` AND state IN('ready','review','stale')`
	}
	query += ` ORDER BY generation DESC,translation_id DESC LIMIT 1`
	var m manifestWatermark
	err := tx.QueryRowContext(ctx, query, document, target, op.DataEpoch).Scan(&m.ID, &m.Generation, &m.State)
	if errors.Is(err, sql.ErrNoRows) {
		return m, false, nil
	}
	return m, err == nil, language.StorageError(err)
}
func ReadTranslation(ctx context.Context, op OperationContext, input ReadTranslationInput) (ReadTranslationResult, error) {
	// 公开站点读访客要看的译文；问答只读英文译文，给决策模型筛选材料用。
	if op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService || (op.CallerService != "public-site" && op.CallerService != "qa") {
		return ReadTranslationResult{}, platform.NewError(platform.CodeForbidden, false, "公开译文只接受公开站点和问答服务读取")
	}
	source, err := localizationruntime.FetchPublishedSnapshot(ctx, op, input.Document)
	if err != nil {
		return ReadTranslationResult{}, err
	}
	split, err := SplitTranslationUnits(ctx, op, SplitTranslationUnitsInput{Document: input.Document, Source: string(source.Body), TargetLanguage: input.TargetLanguage})
	if err != nil {
		return ReadTranslationResult{}, err
	}
	var latest, prior manifestWatermark
	var selected []manifestWatermark
	var current, previous TranslationObject
	var glossary int
	target := ""
	err = language.ReadData(ctx, op, func(tx *sql.Tx) error {
		var err error
		target, err = interfacecatalog.EnabledLanguage(ctx, tx, input.TargetLanguage)
		if err != nil && op.CallerService == "qa" && platform.IsEnglishTag(input.TargetLanguage) {
			// 问答只说要英文，不知道站点用的是 en 还是 en-US。
			target, err = language.EnabledEnglish(ctx, tx, op)
		}
		if err != nil {
			return err
		}
		var found bool
		latest, found, err = LatestManifest(ctx, tx, op, input.Document.DocumentId, target, false)
		if err != nil {
			return err
		}
		if !found {
			return platform.NewError(platform.CodeNotFound, false, "本语言暂无可交付译文")
		}
		selected = append(selected, latest)
		prior, found, err = LatestManifest(ctx, tx, op, input.Document.DocumentId, target, true)
		if err != nil {
			return err
		}
		if found && prior.ID != latest.ID {
			selected = append(selected, prior)
		}
		total := int64(0)
		for _, m := range selected {
			var size int64
			if err = tx.QueryRowContext(ctx, `SELECT json_extract(file_ref,'$.byteSize') FROM translation_manifests WHERE translation_id=?`, m.ID).Scan(&size); err != nil {
				return language.StorageError(err)
			}
			total += size
		}
		if total > 64<<20 {
			return platform.NewError(platform.CodeBudgetExceeded, false, "新旧译文对象读取超过64MiB预算")
		}
		current, err = ReadObject(ctx, tx, op, latest.ID)
		if err != nil {
			return err
		}
		if len(selected) > 1 {
			previous, err = ReadObject(ctx, tx, op, prior.ID)
			if err != nil {
				return err
			}
		}
		return language.StorageError(tx.QueryRowContext(ctx, `SELECT glossary_generation FROM domain_versions WHERE singleton=1`).Scan(&glossary))
	})
	if err != nil {
		return ReadTranslationResult{}, err
	}
	newest := map[string]gen.TranslationUnit{}
	old := map[string]gen.TranslationUnit{}
	projection, err := BuildProtectedProjection(ctx, op, source.Body, []TranslationObject{current, previous})
	if err != nil {
		return ReadTranslationResult{}, err
	}
	for _, id := range split.FallbackBlockIds {
		if projection.Whole[id] || projection.Rows[id] {
			return ReadTranslationResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "受保护块超出可恢复模板范围，保留既有译文等待人工处理")
		}
	}
	for _, u := range current.Units {
		newest[u.UnitId] = u
	}
	for _, u := range previous.Units {
		old[u.UnitId] = u
	}
	result := ReadTranslationResult{Translation: current.Translation, Units: []gen.TranslationUnit{}, Partial: len(split.FallbackBlockIds) > 0}
	result.Translation.Document = input.Document
	result.Translation.TargetLanguage = target
	if current.Title != "" {
		title := current.Title
		result.Title = &title
	}
	result.Translation.Total = len(split.Units) + len(split.FallbackBlockIds)
	result.Translation.Completed = 0
	for i, candidate := range split.Units {
		chosen, exists := newest[candidate.UnitId]
		if !exists || chosen.Status == "fallback" {
			if p, found := old[candidate.UnitId]; found && p.Status != "fallback" {
				chosen = p
				exists = true
			}
		}
		if exists && chosen.Status == "fallback" && (chosen.Protected || chosen.Origin == "human") {
			return ReadTranslationResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "人工或受保护旧译未形成可恢复正文")
		}
		if !exists || chosen.SourceHash != candidate.SourceHash || chosen.Status == "fallback" {
			projected, found, partial, projectionErr := ProjectProtectedUnit(projection, split.Templates[i], candidate)
			if projectionErr != nil {
				return ReadTranslationResult{}, projectionErr
			}
			if found {
				result.Units = append(result.Units, projected)
				result.Partial = result.Partial || partial
				if !partial {
					result.Translation.Completed++
				}
				continue
			}
		}
		if exists && chosen.Status != "fallback" {
			if _, parseErr := ParseSlotValues(split.Templates[i].Slots, chosen.TargetText); parseErr == nil {
				if chosen.SourceHash != candidate.SourceHash {
					chosen.Status = "review"
				}
				// 沿用旧发布修订的译文时按本次请求的发布交付：新译文生成前继续显示旧译文。
				chosen.Document = input.Document
				result.Units = append(result.Units, chosen)
				result.Translation.Completed++
				continue
			}
			if chosen.Protected || chosen.Origin == "human" {
				return ReadTranslationResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "源模板变化后无法安全恢复人工或受保护译文")
			}
			priorText := chosen.TargetText
			candidate.PreviousTargetText = &priorText
		}
		candidate.Status = "fallback"
		result.Partial = true
		result.Units = append(result.Units, candidate)
	}
	for _, unit := range result.Units {
		if unit.Status == "review" {
			result.Translation.State = "review"
			break
		}
	}
	result.Body, err = RestoreSourceTree(source.Body, split.Templates, result.Units)
	if err != nil {
		return ReadTranslationResult{}, err
	}
	if len(result.Body) > 16<<20 {
		return ReadTranslationResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "目标正文超过公开渲染16MiB预算")
	}
	result.BodyHash = platform.HashBytes(result.Body)
	if result.Partial {
		key := "translation.partial"
		result.NoticeKey = &key
	}
	if _, err = localizationruntime.CheckPublication(ctx, op, input.Document, &source.VisibilityRevision); err != nil {
		return ReadTranslationResult{}, err
	}
	// 远程IO之后再核本地精确水位，不能把新旧片段混装成新成功结果。
	err = language.ReadData(ctx, op, func(tx *sql.Tx) error {
		now, found, err := LatestManifest(ctx, tx, op, input.Document.DocumentId, target, false)
		if err != nil {
			return err
		}
		if !found || now != latest {
			return platform.NewError(platform.CodeStaleResult, false, "译文读取期间目标水位改变")
		}
		for _, m := range selected {
			var generation int
			var state string
			if err = tx.QueryRowContext(ctx, `SELECT generation,state FROM translation_manifests WHERE translation_id=?`, m.ID).Scan(&generation, &state); err != nil {
				return language.StorageError(err)
			}
			if generation != m.Generation || state != m.State {
				return platform.NewError(platform.CodeStaleResult, false, "译文读取期间状态改变")
			}
		}
		var g int
		if err = tx.QueryRowContext(ctx, `SELECT glossary_generation FROM domain_versions WHERE singleton=1`).Scan(&g); err != nil {
			return language.StorageError(err)
		}
		if g != glossary {
			return platform.NewError(platform.CodeStaleResult, false, "译文读取期间术语代数改变")
		}
		return nil
	})
	if err != nil {
		return ReadTranslationResult{}, err
	}
	return result, nil
}
