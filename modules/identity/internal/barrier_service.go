package identity

import (
	"context"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/identity/internal/gen"
)

// BarrierRequest、BarrierStatus 和快照请求结果沿用生成目录的唯一类型来源。
type BarrierRequest = gen.BarrierRequest
type BarrierStatus = gen.BarrierStatus
type ReleaseBarrierResult = gen.ReleaseBarrierResult

// ReleaseBarrierInput补上OpenAPI路径中的barrierId；正文只承载expectedEpoch。
type ReleaseBarrierInput struct {
	BarrierID     platform.Identifier `json:"barrierId"`
	ExpectedEpoch platform.Identifier `json:"expectedEpoch"`
}

// AcquireBarrier按公共屏障协议逐步推进本域状态。单域prepare/drain/observe
// 不宣称全域safe，seal仍交给公共Manager核验全域证明。
func AcquireBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error) {
	if err := requireProtectionService(op); err != nil {
		return BarrierStatus{}, err
	}
	if input.OperationId != op.OperationID || input.BarrierId == "" || input.Deadline.IsZero() || !input.Deadline.After(time.Now().UTC()) {
		return BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "屏障请求与可信操作上下文不一致")
	}
	manager, err := snapshotManagerFromContext(ctx)
	if err != nil {
		return BarrierStatus{}, err
	}
	mode, err := barrierMode(input.Mode)
	if err != nil {
		return BarrierStatus{}, err
	}
	step := "prepare"
	if input.Step != nil {
		step = string(*input.Step)
	}
	request := lifecycle.SnapshotBarrierInput{
		BarrierID:   string(input.BarrierId),
		OperationID: string(input.OperationId),
		Epoch:       string(op.DataEpoch),
		Mode:        mode,
		Deadline:    input.Deadline,
	}
	var status lifecycle.SnapshotBarrierStatus
	switch step {
	case "prepare":
		status, err = manager.EnterBarrier(ctx, request)
	case "drain":
		status, err = manager.DrainBarrier(ctx, request.BarrierID)
	case "observe":
		observation, observeErr := manager.ObserveBarrier(ctx, request.BarrierID)
		if observeErr != nil {
			return BarrierStatus{}, observeErr
		}
		observedStatus, _ := manager.ReadBarrierStatus()
		if observedStatus == nil {
			return BarrierStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域屏障状态尚未持久化")
		}
		statusCopy := *observedStatus
		statusCopy.BarrierID = request.BarrierID
		statusCopy.DataEpoch = request.Epoch
		statusCopy.RevisionWatermark = observation.RevisionWatermark
		return barrierStatus(statusCopy, &observation), nil
	case "seal":
		if input.SealProof == nil {
			return BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "封口缺少完整全域证明")
		}
		proof, proofErr := snapshotSealProof(*input.SealProof)
		if proofErr != nil {
			return BarrierStatus{}, proofErr
		}
		status, err = manager.SealBarrier(ctx, proof)
	case "resume-drain":
		if input.ExpectedSealHash == nil || !platform.ValidHash(*input.ExpectedSealHash) {
			return BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "重开清理缺少原封口摘要")
		}
		status, err = manager.ResumeDrain(ctx, request.BarrierID, string(*input.ExpectedSealHash))
	default:
		return BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "屏障步骤不受支持")
	}
	if err != nil {
		return BarrierStatus{}, err
	}
	return barrierStatus(status, nil), nil
}

// ReleaseBarrier只在site-config已退出维护且本域无在途工作时释放Manager屏障。
func ReleaseBarrier(ctx context.Context, op OperationContext, input ReleaseBarrierInput) (ReleaseBarrierResult, error) {
	if err := requireProtectionService(op); err != nil {
		return ReleaseBarrierResult{}, err
	}
	if input.BarrierID == "" || input.ExpectedEpoch != op.DataEpoch {
		return ReleaseBarrierResult{}, platform.NewError(platform.CodeVersionMismatch, false, "释放屏障代次不一致")
	}
	manager, err := snapshotManagerFromContext(ctx)
	if err != nil {
		return ReleaseBarrierResult{}, err
	}
	status, err := manager.ReleaseBarrier(ctx, string(input.BarrierID))
	if err != nil {
		return ReleaseBarrierResult{}, err
	}
	return ReleaseBarrierResult{Status: barrierStatusValue(status, nil)}, nil
}

func requireProtectionService(op OperationContext) error {
	if op.CallerService != "data-protection" {
		return platform.NewError(platform.CodeForbidden, false, "屏障控制只允许data-protection调用")
	}
	if op.Actor.Kind == "service" && op.Actor.SubjectID == "data-protection" {
		return nil
	}
	if validSuperadminDelegation(op) {
		return nil
	}
	return platform.NewError(platform.CodeForbidden, false, "屏障控制缺少有效超管委托")
}

func barrierMode(value interface{}) (string, error) {
	mode, ok := value.(string)
	if !ok || (mode != "backup" && mode != "restore") {
		return "", platform.NewError(platform.CodeValidation, false, "屏障模式不合法")
	}
	return mode, nil
}

func snapshotSealProof(input gen.SnapshotSealProof) (lifecycle.SnapshotSealProof, error) {
	proof := lifecycle.SnapshotSealProof{
		BarrierID:   string(input.BarrierId),
		DataEpoch:   string(input.DataEpoch),
		BrokerBefore: lifecycle.SnapshotBrokerObservation{
			ActiveLeases:      int64(input.BrokerBefore.ActiveLeases),
			PendingDeliveries: int64(input.BrokerBefore.PendingDeliveries),
			Revision:          int64(input.BrokerBefore.Revision),
		},
		BrokerAfter: lifecycle.SnapshotBrokerObservation{
			ActiveLeases:      int64(input.BrokerAfter.ActiveLeases),
			PendingDeliveries: int64(input.BrokerAfter.PendingDeliveries),
			Revision:          int64(input.BrokerAfter.Revision),
		},
		Observations: make([]lifecycle.SnapshotBarrierObservation, 0, len(input.Observations)),
	}
	if input.NonJobsProofHash != nil {
		proof.NonJobsProofHash = string(*input.NonJobsProofHash)
	}
	if input.NonJobsSeals != nil {
		proof.NonJobsSeals = make([]lifecycle.SnapshotSealReceipt, 0, len(*input.NonJobsSeals))
		for _, seal := range *input.NonJobsSeals {
			proof.NonJobsSeals = append(proof.NonJobsSeals, lifecycle.SnapshotSealReceipt{Owner: seal.Owner, SealProofHash: string(seal.SealProofHash)})
		}
	}
	for _, observation := range input.Observations {
		proof.Observations = append(proof.Observations, lifecycle.SnapshotBarrierObservation{
			Owner:            observation.Owner,
			ActivityRevision: int64(observation.ActivityRevision),
			Work: lifecycle.SnapshotWorkState{
				OutboxRevision: int64(observation.Work.OutboxRevision),
				WorkRevision:   int64(observation.Work.WorkRevision),
				PendingOutbox:  int64(observation.Work.PendingOutbox),
				PendingWork:    int64(observation.Work.PendingWork),
			},
			RevisionWatermark: int64(observation.RevisionWatermark),
			Business:          int64(observation.Business),
			Cleanup:           int64(observation.Cleanup),
			FileOps:           int64(observation.FileOps),
			DrainCallbacks:    int64(observation.DrainCallbacks),
		})
	}
	if proof.BarrierID == "" || proof.DataEpoch == "" {
		return lifecycle.SnapshotSealProof{}, platform.NewError(platform.CodeValidation, false, "封口证明缺少屏障或代次")
	}
	return proof, nil
}

func barrierStatus(status lifecycle.SnapshotBarrierStatus, observation *lifecycle.SnapshotBarrierObservation) BarrierStatus {
	return barrierStatusValue(status, observation)
}

func barrierStatusValue(status lifecycle.SnapshotBarrierStatus, observation *lifecycle.SnapshotBarrierObservation) gen.BarrierStatus {
	result := gen.BarrierStatus{
		BarrierId:         gen.Identifier(status.BarrierID),
		Owner:             status.Owner,
		State:             status.State,
		DataEpoch:         gen.Identifier(status.DataEpoch),
		RevisionWatermark: gen.Revision(status.RevisionWatermark),
	}
	if status.SealProofHash != "" {
		value := gen.Hash(status.SealProofHash)
		result.SealProofHash = &value
	}
	if observation != nil {
		result.Observation = &gen.SnapshotBarrierObservation{
			Owner:            observation.Owner,
			ActivityRevision: gen.Revision(observation.ActivityRevision),
			Work: gen.SnapshotWorkState{
				OutboxRevision: gen.Revision(observation.Work.OutboxRevision),
				WorkRevision:   gen.Revision(observation.Work.WorkRevision),
				PendingOutbox:  gen.Revision(observation.Work.PendingOutbox),
				PendingWork:    gen.Revision(observation.Work.PendingWork),
			},
			RevisionWatermark: gen.Revision(observation.RevisionWatermark),
			Business:          gen.Revision(observation.Business),
			Cleanup:           gen.Revision(observation.Cleanup),
			FileOps:           gen.Revision(observation.FileOps),
			DrainCallbacks:    gen.Revision(observation.DrainCallbacks),
		}
	}
	return result
}
