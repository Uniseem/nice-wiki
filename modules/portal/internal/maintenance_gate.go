package internal

import (
	"context"
	"sync"
	"time"

	"nice-wiki/libs/platform"
)

// Gate 是进程内入口屏障；权威维护状态仍来自 site-config，Gate 不能单独
// 把维护状态改为开放。
type Gate struct {
	mu        sync.Mutex
	barrierID Identifier
	dataEpoch Identifier
	closed    bool
}

func NewGate() *Gate { return &Gate{} }

func (g *Gate) isClosed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closed
}

func CheckEntranceGate(ctx context.Context, op OperationContext, input CheckEntranceGateInput) (CheckEntranceGateResult, error) {
	current, err := ReadRuntimeState(ctx, op, ReadRuntimeStateInput{})
	if err != nil {
		return CheckEntranceGateResult{}, err
	}
	if input.RuntimeRevision != 0 && input.RuntimeRevision != current.Revision {
		return CheckEntranceGateResult{}, platform.NewError(platform.CodeVersionMismatch, false, "运行状态版本已变化")
	}
	if runtime, ok := proxyRuntimeFromContext(ctx); ok && runtime.Gate != nil && runtime.Gate.isClosed() {
		if !maintenanceControlRoute(input.Route) {
			return CheckEntranceGateResult{}, platform.NewError(platform.CodeLocked, true, "入口屏障已关闭")
		}
	}
	if current.Maintenance && !maintenanceControlRoute(input.Route) {
		return CheckEntranceGateResult{}, platform.NewError(platform.CodeLocked, true, "站点正在维护")
	}
	return CheckEntranceGateResult{Allowed: true, RetryAfterSeconds: 5}, nil
}

func AcquireBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error) {
	if op.CallerService != "data-protection" {
		return BarrierStatus{}, platform.NewError(platform.CodeForbidden, false, "仅data-protection可取得入口屏障")
	}
	if !platform.ValidIdentifier(input.BarrierID) || !platform.ValidIdentifier(input.OperationID) || input.Deadline == "" {
		return BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "屏障请求不完整")
	}
	deadline, err := platform.ParseTimestamp(input.Deadline)
	if err != nil || !deadline.After(time.Now()) {
		return BarrierStatus{}, platform.NewError(platform.CodeDeadlineExceeded, true, "屏障期限已过")
	}
	runtime, ok := proxyRuntimeFromContext(ctx)
	if !ok || runtime.Runtime == nil || runtime.Gate == nil {
		return BarrierStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "屏障依赖未装配")
	}
	view, err := runtime.Runtime.ReadRuntimeState(ctx)
	if err != nil {
		return BarrierStatus{}, err
	}
	if !view.Maintenance {
		return BarrierStatus{}, platform.NewError(platform.CodeLocked, true, "site-config尚未进入维护")
	}
	runtime.Gate.mu.Lock()
	defer runtime.Gate.mu.Unlock()
	if runtime.Gate.closed && runtime.Gate.barrierID != input.BarrierID {
		return BarrierStatus{}, platform.NewError(platform.CodeLocked, false, "已有其他入口屏障")
	}
	runtime.Gate.closed = true
	runtime.Gate.barrierID = input.BarrierID
	runtime.Gate.dataEpoch = view.DataEpoch
	return BarrierStatus{BarrierID: input.BarrierID, DataEpoch: view.DataEpoch, Owner: "portal", State: "safe"}, nil
}

func ReleaseBarrier(ctx context.Context, op OperationContext, input ReleaseBarrierInput) (ReleaseBarrierResult, error) {
	if op.CallerService != "data-protection" {
		return ReleaseBarrierResult{}, platform.NewError(platform.CodeForbidden, false, "仅data-protection可释放入口屏障")
	}
	runtime, ok := proxyRuntimeFromContext(ctx)
	if !ok || runtime.Runtime == nil || runtime.Gate == nil {
		return ReleaseBarrierResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "屏障依赖未装配")
	}
	view, err := runtime.Runtime.ReadRuntimeState(ctx)
	if err != nil {
		return ReleaseBarrierResult{}, err
	}
	runtime.Gate.mu.Lock()
	defer runtime.Gate.mu.Unlock()
	if !runtime.Gate.closed || runtime.Gate.barrierID != input.BarrierID {
		return ReleaseBarrierResult{}, platform.NewError(platform.CodeLocked, false, "屏障不存在")
	}
	if runtime.Gate.dataEpoch != input.ExpectedEpoch || view.DataEpoch != input.ExpectedEpoch {
		return ReleaseBarrierResult{}, platform.NewError(platform.CodeVersionMismatch, false, "屏障代次不匹配")
	}
	if view.Maintenance {
		return ReleaseBarrierResult{}, platform.NewError(platform.CodeLocked, true, "维护尚未解除")
	}
	runtime.Gate.closed = false
	status := BarrierStatus{BarrierID: input.BarrierID, DataEpoch: view.DataEpoch, Owner: "portal", State: "released"}
	return ReleaseBarrierResult{Status: status}, nil
}

func maintenanceControlRoute(route ResolvedRoute) bool {
	path := route.Definition.PathTemplate
	if path == "/healthz" || path == "/readyz" {
		return true
	}
	for _, prefix := range []string{"/v1/registration", "/v1/sessions", "/v1/restores"} {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
