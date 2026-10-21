package translation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
)

type CancelTranslationInput = gen.CancelTranslationInput
type CancelTranslationResult = gen.CancelTranslationResult

// CancelTranslation 管理员取消排队或进行中的译文：置为 cancelled，执行协程在下一批调用模型之前发现后停止，不再产生新的费用。
// 已经发出的那一批调用的费用无法撤销。已完成/已取消的译文不能取消。
func CancelTranslation(ctx context.Context, op OperationContext, input CancelTranslationInput) (CancelTranslationResult, error) {
	if err := language.RequireAdministrator(op); err != nil {
		return CancelTranslationResult{}, err
	}
	var result CancelTranslationResult
	err := language.Mutate(ctx, op, "localization.CancelTranslation", input.IdempotencyKey, input, &result, func(tx *sql.Tx) error {
		var ref, target, state string
		err := tx.QueryRowContext(ctx, `SELECT publication_ref,target_language,state FROM translation_manifests WHERE translation_id=? AND data_epoch=?`, input.TranslationId, op.DataEpoch).Scan(&ref, &target, &state)
		if errors.Is(err, sql.ErrNoRows) {
			return platform.NewError(platform.CodeNotFound, false, "译文不存在")
		}
		if err != nil {
			return language.StorageError(err)
		}
		if state != "queued" && state != "running" {
			return platform.NewError(platform.CodeConflict, false, "只有排队或进行中的译文可以取消")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE translation_manifests SET state='cancelled',updated_at=? WHERE translation_id=? AND state IN('queued','running')`, platform.Now(), input.TranslationId); err != nil {
			return language.StorageError(err)
		}
		var document gen.PublishedRevisionRef
		if json.Unmarshal([]byte(ref), &document) != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "译文来源引用损坏")
		}
		var total int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM translation_units WHERE translation_id=?`, input.TranslationId).Scan(&total); err != nil {
			return language.StorageError(err)
		}
		result = CancelTranslationResult{Translation: gen.Translation{TranslationId: input.TranslationId, Document: document, TargetLanguage: target, State: gen.TranslationState("cancelled"), Total: total, MemoryVersionHash: platform.HashBytes(nil)}}
		return nil
	})
	return result, err
}

// translationCancelled 执行协程在每批调用模型之前检查，被取消就停止。
func translationCancelled(ctx context.Context, op OperationContext, translationID string) bool {
	cancelled := false
	_ = language.ReadData(ctx, op, func(tx *sql.Tx) error {
		var state string
		if err := tx.QueryRowContext(ctx, `SELECT state FROM translation_manifests WHERE translation_id=?`, translationID).Scan(&state); err == nil {
			cancelled = state == "cancelled"
		}
		return nil
	})
	return cancelled
}
