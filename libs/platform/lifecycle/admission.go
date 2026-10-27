package lifecycle

import (
	"context"
	"time"

	"nice-wiki/libs/platform"
)

// SimpleAdmission 是各服务共用的业务准入：只确认运行代次一致、处于运行阶段且不在维护中。
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

// SimpleWriteWindow 是各服务共用的写窗口：不在维护中就没有截止时间，维护中拒绝写入。
func SimpleWriteWindow(runtime func(context.Context) (platform.RuntimeView, error)) func(context.Context) (time.Time, error) {
	return func(ctx context.Context) (time.Time, error) {
		view, err := runtime(ctx)
		if err != nil {
			return time.Time{}, err
		}
		if view.Maintenance {
			return time.Time{}, platform.NewError(platform.CodeLocked, false, "本域维护写窗口关闭")
		}
		return time.Time{}, nil
	}
}
