package internal

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/jobs/internal/gen"
)

var barrierProofOwners = [...]string{
	"identity", "site-config", "jobs", "content", "media", "model-gateway", "knowledge",
	"qa", "memory", "localization", "analytics", "traffic-guard", "data-protection", "public-site",
}

type BarrierObserveEvidence struct {
	BarrierID         platform.Identifier           `json:"barrierId"`
	DataEpoch         platform.Identifier           `json:"dataEpoch"`
	Phase             string                        `json:"phase"`
	RevisionWatermark gen.Revision                  `json:"revisionWatermark"`
	SelectedProofHash gen.Hash                      `json:"selectedProofHash"`
	Owners            []string                      `json:"owners"`
	BrokerBefore      gen.SnapshotBrokerObservation `json:"brokerBefore"`
	BrokerAfter       gen.SnapshotBrokerObservation `json:"brokerAfter"`
}

type CoordinateBarrierResult struct {
	Status        gen.BarrierStatus       `json:"status"`
	SelectedProof *BarrierObserveEvidence `json:"selectedProof,omitempty"`
}

type persistedBrokerObservation struct {
	Revision          int64 `json:"revision"`
	PendingDeliveries int64 `json:"pendingDeliveries"`
	ActiveLeases      int64 `json:"activeLeases"`
}

type persistedBarrierObserveEvidence struct {
	Version           int                        `json:"version"`
	BarrierID         string                     `json:"barrierId"`
	DataEpoch         string                     `json:"dataEpoch"`
	Phase             string                     `json:"phase"`
	RevisionWatermark int64                      `json:"revisionWatermark"`
	SelectedProofHash string                     `json:"selectedProofHash"`
	Owners            []string                   `json:"owners"`
	BrokerBefore      persistedBrokerObservation `json:"brokerBefore"`
	BrokerAfter       persistedBrokerObservation `json:"brokerAfter"`
}

// CoordinateBarrier允许全部已登记非jobs服务只读observe；prepare/drain/seal/resume-drain
// 仍只接受data-protection。本域不把自己的SnapshotManager冒充全域结果。
func CoordinateBarrier(ctx context.Context, op OperationContext, in CoordinateBarrierInput) (CoordinateBarrierResult, error) {
	input, step, err := coordinateBarrierRequest(op, in.Request)
	if err != nil {
		return CoordinateBarrierResult{}, err
	}
	if step != "prepare" && step != "drain" && step != "observe" && step != "seal" && step != "resume-drain" {
		return CoordinateBarrierResult{}, platform.NewError(platform.CodeValidation, false, "全域屏障步骤不合法")
	}
	if step == "observe" {
		if !isBarrierObserveCaller(op) {
			return CoordinateBarrierResult{}, platform.NewError(platform.CodeForbidden, false, "仅已登记非jobs服务身份可以只读观察屏障")
		}
		return observeBarrier(ctx, op, input)
	}
	if op.CallerService != "data-protection" {
		return CoordinateBarrierResult{}, platform.NewError(platform.CodeForbidden, false, "屏障变更步骤仅允许data-protection")
	}
	return CoordinateBarrierResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "全域两阶段协调器尚未装配")
}

func isBarrierObserveCaller(op OperationContext) bool {
	if op.Actor.Kind != "service" || string(op.Actor.SubjectID) != op.CallerService {
		return false
	}
	for _, owner := range barrierProofOwners {
		if owner == "jobs" {
			continue
		}
		if owner == op.CallerService {
			return true
		}
	}
	return false
}

// observeBarrier只读jobs自有barriers历史行中的协调器选择记录。该行未来由
// jobs CoordinateBarrier在data-protection通过版本化jobs API发起的变更步骤中持久写入；本函数不把本域safe封口hash当作基础proof。
func observeBarrier(ctx context.Context, op OperationContext, input lifecycle.SnapshotBarrierInput) (CoordinateBarrierResult, error) {
	store := currentStore()
	if store == nil {
		return CoordinateBarrierResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	db, err := store.SnapshotDatabase()
	if err != nil {
		return CoordinateBarrierResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CoordinateBarrierResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "开始屏障只读事务失败")
	}
	defer tx.Rollback()
	var state, raw, epoch, deadlineText string
	err = tx.QueryRowContext(ctx, `SELECT state,domain_status_json,data_epoch,deadline FROM barriers WHERE barrier_id=?`, input.BarrierID).Scan(&state, &raw, &epoch, &deadlineText)
	if errors.Is(err, sql.ErrNoRows) {
		return CoordinateBarrierResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "协调器基础proof尚未持久化")
	}
	if err != nil {
		return CoordinateBarrierResult{}, mapQueueDBError(err)
	}
	deadline, err := platform.ParseTimestamp(platform.Timestamp(deadlineText))
	if err != nil || !deadline.After(time.Now().UTC()) {
		return CoordinateBarrierResult{}, platform.NewError(platform.CodeLocked, true, "协调器基础proof已过期")
	}
	if state != "draining" || epoch != string(op.DataEpoch) {
		return CoordinateBarrierResult{}, platform.NewError(platform.CodeVersionMismatch, true, "协调器屏障状态或代次不一致")
	}
	var evidence persistedBarrierObserveEvidence
	decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&evidence) != nil || decoder.Decode(new(any)) != io.EOF {
		return CoordinateBarrierResult{}, platform.NewError(platform.CodeIntegrityError, false, "协调器基础proof记录不可读")
	}
	if err := validateBarrierObserveEvidence(evidence, input, op); err != nil {
		return CoordinateBarrierResult{}, err
	}
	broker, err := observeBrokerTx(ctx, tx, op.DataEpoch)
	if err != nil {
		return CoordinateBarrierResult{}, err
	}
	baseBroker := gen.SnapshotBrokerObservation{Revision: gen.Revision(evidence.BrokerBefore.Revision), PendingDeliveries: gen.Revision(evidence.BrokerBefore.PendingDeliveries), ActiveLeases: gen.Revision(evidence.BrokerBefore.ActiveLeases)}
	finalBroker := gen.SnapshotBrokerObservation{Revision: gen.Revision(evidence.BrokerAfter.Revision), PendingDeliveries: gen.Revision(evidence.BrokerAfter.PendingDeliveries), ActiveLeases: gen.Revision(evidence.BrokerAfter.ActiveLeases)}
	if baseBroker != finalBroker || broker != baseBroker {
		return CoordinateBarrierResult{}, platform.NewError(platform.CodeConflict, true, "broker基础观察已变化")
	}
	status := gen.BarrierStatus{BarrierId: gen.Identifier(evidence.BarrierID), Owner: "jobs", State: state, DataEpoch: gen.Identifier(evidence.DataEpoch), RevisionWatermark: gen.Revision(evidence.RevisionWatermark)}
	return CoordinateBarrierResult{
		Status: status,
		SelectedProof: &BarrierObserveEvidence{
			BarrierID: gen.Identifier(evidence.BarrierID), DataEpoch: gen.Identifier(evidence.DataEpoch), Phase: evidence.Phase,
			RevisionWatermark: gen.Revision(evidence.RevisionWatermark), SelectedProofHash: gen.Hash(evidence.SelectedProofHash),
			Owners:       append([]string(nil), evidence.Owners...),
			BrokerBefore: gen.SnapshotBrokerObservation{Revision: gen.Revision(evidence.BrokerBefore.Revision), PendingDeliveries: gen.Revision(evidence.BrokerBefore.PendingDeliveries), ActiveLeases: gen.Revision(evidence.BrokerBefore.ActiveLeases)},
			BrokerAfter:  gen.SnapshotBrokerObservation{Revision: gen.Revision(evidence.BrokerAfter.Revision), PendingDeliveries: gen.Revision(evidence.BrokerAfter.PendingDeliveries), ActiveLeases: gen.Revision(evidence.BrokerAfter.ActiveLeases)},
		},
	}, nil
}

// observeBrokerTx复用调用方已开启的SQLite事务；不得在此重新BeginTx。
func observeBrokerTx(ctx context.Context, tx *sql.Tx, dataEpoch platform.Identifier) (gen.SnapshotBrokerObservation, error) {
	if tx == nil || !platform.ValidIdentifier(dataEpoch) {
		return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeValidation, false, "broker观察事务或代次不合法")
	}
	observation, err := outbox.NewStore(nil).ObserveTx(ctx, tx)
	if err != nil {
		return gen.SnapshotBrokerObservation{}, err
	}
	if observation.Revision < 1 || observation.Pending < 0 || observation.Leased < 0 {
		return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "共享outbox水位未知")
	}

	var pendingDeliveries, activeDeliveryLeases int64
	rows, err := tx.QueryContext(ctx, `SELECT d.state,d.lease_expires_at,e.data_epoch
		FROM deliveries d LEFT JOIN events e ON e.event_id=d.event_id`)
	if err != nil {
		return gen.SnapshotBrokerObservation{}, mapQueueDBError(err)
	}
	for rows.Next() {
		var state, epoch string
		var leaseExpires sql.NullString
		if err := rows.Scan(&state, &leaseExpires, &epoch); err != nil {
			_ = rows.Close()
			return gen.SnapshotBrokerObservation{}, mapQueueDBError(err)
		}
		if epoch == "" {
			_ = rows.Close()
			return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "事件投递缺少父事件代次")
		}
		hasLease := leaseExpires.Valid && leaseExpires.String != ""
		switch state {
		case "queued":
			if hasLease {
				_ = rows.Close()
				return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "queued投递带有活动租约")
			}
		case "delivering":
			if !hasLease {
				_ = rows.Close()
				return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "delivering投递缺少活动租约")
			}
			if _, err := platform.ParseTimestamp(platform.Timestamp(leaseExpires.String)); err != nil {
				_ = rows.Close()
				return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "delivering投递租约截止损坏")
			}
		case "acknowledged":
			if hasLease {
				_ = rows.Close()
				return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "acknowledged投递仍有活动租约")
			}
		case "dead-letter":
			if hasLease {
				_ = rows.Close()
				return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "dead-letter投递仍有活动租约")
			}
		default:
			_ = rows.Close()
			return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "事件投递状态未知")
		}
		if state != "acknowledged" {
			if pendingDeliveries == math.MaxInt64 {
				_ = rows.Close()
				return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "未确认事件投递计数溢出")
			}
			pendingDeliveries++
		}
		if state == "delivering" {
			if activeDeliveryLeases == math.MaxInt64 {
				_ = rows.Close()
				return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "事件投递租约计数溢出")
			}
			activeDeliveryLeases++
		}
		if epoch != string(dataEpoch) && state != "acknowledged" {
			_ = rows.Close()
			return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeVersionMismatch, false, "旧代仍有未确认事件投递")
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return gen.SnapshotBrokerObservation{}, mapQueueDBError(err)
	}
	_ = rows.Close()
	var activeJobsWithoutLease int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs j LEFT JOIN leases l ON l.job_id=j.job_id
		WHERE j.data_epoch=? AND j.state IN ('leased','running','cancel-requested') AND l.job_id IS NULL`, dataEpoch).Scan(&activeJobsWithoutLease); err != nil {
		return gen.SnapshotBrokerObservation{}, mapQueueDBError(err)
	}
	if activeJobsWithoutLease != 0 {
		return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "当前epoch存在无租约的活动任务")
	}
	var currentOrphans int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM job_orphans o JOIN jobs j ON j.job_id=o.job_id WHERE j.data_epoch=?`, dataEpoch).Scan(&currentOrphans); err != nil {
		return gen.SnapshotBrokerObservation{}, mapQueueDBError(err)
	}
	if currentOrphans != 0 {
		return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeLocked, true, "当前epoch仍有待决任务孤儿")
	}

	var activeJobLeases int64
	rows, err = tx.QueryContext(ctx, `SELECT j.data_epoch,j.state,l.expires_at
		FROM leases l LEFT JOIN jobs j ON j.job_id=l.job_id`)
	if err != nil {
		return gen.SnapshotBrokerObservation{}, mapQueueDBError(err)
	}
	for rows.Next() {
		var epoch, state, expires string
		if err := rows.Scan(&epoch, &state, &expires); err != nil {
			_ = rows.Close()
			return gen.SnapshotBrokerObservation{}, mapQueueDBError(err)
		}
		if epoch == "" || epoch != string(dataEpoch) {
			_ = rows.Close()
			return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeVersionMismatch, false, "任务租约代次未知或过期")
		}
		switch state {
		case "leased", "running", "cancel-requested":
		default:
			_ = rows.Close()
			return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "任务租约状态未知")
		}
		if _, err := platform.ParseTimestamp(platform.Timestamp(expires)); err != nil {
			_ = rows.Close()
			return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "任务租约截止损坏")
		}
		if activeJobLeases == math.MaxInt64 {
			_ = rows.Close()
			return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "任务租约计数溢出")
		}
		activeJobLeases++
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return gen.SnapshotBrokerObservation{}, mapQueueDBError(err)
	}
	_ = rows.Close()

	if observation.Pending > math.MaxInt64-pendingDeliveries || activeDeliveryLeases > math.MaxInt64-activeJobLeases {
		return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "broker观察计数溢出")
	}
	activeLeases := activeDeliveryLeases + activeJobLeases
	if observation.Leased > math.MaxInt64-activeLeases {
		return gen.SnapshotBrokerObservation{}, platform.NewError(platform.CodeIntegrityError, false, "broker观察计数溢出")
	}
	return gen.SnapshotBrokerObservation{
		Revision:          gen.Revision(observation.Revision),
		PendingDeliveries: gen.Revision(observation.Pending + pendingDeliveries),
		ActiveLeases:      gen.Revision(observation.Leased + activeLeases),
	}, nil
}

func validateBarrierObserveEvidence(evidence persistedBarrierObserveEvidence, input lifecycle.SnapshotBarrierInput, op OperationContext) error {
	expectedPhase := "backup_barrier"
	if input.Mode == "restore" {
		expectedPhase = "restore_staging"
	}
	if evidence.Version != 1 || evidence.BarrierID != input.BarrierID || evidence.DataEpoch != string(op.DataEpoch) || evidence.Phase != expectedPhase || evidence.RevisionWatermark < 0 || !platform.ValidHash(platform.Hash(evidence.SelectedProofHash)) || len(evidence.Owners) != len(barrierProofOwners) {
		return platform.NewError(platform.CodeIntegrityError, false, "协调器基础proof字段不完整")
	}
	owners := make(map[string]bool, len(evidence.Owners))
	for _, owner := range evidence.Owners {
		if owners[owner] || !containsBarrierProofOwner(owner) {
			return platform.NewError(platform.CodeIntegrityError, false, "协调器基础proof owner集合不合法")
		}
		owners[owner] = true
	}
	if len(owners) != len(barrierProofOwners) || !owners[op.CallerService] {
		return platform.NewError(platform.CodeForbidden, false, "调用服务不在基础proof owner集合")
	}
	if evidence.BrokerBefore != evidence.BrokerAfter || evidence.BrokerBefore.Revision < 1 || evidence.BrokerBefore.PendingDeliveries != 0 || evidence.BrokerBefore.ActiveLeases != 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "协调器基础proof broker观察不稳定")
	}
	return nil
}

func containsBarrierProofOwner(owner string) bool {
	for _, expected := range barrierProofOwners {
		if owner == expected {
			return true
		}
	}
	return false
}

func coordinateBarrierRequest(op OperationContext, request BarrierRequest) (lifecycle.SnapshotBarrierInput, string, error) {
	return barrierInput(op, request)
}
