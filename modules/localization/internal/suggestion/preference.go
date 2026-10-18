package suggestion

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	interfacecatalog "nice-wiki/modules/localization/internal/interface"
	"nice-wiki/modules/localization/internal/language"
	"time"
)

type OperationContext = platform.OperationContext
type SaveLanguagePreferenceInput = gen.SaveLanguagePreferenceInput
type SaveLanguagePreferenceResult = gen.SaveLanguagePreferenceResult

// SaveLanguagePreference 仅保存当前访客自己的明确选择，不记录推测信号或访问轨迹。
func SaveLanguagePreference(ctx context.Context, op OperationContext, input SaveLanguagePreferenceInput) (SaveLanguagePreferenceResult, error) {
	if op.Actor.Kind != "visitor" || op.Actor.SubjectID != input.VisitorSubject {
		return SaveLanguagePreferenceResult{}, platform.NewError(platform.CodeForbidden, false, "语言偏好只能由同一伪名访客保存")
	}
	var result SaveLanguagePreferenceResult
	err := language.Mutate(ctx, op, "localization.SaveLanguagePreference", input.IdempotencyKey, input, &result, func(tx *sql.Tx) error {
		target, err := interfacecatalog.EnabledLanguage(ctx, tx, input.Language)
		if err != nil {
			var domain *platform.Error
			if errors.As(err, &domain) && domain.Code == platform.CodeNotFound {
				return platform.NewError(platform.CodeValidation, false, "偏好语言未启用")
			}
			return err
		}
		// 回执重放沿用第一次到期时刻，不能借重试无限延长持久期。
		result = SaveLanguagePreferenceResult{Language: target, ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour)}
		_, err = tx.ExecContext(ctx, `INSERT INTO language_preferences(visitor_subject,language,expires_at) VALUES(?,?,?) ON CONFLICT(visitor_subject) DO UPDATE SET language=excluded.language,expires_at=excluded.expires_at`, input.VisitorSubject, target, result.ExpiresAt.Format(time.RFC3339Nano))
		return language.StorageError(err)
	})
	return result, err
}

func NewPreferenceHandlers() map[string]http.Handler {
	return map[string]http.Handler{"saveLanguagePreference": language.JSONHandler(SaveLanguagePreference)}
}
