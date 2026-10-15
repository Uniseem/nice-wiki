package siteconfig

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

// CheckWriteEpochInput 是业务写入前epoch与屏障检查的输入。
type CheckWriteEpochInput struct {
	DataEpoch gen.Identifier `json:"dataEpoch"`
}

// CheckWriteEpochResult 是epoch与屏障检查结果。
type CheckWriteEpochResult struct {
	Allowed bool `json:"allowed"`
}

// CheckWriteEpoch 是业务域提交前使用的只读控制门；本工作包只实现
// control.db 判定，不打开业务库或登记在途事务。
func CheckWriteEpoch(ctx context.Context, op platform.OperationContext, input CheckWriteEpochInput) (CheckWriteEpochResult, error) {
	if err := checkOperationDeadline(ctx, op); err != nil {
		return CheckWriteEpochResult{}, err
	}
	store := activeRuntimeStore()
	if store == nil {
		return CheckWriteEpochResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库不可用")
	}
	state, err := store.readState(ctx)
	if err != nil {
		return CheckWriteEpochResult{}, err
	}
	if state.Phase != gen.Running {
		return CheckWriteEpochResult{}, platform.NewError(platform.CodeLocked, false, "运行状态处于维护阶段")
	}
	if input.DataEpoch == "" || input.DataEpoch != state.DataEpoch || (op.DataEpoch != "" && op.DataEpoch != input.DataEpoch) {
		return CheckWriteEpochResult{}, platform.NewError(platform.CodeVersionMismatch, false, "写入dataEpoch已变化")
	}
	return CheckWriteEpochResult{Allowed: true}, nil
}
