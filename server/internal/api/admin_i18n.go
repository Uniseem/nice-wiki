package api

import (
	"errors"
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/i18n"
	itranslate "nice-wiki/internal/i18n/translate"
	"nice-wiki/internal/jobs"
	"nice-wiki/internal/settings"
	"nice-wiki/internal/textutil"
	"strings"
	"time"
)

func (s *Server) ListLanguages(w http.ResponseWriter, r *http.Request) {
	value := s.Settings.Languages()
	writeJSON(w, gen.LanguagesAdmin{Base: value.Base, Enabled: value.Enabled, Available: func() []gen.LanguageInfo {
		result := make([]gen.LanguageInfo, len(i18n.Languages))
		for index, v := range i18n.Languages {
			result[index] = gen.LanguageInfo{Code: v.Code, Name: v.Name, NativeName: v.NativeName, Rtl: v.RTL}
		}
		return result
	}()})
}
func (s *Server) SaveLanguages(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.LanguagesUpdate](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	current := s.Settings.Languages()
	existing := map[string]bool{}
	for _, code := range current.Enabled {
		existing[code] = true
	}
	newTargets := []string{}
	for _, code := range body.Enabled {
		if code != current.Base && !existing[code] {
			newTargets = append(newTargets, code)
		}
	}
	if len(newTargets) > 0 && !body.ConfirmCosts {
		s.validationOrFailure(w, &settings.ValidationError{Message: "启用新语言前必须确认翻译费用"})
		return
	}
	if len(newTargets) > 0 && (!s.Settings.AI().Enabled || !s.Settings.AIReady()) {
		s.validationOrFailure(w, &settings.ValidationError{Message: "模型档位和 AI 尚未就绪，不能启用新语言"})
		return
	}
	if err := s.Settings.SaveLanguages(settings.Languages{Base: current.Base, Enabled: body.Enabled}); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if len(newTargets) > 0 && s.Jobs != nil {
		rollback := func(cause error) {
			s.fail(w, errors.Join(cause, s.Settings.SaveLanguages(current)))
		}
		for _, meta := range s.Content.List() {
			if meta.Published {
				if err := s.Jobs.Enqueue(r.Context(), jobs.TranslateDocument, meta.ID); err != nil {
					rollback(err)
					return
				}
			}
		}
		if err := s.Jobs.Enqueue(r.Context(), jobs.TranslateSite, "settings"); err != nil {
			rollback(err)
			return
		}
	}
	if err := s.commitSetting(r.Context(), r, "languages"); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "save_languages", "") {
		return
	}
	writeJSON(w, s.Settings.Languages())
}
func (s *Server) TranslationStatus(w http.ResponseWriter, r *http.Request) {
	languages := s.Settings.Languages()
	documents := s.Content.List()
	result := []itranslate.Status{}
	for _, language := range languages.Enabled {
		if language == languages.Base {
			continue
		}
		total, translated, review := 0, 0, 0
		for _, meta := range documents {
			if !meta.Published {
				continue
			}
			blocks, err := s.translationSource(language, meta.ID)
			if err != nil {
				continue
			}
			total += len(blocks)
			translation, err := s.Translations.ReadOptional(language, meta.ID)
			if err != nil {
				continue
			}
			current := itranslate.File{Blocks: map[string]itranslate.Block{}}
			for _, block := range blocks {
				if value, ok := translation.Blocks[block.ID]; ok {
					current.Blocks[block.ID] = value
				}
			}
			status := itranslate.Progress(language, len(blocks), current)
			translated += status.Translated
			review += status.NeedsReview
		}
		percent := 0
		if total > 0 {
			percent = translated * 100 / total
		}
		result = append(result, itranslate.Status{Language: language, Total: total, Translated: translated, NeedsReview: review, Percent: percent})
	}
	writeJSON(w, result)
}
func (s *Server) translationSource(language, id string) ([]doc.Block, error) {
	languages := s.Settings.Languages()
	enabled := false
	for _, code := range languages.Enabled {
		if code == language && language != languages.Base {
			enabled = true
			break
		}
	}
	if !enabled {
		return nil, &settings.ValidationError{Message: "目标语言未启用"}
	}
	file, err := s.Content.Get(id)
	if err != nil {
		return nil, err
	}
	if !file.Meta.Published {
		return nil, &settings.ValidationError{Message: "只能管理已发布文档的译文"}
	}
	blocks, err := doc.Blocks(file.Document)
	if err != nil {
		return nil, err
	}
	result := []doc.Block{{ID: itranslate.MetaTitleID, Kind: "meta", ContentHash: textutil.Hash([]byte(file.Meta.Title)), Translatable: []string{file.Meta.Title}}}
	for _, block := range blocks {
		if len(block.Translatable) > 0 {
			result = append(result, block)
		}
	}
	if strings.TrimSpace(file.Meta.SEODescription) != "" {
		result = append(result, doc.Block{ID: itranslate.MetaDescriptionID, Kind: "meta", ContentHash: textutil.Hash([]byte(file.Meta.SEODescription)), Translatable: []string{file.Meta.SEODescription}})
	}
	return result, nil
}
func (s *Server) GetTranslation(w http.ResponseWriter, r *http.Request, language, id string) {
	blocks, err := s.translationSource(language, id)
	if err != nil {
		s.validationOrFailure(w, err)
		return
	}
	value, err := s.Translations.ReadOptional(language, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	current := map[string]itranslate.Block{}
	for _, block := range blocks {
		if len(block.Translatable) == 0 {
			continue
		}
		if translated, ok := value.Blocks[block.ID]; ok {
			current[block.ID] = translated
		} else {
			current[block.ID] = itranslate.Block{ID: block.ID, SourceHash: block.ContentHash, Text: block.Translatable[0], Source: itranslate.SourceMachine, NeedsReview: true}
		}
	}
	value.Blocks = current
	writeJSON(w, value)
}
func (s *Server) SaveTranslation(w http.ResponseWriter, r *http.Request, language, id string) {
	body, err := decode[gen.TranslationFile](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	source, err := s.translationSource(language, id)
	if err != nil {
		s.validationOrFailure(w, err)
		return
	}
	hashes := map[string]string{}
	for _, block := range source {
		if len(block.Translatable) > 0 {
			hashes[block.ID] = block.ContentHash
		}
	}
	blocks := map[string]itranslate.Block{}
	for key, value := range body.Blocks {
		hash, ok := hashes[key]
		if !ok || value.Id != key || strings.TrimSpace(value.Text) == "" {
			s.validationOrFailure(w, &settings.ValidationError{Message: "译文包含空内容、未知或失配的块"})
			return
		}
		blocks[key] = itranslate.Block{ID: key, SourceHash: hash, Text: value.Text, Source: itranslate.SourceHuman, NeedsReview: false}
	}
	file := itranslate.File{SchemaVersion: body.SchemaVersion, DocumentID: id, Language: language, Blocks: blocks, UpdatedAt: time.Now().Unix()}
	if err := s.Translations.Write(file); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if _, err := s.History.Commit(r.Context(), current(r).Account.Username, "人工更新译文 "+language, []string{"i18n/" + language + "/" + id + ".json"}); err != nil {
		s.fail(w, err)
		return
	}
	if s.ClearAnswerCacheData != nil {
		if err := s.ClearAnswerCacheData(r.Context()); err != nil {
			s.fail(w, err)
			return
		}
	}
	if !s.audit(w, r, "save_translation", language+":"+id) {
		return
	}
	writeJSON(w, file)
}
func (s *Server) ListGlossary(w http.ResponseWriter, r *http.Request, params gen.ListGlossaryParams) {
	language := ""
	if params.Language != nil {
		language = *params.Language
	}
	writeJSON(w, s.Glossary.List(language, ""))
}
func (s *Server) SaveGlossaryTerm(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.GlossaryTerm](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value := itranslate.Term{ID: body.Id, SourceText: body.SourceText, TargetText: body.TargetText, Language: body.Language, Source: itranslate.TermManual, DocumentID: body.DocumentId}
	if err := s.Glossary.Save(value); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if _, err := s.History.Commit(r.Context(), current(r).Account.Username, "更新术语表", []string{"i18n/glossary.json"}); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "save_glossary", body.Id) {
		return
	}
	writeJSON(w, value)
}
func (s *Server) EstimateTranslation(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.TranslationEstimateInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	characters := 0
	for _, meta := range s.Content.List() {
		if !meta.Published {
			continue
		}
		file, err := s.Content.Get(meta.ID)
		if err != nil {
			continue
		}
		text, err := doc.RenderText(file.Document)
		if err == nil {
			characters += len([]rune(text))
		}
	}
	estimate := itranslate.EstimateCost(characters, 0, 0)
	if s.TranslationEstimate != nil {
		estimate = s.TranslationEstimate(characters)
	}
	_ = body.Language
	writeJSON(w, estimate)
}
