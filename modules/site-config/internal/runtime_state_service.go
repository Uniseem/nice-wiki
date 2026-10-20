package siteconfig

import (
	"context"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

// ReadRuntimeStateInput 是读取独立控制库权威运行状态的输入；无请求字段。
type ReadRuntimeStateInput struct{}

// ReadRuntimeState 读取 control.db 当前 revision，并返回经过完整性校验的
// RuntimeState；未知或不可读状态一律失败，不转成允许状态。
func ReadRuntimeState(ctx context.Context, op platform.OperationContext, input ReadRuntimeStateInput) (gen.ReadRuntimeStateResult, error) {
	_ = input
	if err := checkOperationDeadline(ctx, op); err != nil {
		return gen.ReadRuntimeStateResult{}, err
	}
	store := activeRuntimeStore()
	if store == nil {
		return gen.ReadRuntimeStateResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库不可用")
	}
	state, err := store.readState(ctx)
	if err != nil {
		return gen.ReadRuntimeStateResult{}, err
	}
	if string(state.DataEpoch) != string(op.DataEpoch) {
		return gen.ReadRuntimeStateResult{}, platform.NewError(platform.CodeVersionMismatch, false, "运行状态dataEpoch已变化")
	}
	return gen.ReadRuntimeStateResult{State: state}, nil
}

func checkOperationDeadline(ctx context.Context, op platform.OperationContext) error {
	if err := ctx.Err(); err != nil {
		if err == context.DeadlineExceeded {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "操作期限已到")
		}
		return platform.NewError(platform.CodeCancelled, true, "操作已取消")
	}
	if op.Deadline != "" {
		deadline, err := platform.ParseTimestamp(op.Deadline)
		if err != nil {
			return platform.NewError(platform.CodeValidation, false, "操作期限格式不合法")
		}
		if time.Now().After(deadline) {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "操作期限已到")
		}
	}
	return nil
}
