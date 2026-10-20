package lifecycle

import (
	"context"

	"nice-wiki/libs/platform"
)

// SimpleAdmission 是尚未接入快照屏障的服务使用的业务准入：只确认运行代次一致、
// 处于运行阶段且不在维护中。屏障/快照接入后换成 SnapshotManager.AdmitBusiness。
func SimpleAdmission(runtime func(context.Context) (platform.RuntimeView, error), openedEpoch platform.Identifier) platform.WorkAdmission {
	return func(ctx context.Context) (context.Context, func(), error) {
		view, err := runtime(ctx)
		if err != nil {
			return nil, nil, err
		}
		if openedEpoch != "" && view.DataEpoch != openedEpoch {
			return nil, nil, platform.NewError(platform.CodeVersionMismatch, false, "进程数据代次已变化，等待重启")
		}
		if view.Phase != platform.PhaseRunning || view.Maintenance {
			return nil, nil, platform.NewError(platform.CodeLocked, true, "维护中不接受新业务")
		}
		return ctx, func() {}, nil
	}
}
