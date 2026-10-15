package identity

import (
	"context"
	"database/sql"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/identity/internal/gen"
)

// PortalPreferences 只保存本人界面选择，不承载业务授权或会话秘密。
type PortalPreferences struct {
	Theme        string            `json:"theme"`
	Locale       string            `json:"locale"`
	DefaultRoute string            `json:"defaultRoute"`
	Revision     platform.Revision `json:"revision"`
}

type ReadPreferencesInput struct{}
type ReadPreferencesResult struct {
	Preferences PortalPreferences `json:"preferences"`
}
type UpdatePreferencesInput struct {
	Preferences      PortalPreferences `json:"preferences"`
	ExpectedRevision platform.Revision `json:"expectedRevision"`
}
type UpdatePreferencesResult struct {
	Preferences PortalPreferences `json:"preferences"`
}

// ReadPreferences 在同一授权快照读取本人偏好；未保存过时不创建默认行。
func ReadPreferences(ctx context.Context, op OperationContext, input ReadPreferencesInput) (ReadPreferencesResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return ReadPreferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "偏好私有存储尚未装配")
	}
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ReadPreferencesResult{}, identityDBError()
	}
	defer tx.Rollback()
	if _, _, err := readActorAccount(ctx, op, tx, "portal.use", "identity:readPreferences"); err != nil {
		return ReadPreferencesResult{}, err
	}
	preferences, _, err := readPreferenceRecord(ctx, tx, op.Actor.SubjectID)
	if err != nil {
		return ReadPreferencesResult{}, err
	}
	return ReadPreferencesResult{Preferences: preferences}, nil
}

// UpdatePreferences 只在本人事务写偏好与原安全结果，不改变账号或认证水位。
func UpdatePreferences(ctx context.Context, op OperationContext, input UpdatePreferencesInput) (UpdatePreferencesResult, error) {
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return UpdatePreferencesResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "偏好私有存储尚未装配")
	}
	digest, keyID, err := privateIdentityDigest(ctx, op, "updatePreferences", input)
	if err != nil {
		return UpdatePreferencesResult{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return UpdatePreferencesResult{}, identityDBError()
	}
	defer tx.Rollback()
	_, account, err := readActorAccount(ctx, op, tx, "portal.use", "identity:updatePreferences")
	if err != nil {
		return UpdatePreferencesResult{}, err
	}
	var result UpdatePreferencesResult
	found, err := readIdentityReceipt(ctx, op, tx, digest, keyID, &result)
	if err != nil {
		return UpdatePreferencesResult{}, err
	}
	if found {
		return result, nil
	}
	if input.Preferences.Revision != input.ExpectedRevision {
		return UpdatePreferencesResult{}, platform.NewError(platform.CodeValidation, false, "偏好输入版本必须等于expectedRevision")
	}
	if err := validatePreferenceSelection(input.Preferences, account); err != nil {
		return UpdatePreferencesResult{}, err
	}
	current, exists, err := readPreferenceRecord(ctx, tx, op.Actor.SubjectID)
	if err != nil {
		return UpdatePreferencesResult{}, err
	}
	if current.Revision != input.ExpectedRevision {
		return UpdatePreferencesResult{}, platform.NewError(platform.CodeVersionMismatch, false, "偏好版本已变化")
	}
	result.Preferences = input.Preferences
	result.Preferences.Revision = current.Revision + 1
	if exists {
		changed, err := tx.ExecContext(ctx, `UPDATE preferences SET theme=?,locale=?,default_route=?,revision=? WHERE account_id=? AND revision=?`,
			result.Preferences.Theme, result.Preferences.Locale, result.Preferences.DefaultRoute, result.Preferences.Revision, op.Actor.SubjectID, input.ExpectedRevision)
		if err != nil {
			return UpdatePreferencesResult{}, identityDBError()
		}
		if n, err := changed.RowsAffected(); err != nil || n != 1 {
			return UpdatePreferencesResult{}, platform.NewError(platform.CodeVersionMismatch, false, "偏好版本已变化")
		}
	} else {
		if _, err := tx.ExecContext(ctx, `INSERT INTO preferences(account_id,theme,locale,default_route,revision) VALUES(?,?,?,?,?)`,
			op.Actor.SubjectID, result.Preferences.Theme, result.Preferences.Locale, result.Preferences.DefaultRoute, result.Preferences.Revision); err != nil {
			return UpdatePreferencesResult{}, identityDBError()
		}
	}
	if err := saveIdentityReceipt(ctx, op, tx, digest, keyID, result); err != nil {
		return UpdatePreferencesResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return UpdatePreferencesResult{}, identityDBError()
	}
	return result, nil
}

func readPreferenceRecord(ctx context.Context, query sessionQueryer, accountID string) (PortalPreferences, bool, error) {
	var preferences PortalPreferences
	err := query.QueryRowContext(ctx, `SELECT theme,locale,default_route,revision FROM preferences WHERE account_id=?`, accountID).
		Scan(&preferences.Theme, &preferences.Locale, &preferences.DefaultRoute, &preferences.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return PortalPreferences{Theme: "system", Locale: "zh-Hans", DefaultRoute: "dashboard", Revision: 0}, false, nil
	}
	if err != nil {
		return PortalPreferences{}, false, identityDBError()
	}
	return preferences, true, nil
}

// validatePreferenceSelection 只核目标页当前权限；允许的locale/key已由公共入口校验。
func validatePreferenceSelection(preferences PortalPreferences, account Account) error {
	mapped, err := MapFunctionGrants(MapFunctionGrantsInput{Permissions: toFunctionPermissions(account.Permissions), IsSuperadmin: account.IsSuperadmin})
	if err != nil {
		return err
	}
	for _, route := range gen.AdminRoutes {
		if route.Key != preferences.DefaultRoute {
			continue
		}
		for _, permission := range route.Permissions {
			if hasGrant(mapped.Grants, permission) {
				return nil
			}
		}
		break
	}
	return platform.NewError(platform.CodeForbidden, false, "当前账户没有默认后台页所需权限")
}
