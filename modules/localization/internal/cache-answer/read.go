// Package cacheanswer 只交付由QA当前源修订精确引用的已完成译文。
package cacheanswer

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
)

type OperationContext = platform.OperationContext
type ReadCacheAnswerTranslationInput struct {
	TranslationId          string `json:"translationId"`
	ExpectedAnswerRevision int    `json:"expectedAnswerRevision"`
}
type ReadCacheAnswerTranslationResult = gen.ReadCacheAnswerTranslationResult

func ReadCacheAnswerTranslation(ctx context.Context, op OperationContext, input ReadCacheAnswerTranslationInput) (ReadCacheAnswerTranslationResult, error) {
	if op.CallerService != "qa" || op.Actor.Kind != "service" || op.Actor.SubjectID != "qa" {
		return ReadCacheAnswerTranslationResult{}, platform.NewError(platform.CodeForbidden, false, "缓存回答译文只供QA读取")
	}
	var result ReadCacheAnswerTranslationResult
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		t := gen.CacheAnswerTranslation{TranslationId: input.TranslationId}
		var epoch string
		err := tx.QueryRowContext(ctx, `SELECT cache_id,answer_revision,source_hash,target_language,translated_question,translated_answer,state,data_epoch FROM cache_answer_translations WHERE translation_id=?`, input.TranslationId).Scan(&t.CacheId, &t.AnswerRevision, &t.SourceHash, &t.TargetLanguage, &t.TranslatedQuestion, &t.TranslatedAnswer, &t.State, &epoch)
		if errors.Is(err, sql.ErrNoRows) {
			return platform.NewError(platform.CodeNotFound, false, "缓存回答译文不存在")
		}
		if err != nil {
			return language.StorageError(err)
		}
		if epoch != op.DataEpoch || t.AnswerRevision != input.ExpectedAnswerRevision {
			return platform.NewError(platform.CodeVersionMismatch, false, "缓存回答译文来源版本已变化")
		}
		if t.State != "ready" {
			return platform.NewError(platform.CodeNotFound, false, "缓存回答译文尚未就绪")
		}
		result.Translation = t
		return nil
	})
	return result, err
}
