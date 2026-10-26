// Package routing 提供本域免费路线元数据解析入口。
package routing

import (
	"context"
	"encoding/json"
	"net/http"
	"nice-wiki/libs/platform/problem"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/catalog"
	"nice-wiki/modules/model-gateway/internal/ownership"
)

type OperationContext = platform.OperationContext
type ResolveRouteInput = catalog.ResolveRouteInput
type ResolveRouteResult = catalog.ResolveRouteResult

// ResolveRoute 在一个SQL快照中解析配置和候选，不创建费用或派发操作。
func ResolveRoute(ctx context.Context, op OperationContext, input ResolveRouteInput) (ResolveRouteResult, error) {
	return catalog.SelectCandidates(ctx, op, input)
}

// ResolveTaskRoute是免费纯owner读取，创建入口仍负责原批准与实际事务条件。
func ResolveTaskRoute(ctx context.Context, op OperationContext, input ResolveRouteInput) (ResolveRouteResult, error) {
	if op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService {
		return ResolveRouteResult{}, platform.NewError(platform.CodeForbidden, false, "任务路线只允许目录owner纯服务读取")
	}
	options, err := gatewayruntime.ExecutionDependencies()
	if err != nil {
		return ResolveRouteResult{}, err
	}
	if options.SiteID == "" || op.Actor.SiteID == "" {
		return ResolveRouteResult{}, platform.NewError(platform.CodeForbidden, false, "任务路线须带站点")
	}
	// 先读到站点的 AI 策略（谁付费、有没有额度、有没有自带渠道），读不到就不给路线。
	if _, err := ownership.Lookup(ctx, string(op.Actor.SiteID), op.DataEpoch); err != nil {
		return ResolveRouteResult{}, err
	}
	return ResolveRoute(ctx, op, input)
}
func NewRouteHandlers() map[string]http.Handler {
	return map[string]http.Handler{"resolveTaskRoute": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := platform.OperationContextFrom(r.Context())
		var input ResolveRouteInput
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "路线读取输入不可读"), op.RequestID))
			return
		}
		result, err := ResolveTaskRoute(r.Context(), op, input)
		w.Header().Set("Cache-Control", "no-store")
		if err != nil {
			_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})}
}
