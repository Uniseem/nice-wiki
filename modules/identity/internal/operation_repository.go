// Package identity 提供身份域的受信私有仓库上下文。
package identity

import (
	"context"
	"errors"
	"time"

	"nice-wiki/libs/platform"
)

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
