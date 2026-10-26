package translation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
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
		fmt.Fprintf(os.Stderr, "翻译失败 translation=%s err=%v\n", translationID, err)
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
	templates := make(map[string]UnitTemplate, len(split.Templates))
	for _, template := range split.Templates {
		templates[template.UnitId] = template
	}
	dispatch := func(units []gen.TranslationUnit) ([]gen.TranslationUnit, error) {
		if translationCancelled(ctx, op, translationID) {
			return nil, errTranslationCancelled
		}
		return translateBatch(ctx, op, translationID, route, memory, source, templates, units)
	}
	var batch []gen.TranslationUnit
	size := 0
	// 表格按行交给模型、译完再拼回的整表单元，及其各行的临时单元 ID（即行的 rowId）。
	type tableParts struct {
		unit gen.TranslationUnit
		rows []string
	}
	var tables []tableParts
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
	// 每批控制在约 1500 字节原文：日文等语言的译文词元多，批太大时一次回复要写很久，容易超出请求超时。
	enqueue := func(unit gen.TranslationUnit, text string) error {
		if size+len(text) > 1500 || len(batch) >= 12 {
			if err := flush(); err != nil {
				return err
			}
		}
		batch = append(batch, unit)
		size += len(text)
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
		// 表格按行交给模型：整表一起译时只要一个标签写错整表就退回原文，按行译出错只牵连那一行；各行译完再拼回整表单元。
		if rows := split.Templates[i].Rows; len(rows) > 1 {
			parts := tableParts{unit: unit}
			for _, row := range rows {
				slots := split.Templates[i].Slots[row.Start:row.End]
				rowText := SlotText(slots)
				part := unit
				part.UnitId, part.SourceText, part.ParentBlockId = row.RowId, &rowText, &unit.UnitId
				templates[row.RowId] = UnitTemplate{UnitId: row.RowId, ParentBlockId: unit.UnitId, Slots: slots}
				parts.rows = append(parts.rows, row.RowId)
				if VisibleCharacterCount(rowText) == 0 && strings.TrimSpace(inlineTokenPattern.ReplaceAllString(rowText, "")) == "" {
					part.TargetText, part.TargetHash, part.Origin, part.Status = rowText, platform.HashBytes([]byte(rowText)), "machine", "normal"
					translated = append(translated, part)
					continue
				}
				if err := enqueue(part, rowText); err != nil {
					return err
				}
			}
			tables = append(tables, parts)
			continue
		}
		if err := enqueue(unit, text); err != nil {
			return err
		}
	}
	if err := flush(); err != nil {
		return err
	}
	// 拼回整表：各行都译出才算译完；任一行仍不通过就整表先显示原文，交给之后的自动补译。
	if len(tables) > 0 {
		byID := make(map[string]gen.TranslationUnit, len(translated))
		for _, unit := range translated {
			byID[unit.UnitId] = unit
		}
		for _, table := range tables {
			var text strings.Builder
			complete := true
			for _, id := range table.rows {
				row := byID[id]
				delete(byID, id)
				complete = complete && row.Status == "normal"
				text.WriteString(row.TargetText)
			}
			unit := table.unit
			unit.TargetText, unit.Origin, unit.Status = text.String(), "machine", "normal"
			if !complete {
				unit.TargetText, unit.Status = *unit.SourceText, "fallback"
			}
			unit.TargetHash = platform.HashBytes([]byte(unit.TargetText))
			byID[unit.UnitId] = unit
		}
		ordered := make([]gen.TranslationUnit, 0, len(split.Units))
		for _, unit := range split.Units {
			if done, found := byID[unit.UnitId]; found {
				ordered = append(ordered, done)
			}
		}
		translated = ordered
	}
	// 标题和搜索摘要合成一个小批次单独翻译；失败只是没有标题或摘要译文（公开页沿用原文），不影响正文译文交付。
	title, description := "", ""
	extra := []gen.TranslationUnit{}
	for _, field := range []struct{ prefix, text string }{{"title.", source.Title}, {"description.", source.Description}} {
		if text := field.text; strings.TrimSpace(text) != "" {
			extra = append(extra, gen.TranslationUnit{UnitId: field.prefix + platform.HashBytes([]byte(text))[:16], SourceText: &text, SourceHash: platform.HashBytes([]byte(text))})
		}
	}
	if len(extra) > 0 {
		done, extraErr := dispatch(extra)
		if errors.Is(extraErr, errTranslationCancelled) {
			return extraErr
		}
		for _, unit := range done {
			if extraErr != nil || unit.Status == "fallback" {
				continue
			}
			if strings.HasPrefix(unit.UnitId, "title.") {
				title = strings.TrimSpace(unit.TargetText)
			} else {
				description = strings.TrimSpace(unit.TargetText)
			}
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
		object := TranslationObject{Title: title, Description: description,
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

// translateBatch 翻译一批单元并逐个校验（模型须返回本批 unitId，编号标签不得改变）。
// 个别单元没通过（例如模型把标签嵌套错了）时只重译这几个；仍不通过的先显示原文（fallback），不让整篇作废。
func translateBatch(ctx context.Context, op OperationContext, translationID string, route localizationruntime.PlanTaskRoute, memory platform.MemoryContext, source localizationruntime.PublishedSnapshot, templates map[string]UnitTemplate, batch []gen.TranslationUnit) ([]gen.TranslationUnit, error) {
	done, failed, err := translateUnits(ctx, op, translationID, "", route, memory, source, templates, batch)
	if err != nil {
		return nil, err
	}
	if len(failed) > 0 {
		retried, still, err := translateUnits(ctx, op, translationID, "retry", route, memory, source, templates, failed)
		if err != nil {
			return nil, err
		}
		done = append(done, retried...)
		// 仍不通过的先放原文（带原样标签，还原出来就是原文），标 fallback：公开页这一段显示原文并提示未译完。
		for _, unit := range still {
			unit.TargetText, unit.Origin, unit.Status = *unit.SourceText, "machine", "fallback"
			unit.TargetHash = platform.HashBytes([]byte(unit.TargetText))
			done = append(done, unit)
		}
	}
	// 保持本批原有顺序。
	order := make(map[string]int, len(batch))
	for i, unit := range batch {
		order[unit.UnitId] = i
	}
	sort.Slice(done, func(i, j int) bool { return order[done[i].UnitId] < order[done[j].UnitId] })
	return done, nil
}

// translateUnits 调一次模型翻译这些单元，返回通过校验的和没通过的。
func translateUnits(ctx context.Context, op OperationContext, translationID, attempt string, route localizationruntime.PlanTaskRoute, memory platform.MemoryContext, source localizationruntime.PublishedSnapshot, templates map[string]UnitTemplate, batch []gen.TranslationUnit) ([]gen.TranslationUnit, []gen.TranslationUnit, error) {
	prompt, err := BuildTranslationPrompt(ctx, op, BuildTranslationPromptInput{SourceUnits: batch, DocumentText: "", SectionTitle: source.Title, Memory: memory})
	if err != nil {
		return nil, nil, err
	}
	// 幂等键按译文和本批单元定：进程重启后接着翻，同一批复用网关已有的结果；重译另起键。
	ids := make([]string, len(batch))
	for i, unit := range batch {
		ids[i] = unit.UnitId
	}
	key := "translate." + platform.HashBytes([]byte(translationID+":"+attempt+":"+strings.Join(ids, ",")))
	if attempt == "" {
		key = "translate." + platform.HashBytes([]byte(translationID+":"+strings.Join(ids, ",")))
	}
	outcome, err := localizationruntime.InvokeModel(ctx, op, platform.InvokeModelRequest{IDempotencyKey: key, Request: platform.CreateInvocationInput{TaskID: "localization.document", ProfileID: route.ProfileID, ExpectedRouteVersion: route.RouteVersion, Capability: platform.CapabilityChat, Messages: prompt.Messages, MaximumOutputTokens: 4000, IDempotencyKey: key}})
	if err != nil {
		return nil, nil, err
	}
	if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil || outcome.CurrentSnapshot.Output.Text == nil {
		return nil, nil, platform.NewError(platform.CodeDependencyUnavailable, false, "模型翻译结果未完成")
	}
	var reply struct {
		Units []struct {
			UnitID     string `json:"unitId"`
			TargetText string `json:"targetText"`
		} `json:"units"`
	}
	if json.Unmarshal([]byte(stripCodeFence(*outcome.CurrentSnapshot.Output.Text)), &reply) != nil {
		// 整批回复不是约定的 JSON：全部算没通过，交给重译。
		return nil, batch, nil
	}
	answers := make(map[string]string, len(reply.Units))
	for _, item := range reply.Units {
		answers[item.UnitID] = item.TargetText
	}
	var done, failed []gen.TranslationUnit
	for _, unit := range batch {
		text, found := answers[unit.UnitId]
		template, ok := templates[unit.UnitId]
		if found && ok && !fitsTemplate(template, text) {
			if repaired, fixed := RepairSlotText(template.Slots, text); fixed {
				text = repaired
			}
		}
		if !found || ok && !fitsTemplate(template, text) {
			failed = append(failed, unit)
			continue
		}
		checked, err := ValidateTranslationOutput(ctx, op, ValidateTranslationOutputInput{SourceUnits: []gen.TranslationUnit{unit}, TranslatedUnits: []gen.TranslationUnit{{UnitId: unit.UnitId, TargetText: text}}, SourceLanguage: source.SourceLanguage})
		if err != nil {
			// 表格按行译时，整行是代码、数字或专名的常原样返回；整表一起译时这样的行本来就被接受，这里同样接受。
			if ok && template.ParentBlockId != unit.UnitId && text == *unit.SourceText {
				unit.TargetText, unit.TargetHash, unit.Origin, unit.Status = text, platform.HashBytes([]byte(text)), "machine", "normal"
				done = append(done, unit)
				continue
			}
			failed = append(failed, unit)
			continue
		}
		done = append(done, checked.Units...)
	}
	return done, failed, nil
}

// fitsTemplate 检查译文能套回原文的标签结构，且原来有文字的文本节点译后不为空（空文本节点不符合正文合同，会让整篇译文写不出来）。
func fitsTemplate(template UnitTemplate, text string) bool {
	values, err := ParseSlotValues(template.Slots, text)
	if err != nil {
		return false
	}
	originals, replaced := map[string]string{}, map[string]string{}
	for i, slot := range template.Slots {
		if len(slot.Path) == 0 {
			continue
		}
		key := strings.Join(slot.Path, "\x00")
		originals[key] += slot.Value
		replaced[key] += values[i]
	}
	for key, original := range originals {
		if original != "" && replaced[key] == "" {
			return false
		}
	}
	return true
}

// stripCodeFence 去掉模型偶尔包在 JSON 外面的 ```json 代码围栏。
func stripCodeFence(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = text[strings.Index(text, "\n")+1:]
		text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	}
	return text
}
