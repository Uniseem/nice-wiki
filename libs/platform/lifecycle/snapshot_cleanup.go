package lifecycle

import (
	"context"
	"nice-wiki/libs/platform"
	"time"
)

// IssueCleanup只由Manager发出的活跃私有租约实现，随机claim scope不能发行。
func (lease *snapshotAdmission) IssueCleanup(ctx context.Context, request platform.CleanupRequest) (*platform.CleanupScope, error) {
	m := lease.manager
	view, err := m.config.Runtime(platform.WithoutCleanupIssuer(ctx))
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if request.SourceService != m.config.Owner {
		return nil, platform.NewError(platform.CodeForbidden, false, "清理源服务不符")
	}
	if view.Phase == platform.PhaseRunning && !view.Maintenance && (m.record == nil || m.record.Status.State == "released") {
		if !lease.active {
			return nil, platform.NewError(platform.CodeLocked, true, "原在途租约已结束")
		}
		return nil, nil
	}
	var route *platform.CleanupRoute
	resource := ""
	for i := range m.config.CleanupRoutes {
		candidate := &m.config.CleanupRoutes[i]
		value, match, matchErr := platform.MatchCleanupRoute(*candidate, request)
		if matchErr != nil {
			return nil, matchErr
		}
		if match {
			if route != nil {
				return nil, integrity("清理路由策略重叠")
			}
			route = candidate
			resource = value
		}
	}
	if route == nil {
		return nil, platform.NewError(platform.CodeLocked, true, "维护期间该出站操作不属于固定清理或控制端口")
	}
	if route.Control {
		return nil, nil
	}
	if (lease.purpose == "event_broker") != route.BrokerControl {
		return nil, platform.NewError(platform.CodeForbidden, false, "事件控制根与原付费清理根不可混用")
	}
	if !lease.active || !lease.bound || lease.purpose == "job_claim" || m.record == nil || m.record.Status.State != "draining" || view.BarrierID != m.record.Input.BarrierID || view.DataEpoch != lease.epoch || view.DataEpoch != m.record.Epoch || !view.Maintenance || (view.Phase != platform.PhaseBackupBarrier && view.Phase != platform.PhaseRestoreStaging) {
		return nil, platform.NewError(platform.CodeLocked, true, "原执行清理发行窗口已关闭")
	}
	op, ok := platform.OperationContextFrom(ctx)
	original := lease.operation
	if !ok || op.RootOperationID != original.RootOperationID || op.Actor.SiteID != original.Actor.SiteID || op.DataEpoch != original.DataEpoch || op.CostClass != original.CostClass {
		return nil, platform.NewError(platform.CodeForbidden, false, "清理必须保持原执行根与费用作用域")
	}
	until := time.Now().UTC().Add(30 * time.Second)
	if m.record.Input.Deadline.Before(until) {
		until = m.record.Input.Deadline
	}
	if !lease.deadline.IsZero() && lease.deadline.Before(until) {
		until = lease.deadline
	}
	if current, ok := ctx.Deadline(); ok && current.Before(until) {
		until = current
	}
	if !until.After(time.Now()) {
		return nil, platform.NewError(platform.CodeDeadlineExceeded, false, "清理窗口已过期")
	}
	m.signalLocked()
	return &platform.CleanupScope{EventControl: route.BrokerControl, Version: 1, BarrierID: m.record.Input.BarrierID, DataEpoch: lease.epoch, Deadline: until.Format(time.RFC3339Nano), SourceService: m.config.Owner, TargetService: request.TargetService, Action: route.Action, ResourceID: resource, RootOperationID: original.RootOperationID, SiteID: original.Actor.SiteID, CostClass: original.CostClass, QueryHash: platform.HashBytes([]byte(request.RawQuery))}, nil
}

// ExistingWorkContext仅从尚未release的真实旧执行分叉独立有界清理租约，原用户proof不被复活。
func (m *SnapshotManager) ExistingWorkContext(ctx context.Context) (context.Context, func(), error) {
	original, ok := ctx.Value(snapshotAdmissionKey{}).(*snapshotAdmission)
	if !ok || original.manager != m {
		return nil, nil, platform.NewError(platform.CodeLocked, false, "清理缺原私有执行租约")
	}
	read, cancelRead := context.WithTimeout(platform.WithoutCleanupIssuer(platform.WithPortalProof(context.WithoutCancel(ctx), "")), 2*time.Second)
	if op, ok := platform.OperationContextFrom(read); ok {
		until, _ := read.Deadline()
		op.OperationID = platform.NewIdentifier()
		op.Deadline = until.UTC().Format(time.RFC3339Nano)
		read = platform.WithOperationContext(read, op)
	}
	readErr := m.requireDraining(read, m.currentBarrierID())
	cancelRead()
	if readErr != nil {
		return nil, nil, readErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !original.active || !original.bound || original.purpose == "job_claim" || m.record.Status.State != "draining" {
		return nil, nil, platform.NewError(platform.CodeLocked, true, "原执行已结束或未绑定")
	}
	work, done, err := m.newCleanupLocked(context.WithoutCancel(ctx), original.operation)
	if err == nil && original.purpose == "event_broker" {
		work.Value(snapshotAdmissionKey{}).(*snapshotAdmission).purpose = "event_broker"
	}
	return work, done, err
}
func (m *SnapshotManager) currentBarrierID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.record == nil {
		return ""
	}
	return m.record.Input.BarrierID
}
func (m *SnapshotManager) newCleanupLocked(ctx context.Context, op platform.OperationContext) (context.Context, func(), error) {
	until := time.Now().UTC().Add(30 * time.Second)
	if m.record.Input.Deadline.Before(until) {
		until = m.record.Input.Deadline
	}
	if current, ok := ctx.Deadline(); ok && current.Before(until) {
		until = current
	}
	if !until.After(time.Now()) {
		return nil, nil, platform.NewError(platform.CodeDeadlineExceeded, false, "原屏障清理截止已过")
	}
	work, cancel := context.WithDeadline(platform.WithPortalProof(ctx, ""), until)
	eventControl := op.CleanupScope != nil && op.CleanupScope.EventControl
	op.Deadline = until.Format(time.RFC3339Nano)
	op.CleanupScope = nil
	work = platform.WithOperationContext(work, op)
	lease := &snapshotAdmission{manager: m, epoch: op.DataEpoch, active: true, cleanup: true, deadline: until, purpose: "cleanup", bound: true, operation: op}
	if eventControl {
		lease.purpose = "event_broker"
	}
	m.cleanup++
	m.signalLocked()
	done := func() {
		cancel()
		m.mu.Lock()
		defer m.mu.Unlock()
		if lease.active {
			lease.active = false
			m.cleanup--
			m.signalLocked()
		}
	}
	work = context.WithValue(work, snapshotAdmissionKey{}, lease)
	return platform.WithCleanupIssuer(work, lease), done, nil
}

// AdmitCleanup只做真实屏障lease；HTTP已经核过固定action/resource和目标本域原对象归属。
func (m *SnapshotManager) AdmitCleanup(ctx context.Context, op platform.OperationContext, scope platform.CleanupScope) (context.Context, func(), error) {
	if err := m.requireDraining(platform.WithoutCleanupIssuer(ctx), scope.BarrierID); err != nil {
		return nil, nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.record.Status.State != "draining" || scope.TargetService != m.config.Owner || scope.DataEpoch != m.record.Epoch || scope.RootOperationID != op.RootOperationID || scope.SiteID != op.Actor.SiteID || scope.CostClass != op.CostClass {
		return nil, nil, platform.NewError(platform.CodeForbidden, false, "接收清理与本域屏障或原根不符")
	}
	deadline, err := platform.ParseTimestamp(scope.Deadline)
	if err != nil || deadline.After(m.record.Input.Deadline) {
		return nil, nil, platform.NewError(platform.CodeForbidden, false, "清理不得超过本域原屏障截止")
	}
	return m.newCleanupLocked(ctx, op)
}

// AdmitEventBatch仅为真实事件中转批次计数；它不能创建job/模型或替代原付费任务根。
func (m *SnapshotManager) AdmitEventBatch(ctx context.Context) (context.Context, func(), error) {
	view, err := m.config.Runtime(platform.WithoutCleanupIssuer(ctx))
	if err != nil {
		return nil, nil, err
	}
	if view.Phase == platform.PhaseRunning && !view.Maintenance {
		work, done, err := m.AdmitBusiness(ctx)
		if err != nil {
			return nil, nil, err
		}
		m.mu.Lock()
		work.Value(snapshotAdmissionKey{}).(*snapshotAdmission).purpose = "event_broker"
		m.mu.Unlock()
		return work, done, nil
	}
	if err = m.requireDraining(platform.WithoutCleanupIssuer(ctx), view.BarrierID); err != nil {
		return nil, nil, err
	}
	op, ok := platform.OperationContextFrom(ctx)
	if !ok || op.CallerService != m.config.Owner || op.DataEpoch != m.config.OpenedDataEpoch {
		return nil, nil, platform.NewError(platform.CodeForbidden, false, "事件批次缺固定本域服务控制scope")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.record.Status.State != "draining" {
		return nil, nil, platform.NewError(platform.CodeLocked, true, "事件清理批次已封口")
	}
	work, done, err := m.newCleanupLocked(ctx, op)
	if err != nil {
		return nil, nil, err
	}
	lease := work.Value(snapshotAdmissionKey{}).(*snapshotAdmission)
	lease.purpose = "event_broker"
	return work, done, nil
}
func (m *SnapshotManager) AllowEventDrain(ctx context.Context, view platform.RuntimeView) (bool, error) {
	if !view.Maintenance {
		return view.Phase == platform.PhaseRunning, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.record != nil && m.record.Status.State == "draining" && m.record.Epoch == view.DataEpoch && m.record.Input.BarrierID == view.BarrierID && m.record.Input.Deadline.After(time.Now()) && (view.Phase == platform.PhaseBackupBarrier || view.Phase == platform.PhaseRestoreStaging), nil
}

// BindOutboxEventScope从标准Store领取的原行绑定event控制root；不解析payload猜付费根。
func (m *SnapshotManager) BindOutboxEventScope(ctx context.Context, row platform.OutboxRow) (context.Context, func(), error) {
	parent, ok := ctx.Value(snapshotAdmissionKey{}).(*snapshotAdmission)
	if !ok || parent.manager != m {
		return nil, nil, platform.NewError(platform.CodeLocked, false, "事件缺本域批次scope")
	}
	op, ok := platform.OperationContextFrom(ctx)
	if !ok || row.Producer != m.config.Owner || row.DataEpoch != op.DataEpoch || !platform.ValidIdentifier(row.EventID) || platform.HashBytes(row.Payload) != row.PayloadHash {
		return nil, nil, integrity("原outbox事件绑定不符")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !parent.active {
		return nil, nil, platform.NewError(platform.CodeLocked, true, "事件批次已结束")
	}
	op.OperationID = row.EventID
	op.RootOperationID = row.EventID
	op.CleanupScope = nil
	if m.record != nil && m.record.Status.State != "released" {
		if m.record.Status.State != "draining" {
			return nil, nil, platform.NewError(platform.CodeLocked, true, "事件批次已封口")
		}
		work, done, err := m.newCleanupLocked(ctx, op)
		if err != nil {
			return nil, nil, err
		}
		work.Value(snapshotAdmissionKey{}).(*snapshotAdmission).purpose = "event_broker"
		return work, done, nil
	}
	work, cancel := context.WithCancel(platform.WithOperationContext(ctx, op))
	lease := &snapshotAdmission{manager: m, epoch: op.DataEpoch, active: true, purpose: "event_broker", bound: true, operation: op, cancel: cancel}
	m.leases[lease] = struct{}{}
	m.business++
	m.signalLocked()
	done := func() {
		cancel()
		m.mu.Lock()
		defer m.mu.Unlock()
		if lease.active {
			delete(m.leases, lease)
			lease.active = false
			m.business--
			m.signalLocked()
		}
	}
	return platform.WithCleanupIssuer(context.WithValue(work, snapshotAdmissionKey{}, lease), lease), done, nil
}
