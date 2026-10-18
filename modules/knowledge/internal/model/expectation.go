package model

import (
	"context"
	"nice-wiki/libs/platform"
)

type ModelExpectation struct {
	RouteVersion platform.Hash
	Prices       []platform.ExpectedPriceSnapshot
}

// WithModelExpectations 只绑定原明确批准的MG opaque路由与真实价格快照；接受事务CAS由MG拥有。
func WithModelExpectations(ctx context.Context, expectations map[string]ModelExpectation) (context.Context, error) {
	runtime, ok := ctx.Value(modelRuntimeKey{}).(ModelRuntime)
	if !ok {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "原模型批准策略未装配")
	}
	runtime.Expectations = map[string]ModelExpectation{}
	for task, value := range expectations {
		value.Prices = append([]platform.ExpectedPriceSnapshot{}, value.Prices...)
		runtime.Expectations[task] = value
	}
	return WithModelRuntime(ctx, runtime), nil
}
