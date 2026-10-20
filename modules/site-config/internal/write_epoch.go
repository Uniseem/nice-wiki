package siteconfig

import (
	"context"
	"reflect"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/site-config/internal/gen"
)

type snapshotBarrierStatusReader interface {
	ReadBarrierStatus() (*lifecycle.SnapshotBarrierStatus, time.Time)
}

func cleanupWindowActive(ctx context.Context) bool {
	issuer := platform.CleanupIssuerFrom(ctx)
	if issuer == nil {
		return false
	}
	value := reflect.ValueOf(issuer)
	return value.Kind() != reflect.Pointer || !value.IsNil()
}

// CheckWriteEpochInput 是业务写入前epoch与屏障检查的输入。
type CheckWriteEpochInput struct {
	DataEpoch gen.Identifier `json:"dataEpoch"`
}

// CheckWriteEpochResult 是epoch与屏障检查结果。
type CheckWriteEpochResult struct {
	Allowed bool `json:"allowed"`
}

// CheckWriteEpoch 是业务域提交前使用的只读控制门；它同时读取本域运行权威
// 与同一SnapshotManager的内存屏障状态，不打开业务库或登记在途事务。
func CheckWriteEpoch(ctx context.Context, op platform.OperationContext, input CheckWriteEpochInput) (CheckWriteEpochResult, error) {
	if err := checkOperationDeadline(ctx, op); err != nil {
		return CheckWriteEpochResult{}, err
	}
	store := activeRuntimeStore()
	if store == nil {
		return CheckWriteEpochResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库不可用")
	}
	state, err := store.readState(ctx)
	if err != nil {
		return CheckWriteEpochResult{}, err
	}
	cleanup := cleanupWindowActive(ctx)
	if state.Phase != gen.Running && !cleanup {
		return CheckWriteEpochResult{}, platform.NewError(platform.CodeLocked, false, "运行状态处于维护阶段")
	}
	if input.DataEpoch == "" || input.DataEpoch != state.DataEpoch || op.DataEpoch != input.DataEpoch {
		return CheckWriteEpochResult{}, platform.NewError(platform.CodeVersionMismatch, false, "写入dataEpoch已变化")
	}
	active := currentActiveStore()
	if active == nil {
		return CheckWriteEpochResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	barrier, _, configured := active.readSnapshotBarrierStatus()
	if !configured {
		return CheckWriteEpochResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域屏障控制未装配")
	}
	if barrier != nil {
		if barrier.DataEpoch != "" && barrier.DataEpoch != string(op.DataEpoch) {
			return CheckWriteEpochResult{}, platform.NewError(platform.CodeVersionMismatch, false, "本域屏障dataEpoch已变化")
		}
		if barrier.State != "released" && !cleanup {
			return CheckWriteEpochResult{}, platform.NewError(platform.CodeLocked, true, "本域屏障尚未解除")
		}
	}
	return CheckWriteEpochResult{Allowed: true}, nil
}
