// Package identity 提供身份域的幂等回执读取边界。
package identity

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"nice-wiki/libs/platform"
)

// ResolveIdentityOperationInput 是幂等回执查询参数。
type ResolveIdentityOperationInput struct {
	OperationID platform.Identifier `json:"operationId"`
	RequestHash platform.Hash       `json:"requestHash"`
}

// ResolveIdentityOperationResult 是幂等回执的安全摘要。
type ResolveIdentityOperationResult struct {
	State      string         `json:"state"`
	ResultHash *platform.Hash `json:"resultHash"`
}

// identityStoreContextKey 只供本域启动装配把受信私有存储交给业务函数。
type identityStoreContextKey struct{}

// WithIdentityStore 将本域私有存储放入受信调用上下文；不会把句柄编码到网络请求。
func WithIdentityStore(ctx context.Context, store *Store) context.Context {
	return context.WithValue(ctx, identityStoreContextKey{}, store)
}

func identityStoreFromContext(ctx context.Context) (*Store, bool) {
	store, ok := ctx.Value(identityStoreContextKey{}).(*Store)
	return store, ok && store != nil
}

// ResolveIdentityOperation 读取指定站点、主体和操作的幂等回执。
// 只返回状态与结果哈希，不返回请求正文或任何凭据。
func ResolveIdentityOperation(ctx context.Context, op OperationContext, input ResolveIdentityOperationInput) (ResolveIdentityOperationResult, error) {
	if err := checkIdentityRepositoryContext(ctx, op); err != nil {
		return ResolveIdentityOperationResult{}, err
	}
	if !platform.ValidIdentifier(input.OperationID) || !platform.ValidHash(input.RequestHash) {
		return ResolveIdentityOperationResult{}, platform.NewError(platform.CodeValidation, false, "幂等回执查询参数不合法")
	}
	if !platform.ValidIdentifier(op.Actor.SiteID) || !platform.ValidIdentifier(op.Actor.SubjectID) {
		return ResolveIdentityOperationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "幂等回执主体不可用")
	}

	store, ok := identityStoreFromContext(ctx)
	if !ok || store.db == nil {
		return ResolveIdentityOperationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份私有存储尚未装配")
	}

	var inputDigest, state, resultHash string
	err := store.db.QueryRowContext(ctx, `
		SELECT input_digest, state, result_hash
		FROM operation_receipts
		WHERE site_id = ? AND actor_id = ? AND operation_id = ?`,
		op.Actor.SiteID, op.Actor.SubjectID, input.OperationID,
	).Scan(&inputDigest, &state, &resultHash)
	if errors.Is(err, sql.ErrNoRows) {
		return ResolveIdentityOperationResult{State: "absent", ResultHash: nil}, nil
	}
	if err != nil {
		return ResolveIdentityOperationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "幂等回执不可读")
	}
	if inputDigest != input.RequestHash {
		return ResolveIdentityOperationResult{}, platform.NewError(platform.CodeConflict, false, "幂等操作摘要不一致")
	}

	switch state {
	case "in_progress":
		return ResolveIdentityOperationResult{State: state, ResultHash: nil}, nil
	case "committed":
		if !platform.ValidHash(resultHash) {
			return ResolveIdentityOperationResult{}, platform.NewError(platform.CodeConflict, false, "幂等回执结果摘要不一致")
		}
		hash := platform.Hash(resultHash)
		return ResolveIdentityOperationResult{State: state, ResultHash: &hash}, nil
	default:
		return ResolveIdentityOperationResult{}, platform.NewError(platform.CodeConflict, false, "幂等回执状态不一致")
	}
}

// checkIdentityRepositoryContext 校验所有私有事务函数的取消、期限和数据代次。
func checkIdentityRepositoryContext(ctx context.Context, op OperationContext) error {
	if ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "身份操作上下文为空")
	}
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "身份操作已超过期限")
		}
		return platform.NewError(platform.CodeCancelled, true, "身份操作已取消")
	default:
	}
	if !platform.ValidIdentifier(op.OperationID) || !platform.ValidIdentifier(op.DataEpoch) {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "身份操作缺少可信代次或操作标识")
	}
	if op.Deadline == "" {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "身份操作缺少期限")
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "身份操作期限不可用")
	}
	if !time.Now().UTC().Before(deadline) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "身份操作已超过期限")
	}
	return nil
}
