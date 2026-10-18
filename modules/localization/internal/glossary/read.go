// Package glossary 管理人工/批准术语和固定代数；不反向请求memory。
package glossary

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
	"sort"
	"unicode/utf8"
)

type OperationContext = platform.OperationContext
type ListGlossaryInput struct {
	SourceLanguage string  `json:"sourceLanguage"`
	TargetLanguage string  `json:"targetLanguage"`
	Cursor         *string `json:"cursor,omitempty"`
	Limit          int     `json:"limit"`
}
type ListGlossaryResult = gen.ListGlossaryResult
type ResolveGlossaryInput struct {
	SourceLanguage string `json:"sourceLanguage"`
	TargetLanguage string `json:"targetLanguage"`
	DocumentId     string `json:"documentId"`
	Generation     int    `json:"generation"`
}
type ResolveGlossaryResult struct {
	Terms      []gen.GlossaryTerm `json:"terms"`
	Generation int                `json:"generation"`
}

const termColumns = `term_id,version,source_language,target_language,source_term,target_term,scope,document_id,generation,source,extraction_batch_id`

type rowScanner interface{ Scan(...any) error }

func scanTerm(row rowScanner) (gen.GlossaryTerm, error) {
	var t gen.GlossaryTerm
	var document, batch sql.NullString
	err := row.Scan(&t.TermId, &t.Version, &t.SourceLanguage, &t.TargetLanguage, &t.SourceTerm, &t.TargetTerm, &t.Scope, &document, &t.Generation, &t.Source, &batch)
	if document.Valid {
		t.DocumentId = &document.String
	}
	if batch.Valid {
		t.ExtractionBatchId = &batch.String
	}
	if errors.Is(err, sql.ErrNoRows) {
		return t, platform.NewError(platform.CodeNotFound, false, "术语不存在")
	}
	return t, language.StorageError(err)
}
func Generation(ctx context.Context, tx *sql.Tx) (int, error) {
	var g int
	err := tx.QueryRowContext(ctx, `SELECT glossary_generation FROM domain_versions WHERE singleton=1`).Scan(&g)
	return g, language.StorageError(err)
}

type glossaryCursor struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Generation int    `json:"generation"`
	Last       string `json:"last"`
	Epoch      string `json:"epoch"`
}

func ListGlossary(ctx context.Context, op OperationContext, input ListGlossaryInput) (ListGlossaryResult, error) {
	a, normalizeErr := language.NormalizeLanguageTag(ctx, op, language.NormalizeLanguageTagInput{Tag: input.SourceLanguage})
	if normalizeErr != nil {
		return ListGlossaryResult{}, normalizeErr
	}
	b, normalizeErr := language.NormalizeLanguageTag(ctx, op, language.NormalizeLanguageTagInput{Tag: input.TargetLanguage})
	if normalizeErr != nil {
		return ListGlossaryResult{}, normalizeErr
	}
	input.SourceLanguage = a.Tag
	input.TargetLanguage = b.Tag

	if err := language.RequireAdministrator(op); err != nil {
		return ListGlossaryResult{}, err
	}
	result := ListGlossaryResult{Terms: []gen.GlossaryTerm{}}
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		generation, err := Generation(ctx, tx)
		if err != nil {
			return err
		}
		result.Generation = generation
		cursor := glossaryCursor{Source: input.SourceLanguage, Target: input.TargetLanguage, Generation: generation, Epoch: op.DataEpoch}
		if input.Cursor != nil {
			raw, err := base64.RawURLEncoding.DecodeString(*input.Cursor)
			if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Source != input.SourceLanguage || cursor.Target != input.TargetLanguage || cursor.Epoch != op.DataEpoch {
				return platform.NewError(platform.CodeValidation, false, "术语分页游标不匹配")
			}
			if cursor.Generation != generation {
				return platform.NewError(platform.CodeVersionMismatch, false, "术语已变化，请重新查询")
			}
		}
		rows, err := tx.QueryContext(ctx, `SELECT `+termColumns+` FROM glossary WHERE source_language=? AND target_language=? AND term_id>? AND deleted=0 AND data_epoch=? ORDER BY term_id LIMIT ?`, input.SourceLanguage, input.TargetLanguage, cursor.Last, op.DataEpoch, input.Limit+1)
		if err != nil {
			return language.StorageError(err)
		}
		defer rows.Close()
		for rows.Next() {
			t, err := scanTerm(rows)
			if err != nil {
				return err
			}
			result.Terms = append(result.Terms, t)
		}
		if err = rows.Err(); err != nil {
			return language.StorageError(err)
		}
		if len(result.Terms) > input.Limit {
			result.Terms = result.Terms[:input.Limit]
			cursor.Last = result.Terms[len(result.Terms)-1].TermId
			raw, _ := json.Marshal(cursor)
			next := base64.RawURLEncoding.EncodeToString(raw)
			result.NextCursor = &next
		}
		return nil
	})
	return result, err
}
func ResolveGlossary(ctx context.Context, op OperationContext, input ResolveGlossaryInput) (ResolveGlossaryResult, error) {
	a, normalizeErr := language.NormalizeLanguageTag(ctx, op, language.NormalizeLanguageTagInput{Tag: input.SourceLanguage})
	if normalizeErr != nil {
		return ResolveGlossaryResult{}, normalizeErr
	}
	b, normalizeErr := language.NormalizeLanguageTag(ctx, op, language.NormalizeLanguageTagInput{Tag: input.TargetLanguage})
	if normalizeErr != nil {
		return ResolveGlossaryResult{}, normalizeErr
	}
	input.SourceLanguage = a.Tag
	input.TargetLanguage = b.Tag

	var result ResolveGlossaryResult
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		generation, err := Generation(ctx, tx)
		if err != nil {
			return err
		}
		if generation != input.Generation {
			return platform.NewError(platform.CodeVersionMismatch, false, "固定术语代数已变化")
		}
		result.Generation = generation
		rows, err := tx.QueryContext(ctx, `SELECT `+termColumns+` FROM glossary WHERE source_language=? AND target_language=? AND deleted=0 AND data_epoch=? AND (scope='global' OR (scope='document' AND document_id=?)) ORDER BY term_id`, input.SourceLanguage, input.TargetLanguage, op.DataEpoch, input.DocumentId)
		if err != nil {
			return language.StorageError(err)
		}
		defer rows.Close()
		bySource := map[string]gen.GlossaryTerm{}
		scopeSeen := map[string]bool{}
		for rows.Next() {
			t, err := scanTerm(rows)
			if err != nil {
				return err
			}
			key := string(t.Scope) + ":" + t.SourceTerm
			if scopeSeen[key] {
				return platform.NewError(platform.CodeIntegrityError, false, "同范围原词存在冲突术语")
			}
			scopeSeen[key] = true
			prior, exists := bySource[t.SourceTerm]
			if !exists || (prior.Scope == "global" && t.Scope == "document") {
				bySource[t.SourceTerm] = t
			}
		}
		if err = rows.Err(); err != nil {
			return language.StorageError(err)
		}
		result.Terms = []gen.GlossaryTerm{}
		for _, t := range bySource {
			result.Terms = append(result.Terms, t)
		}
		sort.Slice(result.Terms, func(i, j int) bool {
			a, b := result.Terms[i], result.Terms[j]
			la, lb := utf8.RuneCountInString(a.SourceTerm), utf8.RuneCountInString(b.SourceTerm)
			if la != lb {
				return la > lb
			}
			if a.SourceTerm != b.SourceTerm {
				return a.SourceTerm < b.SourceTerm
			}
			return a.TermId < b.TermId
		})
		return nil
	})
	return result, err
}
