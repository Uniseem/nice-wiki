package siteconfig

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/site-config/internal/gen"
)

// SnapshotRuntime 是BuildApplication注入的本域快照端口；不承载跨域协调算法。
type SnapshotRuntime struct {
	API            lifecycle.SnapshotAPI
	RestoreArchive func(context.Context, string, string) (io.ReadCloser, error)
}

type snapshotRuntimeKey struct{}

// WithSnapshotRuntime 把同一SnapshotManager与受控归档读取端口绑定到本次已鉴权调用。
func WithSnapshotRuntime(ctx context.Context, runtime SnapshotRuntime) context.Context {
	return context.WithValue(ctx, snapshotRuntimeKey{}, runtime)
}

func snapshotRuntimeFrom(ctx context.Context) (SnapshotRuntime, error) {
	runtime, ok := ctx.Value(snapshotRuntimeKey{}).(SnapshotRuntime)
	if !ok || runtime.API == nil {
		return SnapshotRuntime{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域快照依赖未装配")
	}
	return runtime, nil
}

func snapshotJSONConvert(input, output any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "快照协议转换失败")
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "快照协议字段不一致")
	}
	return nil
}

func requireSnapshotController(op platform.OperationContext) error {
	if op.CallerService != "data-protection" {
		return platform.NewError(platform.CodeForbidden, false, "快照控制仅允许data-protection服务")
	}
	return nil
}

func toLifecycleBarrierInput(op platform.OperationContext, input gen.BarrierRequest) (lifecycle.SnapshotBarrierInput, error) {
	if string(input.OperationId) != string(op.OperationID) {
		return lifecycle.SnapshotBarrierInput{}, platform.NewError(platform.CodeConflict, false, "屏障operationId与可信操作不一致")
	}
	var converted lifecycle.SnapshotBarrierInput
	if err := snapshotJSONConvert(input, &converted); err != nil {
		return lifecycle.SnapshotBarrierInput{}, err
	}
	converted.Epoch = string(op.DataEpoch)
	return converted, nil
}

func barrierStatusFromLifecycle(status lifecycle.SnapshotBarrierStatus) (gen.BarrierStatus, error) {
	var result gen.BarrierStatus
	if err := snapshotJSONConvert(status, &result); err != nil {
		return gen.BarrierStatus{}, err
	}
	return result, nil
}

func observationStatus(barrierID, epoch string, observation lifecycle.SnapshotBarrierObservation) (gen.BarrierStatus, error) {
	value := struct {
		BarrierID         string                               `json:"barrierId"`
		Owner             string                               `json:"owner"`
		State             string                               `json:"state"`
		DataEpoch         string                               `json:"dataEpoch"`
		RevisionWatermark int64                                `json:"revisionWatermark"`
		Observation       lifecycle.SnapshotBarrierObservation `json:"observation"`
	}{
		BarrierID: barrierID, Owner: "site-config", State: "draining", DataEpoch: epoch,
		RevisionWatermark: observation.RevisionWatermark, Observation: observation,
	}
	var result gen.BarrierStatus
	if err := snapshotJSONConvert(value, &result); err != nil {
		return gen.BarrierStatus{}, err
	}
	return result, nil
}

// AcquireBarrier 承接prepare/drain/observe/seal/resume-drain五步公共屏障端口。
func AcquireBarrier(ctx context.Context, op platform.OperationContext, input gen.BarrierRequest) (gen.BarrierStatus, error) {
	if err := requireSnapshotController(op); err != nil {
		return gen.BarrierStatus{}, err
	}
	runtime, err := snapshotRuntimeFrom(ctx)
	if err != nil {
		return gen.BarrierStatus{}, err
	}
	barrier, err := toLifecycleBarrierInput(op, input)
	if err != nil {
		return gen.BarrierStatus{}, err
	}
	step := "prepare"
	if input.Step != nil {
		step = string(*input.Step)
	}
	switch step {
	case "prepare":
		status, callErr := runtime.API.PrepareBarrier(ctx, barrier)
		if callErr != nil {
			return gen.BarrierStatus{}, callErr
		}
		return barrierStatusFromLifecycle(status)
	case "drain":
		status, callErr := runtime.API.DrainBarrier(ctx, barrier.BarrierID)
		if callErr != nil {
			return gen.BarrierStatus{}, callErr
		}
		return barrierStatusFromLifecycle(status)
	case "observe":
		observation, callErr := runtime.API.ObserveBarrier(ctx, barrier.BarrierID)
		if callErr != nil {
			return gen.BarrierStatus{}, callErr
		}
		return observationStatus(barrier.BarrierID, barrier.Epoch, observation)
	case "seal":
		if input.SealProof == nil {
			return gen.BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "封口缺少完整proof")
		}
		var proof lifecycle.SnapshotSealProof
		if err := snapshotJSONConvert(*input.SealProof, &proof); err != nil {
			return gen.BarrierStatus{}, err
		}
		status, callErr := runtime.API.SealBarrier(ctx, proof)
		if callErr != nil {
			return gen.BarrierStatus{}, callErr
		}
		return barrierStatusFromLifecycle(status)
	case "resume-drain":
		if input.ExpectedSealHash == nil {
			return gen.BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "resume-drain缺少原封口摘要")
		}
		status, callErr := runtime.API.ResumeDrain(ctx, barrier.BarrierID, string(*input.ExpectedSealHash))
		if callErr != nil {
			return gen.BarrierStatus{}, callErr
		}
		return barrierStatusFromLifecycle(status)
	default:
		return gen.BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "未知屏障步骤")
	}
}

type ReleaseBarrierInput struct {
	BarrierId     gen.Identifier `json:"barrierId"`
	ExpectedEpoch gen.Identifier `json:"expectedEpoch"`
}

// ReleaseBarrier 只在公共Manager确认全局running后释放本域旧清理门。
func ReleaseBarrier(ctx context.Context, op platform.OperationContext, input ReleaseBarrierInput) (gen.ReleaseBarrierResult, error) {
	if err := requireSnapshotController(op); err != nil {
		return gen.ReleaseBarrierResult{}, err
	}
	if input.ExpectedEpoch != gen.Identifier(op.DataEpoch) {
		return gen.ReleaseBarrierResult{}, platform.NewError(platform.CodeVersionMismatch, false, "释放屏障代次不一致")
	}
	if !platform.ValidIdentifier(platform.Identifier(input.BarrierId)) {
		return gen.ReleaseBarrierResult{}, platform.NewError(platform.CodeValidation, false, "释放屏障ID不合法")
	}
	runtime, err := snapshotRuntimeFrom(ctx)
	if err != nil {
		return gen.ReleaseBarrierResult{}, err
	}
	barrierReader, ok := runtime.API.(interface {
		ReadBarrierStatus() (*lifecycle.SnapshotBarrierStatus, time.Time)
	})
	if !ok {
		return gen.ReleaseBarrierResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域屏障状态读取器未装配")
	}
	current, _ := barrierReader.ReadBarrierStatus()
	if current == nil || current.BarrierID != string(input.BarrierId) {
		return gen.ReleaseBarrierResult{}, platform.NewError(platform.CodeNotFound, false, "本域屏障不存在")
	}
	if current.Owner != "site-config" || (current.DataEpoch != "" && current.DataEpoch != string(op.DataEpoch) && current.State != "released") {
		return gen.ReleaseBarrierResult{}, platform.NewError(platform.CodeVersionMismatch, false, "本域屏障绑定已变化")
	}
	status, err := runtime.API.ReleaseBarrier(ctx, string(input.BarrierId))
	if err != nil {
		return gen.ReleaseBarrierResult{}, err
	}
	converted, err := barrierStatusFromLifecycle(status)
	if err != nil {
		return gen.ReleaseBarrierResult{}, err
	}
	return gen.ReleaseBarrierResult{Status: converted}, nil
}
