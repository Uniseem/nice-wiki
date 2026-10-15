package siteconfig

import (
	"context"
	"database/sql"
	"slices"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

type ReadInitializationStatusInput struct{}

// ReadInitializationStatus 返回匿名可安全读取的阶段；私有协调标识只
// 附给identity/site-config或同站点超管，不能靠读取状态重新开放注册。
func ReadInitializationStatus(ctx context.Context, op platform.OperationContext, input ReadInitializationStatusInput) (gen.ReadInitializationStatusResult, error) {
	store := currentActiveStore()
	if store == nil {
		return gen.ReadInitializationStatusResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return gen.ReadInitializationStatusResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开初始化读取事务失败")
	}
	defer tx.Rollback()
	state, initialized, err := store.ReadInitializationRecord(ctx, tx)
	if err != nil {
		return gen.ReadInitializationStatusResult{}, err
	}
	result := gen.ReadInitializationStatusResult{
		Initialized: initialized,
		State: gen.PublicInitializationState{
			Phase:     gen.PublicInitializationStatePhase(state.Phase),
			SafeError: state.SafeError,
		},
	}
	trustedService := op.CallerService == "identity" || op.CallerService == "site-config"
	sameSiteAdmin := op.Actor.Kind == "admin" && slices.Contains(op.Actor.Grants, "superadmin") && op.Actor.SiteID != "" && (state.SiteId == nil || op.Actor.SiteID == *state.SiteId)
	if trustedService || sameSiteAdmin {
		result.Coordination = &state
	}
	return result, nil
}
