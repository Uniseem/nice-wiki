package translation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
)

var errTranslationCancelled = errors.New("翻译已被取消")

// executing 记着本进程里正在翻的译文，同一份不会被翻两次。
var executing sync.Map

// ExecuteTranslation 把一份 queued 译文翻译完成：按批调用模型、逐条校验、写出权威译文对象并置 ready；失败置 failed。
// 进程重启后接着翻 running 的译文也走这里：已完成的批次按幂等键由模型网关复用，不重复花钱。
func ExecuteTranslation(ctx context.Context, op OperationContext, translationID string) {
	if _, busy := executing.LoadOrStore(translationID, true); busy {
		return
	}
	defer executing.Delete(translationID)
	work, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := runTranslation(work, op, translationID); err != nil && errors.Is(err, errTranslationCancelled) {
		return
	} else if err != nil {
		if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "翻译失败 translation=%s err=%v\n", translationID, err)
		}
		fail, stop := context.WithTimeout(ctx, 15*time.Second)
		defer stop()
		_ = language.WriteData(fail, op, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(fail, `UPDATE translation_manifests SET state='failed',updated_at=? WHERE translation_id=? AND state IN('queued','running')`, platform.Now(), translationID)
			return language.StorageError(err)
		})
	}
}

func runTranslation(ctx context.Context, op OperationContext, translationID string) error {
	var document gen.PublishedRevisionRef
	var target string
	err := language.WriteData(ctx, op, func(tx *sql.Tx) error {
		var ref string
		if err := tx.QueryRowContext(ctx, `SELECT publication_ref,target_language FROM translation_manifests WHERE translation_id=? AND data_epoch=?`, translationID, op.DataEpoch).Scan(&ref, &target); err != nil {
			return language.StorageError(err)
		}
		if json.Unmarshal([]byte(ref), &document) != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "译文来源引用损坏")
		}
		_, err := tx.ExecContext(ctx, `UPDATE translation_manifests SET state='running',updated_at=? WHERE translation_id=? AND state='queued'`, platform.Now(), translationID)
		return language.StorageError(err)
	})
	if err != nil {
		return err
	}
	source, err := localizationruntime.FetchPublishedSnapshot(ctx, op, document)
	if err != nil {
		return err
	}
	split, err := SplitTranslationUnits(ctx, op, SplitTranslationUnitsInput{Document: document, Source: string(source.Body), TargetLanguage: target})
	if err != nil {
		return err
	}
	route, err := localizationruntime.ResolvePlanTaskRoute(ctx, op, "localization.document")
	if err != nil {
		return err
	}
	memory := platform.MemoryContext{TaskID: "localization.document", Language: target, VersionHash: platform.HashBytes(nil)}
	// 这篇文档上一份完成的译文里，原文没变的单元直接沿用，只把新增或改过的单元交给模型。
	// 人工改过或受保护的单元（及受保护整块下新出现的单元）不交给模型，规则同 PlanIncrementalTranslation。
	previous := map[string]gen.TranslationUnit{}
	wholeProtected, rowProtected := map[string]bool{}, map[string]bool{}
	err = language.ReadData(ctx, op, func(tx *sql.Tx) error {
		prior, found, err := LatestManifest(ctx, tx, op, document.DocumentId, target, true)
		if err != nil || !found || prior.ID == translationID {
			return err
		}
		object, err := ReadObject(ctx, tx, op, prior.ID)
		if err != nil {
			return err
		}
		for _, unit := range object.Units {
			if unit.Status == "fallback" {
				continue
			}
			previous[unit.UnitId] = unit
			if protectedUnit(unit) {
				if unit.ParentBlockId == nil || *unit.ParentBlockId == unit.UnitId {
					wholeProtected[unit.UnitId] = true
				} else {
					rowProtected[*unit.ParentBlockId] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	translated := make([]gen.TranslationUnit, 0, len(split.Units))
	// 每次调用模型之前都确认译文没被取消，正文批次和标题一样。
	dispatch := func(units []gen.TranslationUnit) ([]gen.TranslationUnit, error) {
		if translationCancelled(ctx, op, translationID) {
			return nil, errTranslationCancelled
		}
		return translateBatch(ctx, op, translationID, route, memory, source, units)
	}
	var batch []gen.TranslationUnit
	size := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		done, err := dispatch(batch)
		if err != nil {
			return err
		}
		translated = append(translated, done...)
		batch, size = nil, 0
		return nil
	}
	for i, unit := range split.Units {
		text := *unit.SourceText
		parent := unit.UnitId
		if unit.ParentBlockId != nil {
			parent = *unit.ParentBlockId
		}
		prior, ok := previous[unit.UnitId]
		if ok && protectedUnit(prior) {
			// previousTargetText 不为空表示上一轮起人工旧译就已套不进原文，还在等人重新对齐。
			pending := prior.PreviousTargetText
			fits := pending == nil
			if fits && prior.SourceHash != unit.SourceHash {
				_, parseErr := ParseSlotValues(split.Templates[i].Slots, prior.TargetText)
				fits = parseErr == nil
			}
			if fits {
				// 原文变了但旧译文还套得进新原文的标签结构：沿用并标待复核。
				if prior.SourceHash != unit.SourceHash {
					prior.SourceHash, prior.Status = unit.SourceHash, "review"
				}
				prior.Document, prior.TargetLanguage, prior.SourceText, prior.PreviousTargetText = document, unit.TargetLanguage, unit.SourceText, nil
				translated = append(translated, prior)
				continue
			}
			// 套不进（如原文局部加粗后文本段数变了）：不交给模型，也不省略。
			// 本单元先显示原文并标待复核、保持保护，人工旧译放进 previousTargetText 留待对齐。
			if pending == nil {
				old := prior.TargetText
				pending = &old
			}
			unit.Origin, unit.Protected, unit.Status, unit.PreviousTargetText = prior.Origin, true, "review", pending
			unit.Version, unit.HumanGeneration = prior.Version, prior.HumanGeneration
			translated = append(translated, unit)
			continue
		}
		if !ok && (wholeProtected[parent] || (unit.UnitId == parent && rowProtected[parent])) {
			// 受保护整块下新出现的单元同样不交给模型：先显示原文，标保护和待复核。
			unit.Protected, unit.Status = true, "review"
			translated = append(translated, unit)
			continue
		}
		if ok && prior.SourceHash == unit.SourceHash {
			prior.Document, prior.TargetLanguage, prior.SourceText, prior.PreviousTargetText = document, unit.TargetLanguage, unit.SourceText, nil
			translated = append(translated, prior)
			continue
		}
		if VisibleCharacterCount(text) == 0 && strings.TrimSpace(inlineTokenPattern.ReplaceAllString(text, "")) == "" {
			// 没有可见文字（只有编号标签或空白）的单元不必翻译，沿用原文。
			unit.TargetText = text
			unit.TargetHash = platform.HashBytes([]byte(text))
			unit.Origin, unit.Status = "machine", "normal"
			translated = append(translated, unit)
			continue
		}
		if size+len(text) > 3000 || len(batch) >= 20 {
			if err := flush(); err != nil {
				return err
			}
		}
		batch = append(batch, unit)
		size += len(text)
	}
	if err := flush(); err != nil {
		return err
	}
	// 标题单独走一个单元的翻译批次；失败只是没有标题译文，不影响正文译文交付。
	title := ""
	if strings.TrimSpace(source.Title) != "" {
		sourceTitle := source.Title
		titleUnit := gen.TranslationUnit{UnitId: "title." + platform.HashBytes([]byte(sourceTitle))[:16], SourceText: &sourceTitle, SourceHash: platform.HashBytes([]byte(sourceTitle))}
		done, titleErr := dispatch([]gen.TranslationUnit{titleUnit})
		if errors.Is(titleErr, errTranslationCancelled) {
			return titleErr
		}
		if titleErr == nil && len(done) == 1 {
			title = strings.TrimSpace(done[0].TargetText)
		}
	}
	return language.WriteData(ctx, op, func(tx *sql.Tx) error {
		// 最后一批调用期间被取消的，不再写出结果。
		var state string
		if err := tx.QueryRowContext(ctx, `SELECT state FROM translation_manifests WHERE translation_id=?`, translationID).Scan(&state); err != nil {
			return language.StorageError(err)
		}
		if state != "running" {
			return errTranslationCancelled
		}
		finished := "ready"
		for _, unit := range translated {
			if unit.Status == "review" {
				finished = "review"
				break
			}
		}
		object := TranslationObject{Title: title,
			Translation: gen.Translation{TranslationId: translationID, Document: document, TargetLanguage: target, State: gen.TranslationState(finished), Total: len(translated), Completed: len(translated), MemoryVersionHash: memory.VersionHash},
			SourceTree:  split.SourceTree, Units: translated,
		}
		if _, err := WriteObject(ctx, tx, op, object, 0); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE translation_manifests SET state=?,updated_at=? WHERE translation_id=? AND state='running'`, finished, platform.Now(), translationID)
		return language.StorageError(err)
	})
}

// protectedUnit 人工改过或标了保护的译文单元，不能被机器译文覆盖。
func protectedUnit(unit gen.TranslationUnit) bool {
	return unit.Protected || unit.Origin == "human"
}

// translateBatch 翻译一批单元并校验：模型必须完整返回本批 unitId，编号标签不得改变。
func translateBatch(ctx context.Context, op OperationContext, translationID string, route localizationruntime.PlanTaskRoute, memory platform.MemoryContext, source localizationruntime.PublishedSnapshot, batch []gen.TranslationUnit) ([]gen.TranslationUnit, error) {
	prompt, err := BuildTranslationPrompt(ctx, op, BuildTranslationPromptInput{SourceUnits: batch, DocumentText: "", SectionTitle: source.Title, Memory: memory})
	if err != nil {
		return nil, err
	}
	// 幂等键按译文和本批单元定：进程重启后接着翻，同一批复用网关已有的结果。
	ids := make([]string, len(batch))
	for i, unit := range batch {
		ids[i] = unit.UnitId
	}
	key := "translate." + platform.HashBytes([]byte(translationID+":"+strings.Join(ids, ",")))
	outcome, err := localizationruntime.InvokeModel(ctx, op, platform.InvokeModelRequest{IDempotencyKey: key, Request: platform.CreateInvocationInput{TaskID: "localization.document", ProfileID: route.ProfileID, ExpectedRouteVersion: route.RouteVersion, Capability: platform.CapabilityChat, Messages: prompt.Messages, MaximumOutputTokens: 4000, IDempotencyKey: key}})
	if err != nil {
		return nil, err
	}
	if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil || outcome.CurrentSnapshot.Output.Text == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "模型翻译结果未完成")
	}
	var reply struct {
		Units []struct {
			UnitID     string `json:"unitId"`
			TargetText string `json:"targetText"`
		} `json:"units"`
	}
	if json.Unmarshal([]byte(*outcome.CurrentSnapshot.Output.Text), &reply) != nil {
		return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "模型翻译结果不是约定的JSON")
	}
	candidates := make([]gen.TranslationUnit, 0, len(reply.Units))
	for _, item := range reply.Units {
		candidates = append(candidates, gen.TranslationUnit{UnitId: item.UnitID, TargetText: item.TargetText})
	}
	checked, err := ValidateTranslationOutput(ctx, op, ValidateTranslationOutputInput{SourceUnits: batch, TranslatedUnits: candidates, SourceLanguage: source.SourceLanguage})
	if err != nil {
		return nil, err
	}
	return checked.Units, nil
}
