package publicsite

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/public-site/internal/gen"
	"time"
)

type LoadPublishedBundleInput struct {
	Publication gen.PublishedRevisionRef
	Locale      string
}
type LoadPublishedBundleResult struct {
	Document             TiptapDocument
	Title                string
	UpdatedAt            time.Time
	MediaRefs            []AssetVersionRef
	UntranslatedBlockIDs []string
	Publication          gen.PublishedRevisionRef
	Description          string
	SourceLanguage       string
}

// LoadPublishedBundle 读取精确发布对象；复杂富文本无已确认回填规则时保留原文并明确提示。
// 当前只实现不丢失格式的纯文本译文子集，本函数完整翻译合同仍为partial。
func LoadPublishedBundle(ctx context.Context, op platform.OperationContext, in LoadPublishedBundleInput) (LoadPublishedBundleResult, error) {
	directory, err := directoryFrom(ctx)
	if err != nil {
		return LoadPublishedBundleResult{}, err
	}
	raw, err := directory.Content.ReadPublishedRevision(ctx, in.Publication.DocumentId, url.Values{"publishedRevisionId": []string{in.Publication.PublishedRevisionId}})
	if err != nil {
		return LoadPublishedBundleResult{}, err
	}
	var envelope struct {
		Reference gen.PublishedRevisionRef `json:"reference"`
		Body      json.RawMessage          `json:"body"`
		Metadata  *struct {
			Title           string     `json:"title"`
			SourceLanguage  string     `json:"sourceLanguage"`
			SEODescription  *string    `json:"seoDescription"`
			LastPublishedAt *time.Time `json:"lastPublishedAt"`
		} `json:"metadata"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Metadata == nil || envelope.Metadata.LastPublishedAt == nil {
		return LoadPublishedBundleResult{}, dependency("公开正文元数据权威不完整")
	}
	if !samePublication(envelope.Reference, in.Publication) {
		return LoadPublishedBundleResult{}, stale()
	}
	if err = verifyPublishedBody(envelope.Body, in.Publication); err != nil {
		return LoadPublishedBundleResult{}, err
	}
	parsed, err := ParseTiptapDocument(ctx, op, ParseTiptapDocumentInput{envelope.Body, publishedBodyLimit})
	if err != nil {
		return LoadPublishedBundleResult{}, err
	}
	result := LoadPublishedBundleResult{Document: parsed.Document, Title: envelope.Metadata.Title, UpdatedAt: *envelope.Metadata.LastPublishedAt, MediaRefs: parsed.MediaRefs, UntranslatedBlockIDs: []string{}, Publication: in.Publication, SourceLanguage: envelope.Metadata.SourceLanguage}
	if envelope.Metadata.SEODescription != nil {
		result.Description = *envelope.Metadata.SEODescription
	}
	if in.Locale != result.SourceLanguage {
		translated, readErr := directory.Localization.ReadTranslation(ctx, map[string]any{"document": in.Publication, "targetLanguage": in.Locale})
		units := make(map[string]string)
		if readErr == nil {
			var payload struct {
				Translation *struct {
					Document       gen.PublishedRevisionRef `json:"document"`
					TargetLanguage string                   `json:"targetLanguage"`
				} `json:"translation"`
				Body  json.RawMessage `json:"body"`
				Units []struct {
					UnitID             string                   `json:"unitId"`
					Document           gen.PublishedRevisionRef `json:"document"`
					TargetLanguage     string                   `json:"targetLanguage"`
					TargetText         string                   `json:"targetText"`
					TargetHash         string                   `json:"targetHash"`
					Status             string                   `json:"status"`
					PreviousTargetText string                   `json:"previousTargetText"`
				} `json:"units"`
			}
			if json.Unmarshal(translated, &payload) != nil || payload.Translation == nil || !samePublication(payload.Translation.Document, in.Publication) || payload.Translation.TargetLanguage != in.Locale {
				return LoadPublishedBundleResult{}, dependency("译文来源无法核验")
			}
			translatedBody := payload.Body
			for _, unit := range payload.Units {
				if !samePublication(unit.Document, in.Publication) || unit.TargetLanguage != in.Locale {
					return LoadPublishedBundleResult{}, stale()
				}
				text := unit.TargetText
				if unit.Status == "fallback" {
					text = unit.PreviousTargetText
				}
				if text != "" && unit.Status != "fallback" && string(platform.HashBytes([]byte(text))) != unit.TargetHash {
					return LoadPublishedBundleResult{}, platform.NewError(platform.CodeIntegrityError, false, "TRANSLATION_HASH_MISMATCH")
				}
				units[unit.UnitID] = text
				if unit.Status == "fallback" {
					result.UntranslatedBlockIDs = append(result.UntranslatedBlockIDs, unit.UnitID)
				}
			}
			// 译文服务已按块还原好完整目标树：直接用它，保留行内格式；只有没有目标树时才逐块替换文字。
			if len(translatedBody) > 0 {
				if tree, treeErr := ParseTiptapDocument(ctx, op, ParseTiptapDocumentInput{translatedBody, publishedBodyLimit}); treeErr == nil {
					result.Document = tree.Document
					units = map[string]string{}
				}
			}
		}
		if readErr != nil {
			var problem *platform.Error
			if !errors.As(readErr, &problem) || problem.Code != platform.CodeNotFound {
				return LoadPublishedBundleResult{}, readErr
			}
		}
		var apply func(*TiptapNode)
		apply = func(node *TiptapNode) {
			id := attrString(node.Attrs, "blockId")
			text, exists := units[id]
			simple := (node.Type == "paragraph" || node.Type == "heading") && len(node.Content) == 1 && node.Content[0].Type == "text" && len(node.Content[0].Marks) == 0
			if exists && text != "" && simple {
				node.Content[0].Text = text
				return
			}
			if id != "" && nodeText(*node) != "" {
				result.UntranslatedBlockIDs = append(result.UntranslatedBlockIDs, id)
			}
			for i := range node.Content {
				apply(&node.Content[i])
			}
		}
		for i := range result.Document.Content {
			apply(&result.Document.Content[i])
		}
	}
	revision := in.Publication.Revision
	checked, err := CheckPublication(ctx, op, CheckPublicationInput{in.Publication.DocumentId, in.Locale, &revision, string(op.DataEpoch)})
	if err != nil {
		return LoadPublishedBundleResult{}, err
	}
	if !samePublication(checked.Publication, in.Publication) {
		return LoadPublishedBundleResult{}, stale()
	}
	return result, nil
}
