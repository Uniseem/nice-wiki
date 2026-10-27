package lifecycle

import (
	"context"
	"time"

	"nice-wiki/libs/platform"
)

// SimpleAdmission 是各服务共用的业务准入：只确认运行代次一致。
func SimpleAdmission(runtime func(context.Context) (platform.RuntimeView, error), openedEpoch platform.Identifier) platform.WorkAdmission {
	return func(ctx context.Context) (context.Context, func(), error) {
		view, err := runtime(ctx)
		if err != nil {
			return nil, nil, err
		}
		if openedEpoch != "" && view.DataEpoch != openedEpoch {
			return nil, nil, platform.NewError(platform.CodeVersionMismatch, false, "进程数据代次已变化，等待重启")
		}
		return ctx, func() {}, nil
	}
}

// SimpleWriteWindow 是各服务共用的写窗口：读到运行权威就放行，没有截止时间。
func SimpleWriteWindow(runtime func(context.Context) (platform.RuntimeView, error)) func(context.Context) (time.Time, error) {
	return func(ctx context.Context) (time.Time, error) {
		if _, err := runtime(ctx); err != nil {
			return time.Time{}, err
		}
		return time.Time{}, nil
	}
}
