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
	// 这篇文档上一份完成的译文里，原文没变的单元直接沿用（人工改过的也保留），只把新增或改过的单元交给模型。
	previous := map[string]gen.TranslationUnit{}
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
			if unit.Status != "fallback" {
				previous[unit.UnitId] = unit
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	translated := make([]gen.TranslationUnit, 0, len(split.Units))
	var batch []gen.TranslationUnit
	size := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if translationCancelled(ctx, op, translationID) {
			return errTranslationCancelled
		}
		done, err := translateBatch(ctx, op, translationID, route, memory, source, batch)
		if err != nil {
			return err
		}
		translated = append(translated, done...)
		batch, size = nil, 0
		return nil
	}
	for _, unit := range split.Units {
		text := *unit.SourceText
		if prior, ok := previous[unit.UnitId]; ok && prior.SourceHash == unit.SourceHash {
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
		if done, titleErr := translateBatch(ctx, op, translationID, route, memory, source, []gen.TranslationUnit{titleUnit}); titleErr == nil && len(done) == 1 {
			title = strings.TrimSpace(done[0].TargetText)
		}
	}
	return language.WriteData(ctx, op, func(tx *sql.Tx) error {
		object := TranslationObject{Title: title,
			Translation: gen.Translation{TranslationId: translationID, Document: document, TargetLanguage: target, State: gen.TranslationState("ready"), Total: len(translated), Completed: len(translated), MemoryVersionHash: memory.VersionHash},
			SourceTree:  split.SourceTree, Units: translated,
		}
		if _, err := WriteObject(ctx, tx, op, object, 0); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE translation_manifests SET state='ready',updated_at=? WHERE translation_id=? AND state='running'`, platform.Now(), translationID)
		return language.StorageError(err)
	})
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
