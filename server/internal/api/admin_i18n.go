package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/i18n"
	itranslate "nice-wiki/internal/i18n/translate"
	"nice-wiki/internal/settings"
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
	body, err := decode[gen.LanguageSettings](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := s.Settings.SaveLanguages(settings.Languages{Base: body.Base, Enabled: body.Enabled}); err != nil {
		s.validationOrFailure(w, err)
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
			file, err := s.Content.Get(meta.ID)
			if err != nil {
				continue
			}
			blocks, err := doc.Blocks(file.Document)
			if err != nil {
				continue
			}
			total += len(blocks)
			translation, err := s.Translations.ReadOptional(language, meta.ID)
			if err != nil {
				continue
			}
			status := itranslate.Progress(language, len(blocks), translation)
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
func (s *Server) GetTranslation(w http.ResponseWriter, r *http.Request, language, id string) {
	value, err := s.Translations.ReadOptional(language, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, value)
}
func (s *Server) SaveTranslation(w http.ResponseWriter, r *http.Request, language, id string) {
	body, err := decode[gen.TranslationFile](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	blocks := map[string]itranslate.Block{}
	for key, value := range body.Blocks {
		blocks[key] = itranslate.Block{ID: value.Id, SourceHash: value.SourceHash, Text: value.Text, Source: itranslate.SourceHuman, NeedsReview: value.NeedsReview}
	}
	file := itranslate.File{SchemaVersion: body.SchemaVersion, DocumentID: id, Language: language, Blocks: blocks, UpdatedAt: time.Now().Unix()}
	if err := s.Translations.Write(file); err != nil {
		s.validationOrFailure(w, err)
		return
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
	estimate := itranslate.EstimateCost(characters, 1, 3)
	_ = body.Language
	writeJSON(w, estimate)
}
