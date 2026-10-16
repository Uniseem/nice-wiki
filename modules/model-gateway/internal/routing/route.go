// Package routing 提供本域免费路线元数据解析入口。
package routing

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/catalog"
)

type OperationContext = platform.OperationContext
type ResolveRouteInput = catalog.ResolveRouteInput
type ResolveRouteResult = catalog.ResolveRouteResult

// ResolveRoute 在一个SQL快照中解析配置和候选，不创建费用或派发操作。
func ResolveRoute(ctx context.Context, op OperationContext, input ResolveRouteInput) (ResolveRouteResult, error) {
	return catalog.SelectCandidates(ctx, op, input)
}
