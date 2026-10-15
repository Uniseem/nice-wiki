package siteconfig

import (
	"context"
	"database/sql"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

type ResolveConfigOperationInput struct {
	OperationId gen.Identifier `json:"operationId"`
	RequestHash gen.Hash       `json:"requestHash"`
}

type ResolveConfigOperationResult struct {
	State          string        `json:"state"`
	ResultRevision *gen.Revision `json:"resultRevision"`
}

// ResolveConfigOperation 按原始actor/site/op读取回执；重放不能换请求或主体。
func ResolveConfigOperation(ctx context.Context, op platform.OperationContext, input ResolveConfigOperationInput) (ResolveConfigOperationResult, error) {
	store := currentActiveStore()
	if store == nil {
		return ResolveConfigOperationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	var actorID, siteID, epoch, requestHash, state string
	var revision sql.NullInt64
	err := store.db.QueryRowContext(ctx, `SELECT actor_id,site_id,data_epoch,request_hash,state,result_revision FROM config_operations WHERE operation_id=?`, input.OperationId).Scan(&actorID, &siteID, &epoch, &requestHash, &state, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return ResolveConfigOperationResult{State: "absent"}, nil
	}
	if err != nil {
		return ResolveConfigOperationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取配置操作回执失败")
	}
	if actorID != op.Actor.SubjectID || siteID != op.Actor.SiteID || requestHash != input.RequestHash {
		return ResolveConfigOperationResult{}, platform.NewError(platform.CodeConflict, false, "配置operationId被不同请求或主体复用")
	}
	if epoch != op.DataEpoch {
		return ResolveConfigOperationResult{}, platform.NewError(platform.CodeVersionMismatch, false, "配置操作属于其他dataEpoch")
	}
	result := ResolveConfigOperationResult{State: state}
	if revision.Valid {
		value := gen.Revision(revision.Int64)
		result.ResultRevision = &value
	}
	return result, nil
}
