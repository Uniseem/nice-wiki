package internal

import (
	"context"

	"nice-wiki/libs/platform"
)

func ReadRuntimeState(ctx context.Context, op OperationContext, input ReadRuntimeStateInput) (ReadRuntimeStateResult, error) {
	runtime, ok := proxyRuntimeFromContext(ctx)
	if !ok || runtime.Runtime == nil {
		return ReadRuntimeStateResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行状态依赖未装配")
	}
	view, err := runtime.Runtime.ReadRuntimeState(ctx)
	if err != nil {
		return ReadRuntimeStateResult{}, err
	}
	if view.DataEpoch == "" || view.Phase == "" {
		return ReadRuntimeStateResult{}, platform.NewError(platform.CodeIntegrityError, false, "运行状态不完整")
	}
	return ReadRuntimeStateResult{DataEpoch: view.DataEpoch, Maintenance: view.Maintenance, Revision: 0}, nil
}
