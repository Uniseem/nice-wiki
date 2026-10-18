package translation

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
	"time"
)

type PageRequest struct {
	Limit  int     `json:"limit,omitempty"`
	Cursor *string `json:"cursor,omitempty"`
}
type ReadTranslationUnitsInput struct {
	TranslationId string      `json:"translationId"`
	Page          PageRequest `json:"page"`
}
type AdminTranslationUnit struct {
	UnitId      string `json:"unitId"`
	BlockId     string `json:"blockId"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	Origin      string `json:"origin"`
	Protected   bool   `json:"protected"`
	NeedsReview bool   `json:"needsReview"`
}
type ReadTranslationUnitsResult struct {
	Units    []AdminTranslationUnit `json:"units"`
	PageInfo gen.PageInfo           `json:"pageInfo"`
}
type unitsCursor struct {
	Last      string `json:"last"`
	ExpiresAt int64  `json:"expiresAt"`
}

func ReadTranslationUnits(ctx context.Context, op OperationContext, input ReadTranslationUnitsInput) (ReadTranslationUnitsResult, error) {
	if err := language.RequireAdministrator(op); err != nil {
		return ReadTranslationUnitsResult{}, err
	}
	limit := input.Page.Limit
	if limit == 0 {
		limit = 20
	}
	var object TranslationObject
	var glossary int
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		var err error
		object, err = ReadObject(ctx, tx, op, input.TranslationId)
		if err != nil {
			return err
		}
		return language.StorageError(tx.QueryRowContext(ctx, `SELECT glossary_generation FROM domain_versions WHERE singleton=1`).Scan(&glossary))
	})
	if err != nil {
		return ReadTranslationUnitsResult{}, err
	}
	source, err := localizationruntime.FetchCurrentPublishedSnapshot(ctx, op, object.Translation.Document.DocumentId)
	if err != nil {
		return ReadTranslationUnitsResult{}, err
	}
	split, err := SplitTranslationUnits(ctx, op, SplitTranslationUnitsInput{Document: source.Reference, Source: string(source.Body), TargetLanguage: object.Translation.TargetLanguage})
	if err != nil {
		return ReadTranslationUnitsResult{}, err
	}
	binding := struct {
		Site, Subject, Session, Epoch, Translation, Target string
		SessionRevision, PermissionRevision                int64
		Source                                             gen.PublishedRevisionRef
		Generation, Glossary, Limit                        int
	}{op.Actor.SiteID, op.Actor.SubjectID, op.Actor.SessionID, op.DataEpoch, input.TranslationId, object.Translation.TargetLanguage, op.Actor.SessionRevision, op.Actor.PermissionRevision, source.Reference, object.Translation.Generation, glossary, limit}
	bindingBytes, _ := platform.CanonicalJSON(binding)
	scope := "localization.units.v1:" + platform.HashBytes(bindingBytes)
	key, err := localizationruntime.CursorKey()
	if err != nil {
		return ReadTranslationUnitsResult{}, err
	}
	cursor := unitsCursor{ExpiresAt: time.Now().UTC().Add(10 * time.Minute).Unix()}
	start := 0
	if input.Page.Cursor != nil {
		raw, verifyErr := serviceauth.VerifyCursor(key, scope, *input.Page.Cursor)
		if verifyErr != nil || json.Unmarshal(raw, &cursor) != nil || cursor.ExpiresAt <= time.Now().UTC().Unix() {
			return ReadTranslationUnitsResult{}, platform.NewError(platform.CodeStaleResult, false, "译文分页游标未知、过期或水位已变化")
		}
		found := false
		for i, u := range split.Units {
			if u.UnitId == cursor.Last {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			return ReadTranslationUnitsResult{}, platform.NewError(platform.CodeStaleResult, false, "译文分页位置已不属于固定源树")
		}
	}
	current := map[string]gen.TranslationUnit{}
	wholeProtected := map[string]bool{}
	rowProtected := map[string]bool{}
	for _, u := range object.Units {
		current[u.UnitId] = u
		if u.Protected || u.Origin == "human" {
			parent := u.UnitId
			if u.ParentBlockId != nil {
				parent = *u.ParentBlockId
			}
			if parent == u.UnitId {
				wholeProtected[parent] = true
			} else {
				rowProtected[parent] = true
			}
		}
	}
	result := ReadTranslationUnitsResult{Units: []AdminTranslationUnit{}}
	stop := start + limit
	if stop > len(split.Units) {
		stop = len(split.Units)
	}
	for _, candidate := range split.Units[start:stop] {
		view := AdminTranslationUnit{UnitId: candidate.UnitId, BlockId: candidate.UnitId, Origin: "machine"}
		if candidate.ParentBlockId != nil {
			view.BlockId = *candidate.ParentBlockId
		}
		if candidate.SourceText != nil {
			view.Source = *candidate.SourceText
		}
		if wholeProtected[view.BlockId] || (candidate.UnitId == view.BlockId && rowProtected[view.BlockId]) {
			view.Protected = true
			view.NeedsReview = true
		}
		if stored, found := current[candidate.UnitId]; found {
			view.Origin = string(stored.Origin)
			view.Protected = stored.Protected
			view.NeedsReview = stored.Status == "review" || stored.SourceHash != candidate.SourceHash
			if stored.Status != "fallback" {
				view.Target = stored.TargetText
			}
		}
		result.Units = append(result.Units, view)
	}
	result.PageInfo.HasMore = stop < len(split.Units)
	if result.PageInfo.HasMore {
		cursor.Last = split.Units[stop-1].UnitId
		raw, _ := platform.CanonicalJSON(cursor)
		next, signErr := serviceauth.SignCursor(key, scope, raw)
		if signErr != nil {
			return ReadTranslationUnitsResult{}, signErr
		}
		result.PageInfo.NextCursor = &next
	}
	if _, err = localizationruntime.CheckPublication(ctx, op, source.Reference, &source.VisibilityRevision); err != nil {
		return ReadTranslationUnitsResult{}, err
	}
	err = language.ReadData(ctx, op, func(tx *sql.Tx) error {
		var generation, g int
		var state string
		if err := tx.QueryRowContext(ctx, `SELECT generation,state FROM translation_manifests WHERE translation_id=?`, input.TranslationId).Scan(&generation, &state); err != nil {
			return language.StorageError(err)
		}
		if err := tx.QueryRowContext(ctx, `SELECT glossary_generation FROM domain_versions WHERE singleton=1`).Scan(&g); err != nil {
			return language.StorageError(err)
		}
		if generation != object.Translation.Generation || g != glossary || state != string(object.Translation.State) {
			return platform.NewError(platform.CodeStaleResult, false, "译文分页期间目标或术语状态变化")
		}
		return nil
	})
	if err != nil {
		return ReadTranslationUnitsResult{}, err
	}
	return result, nil
}
