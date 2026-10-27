package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/queue"
)

// LeaseRuntimeView 由装配层从运行权威读取。
type LeaseRuntimeView struct {
	Runtime platform.RuntimeView
}

type LeaseRuntimeProvider = func(context.Context) (LeaseRuntimeView, error)

type leaseOperation struct {
	store   *Store
	tx      *sql.Tx
	runtime LeaseRuntimeView
	now     time.Time
}

func beginLeaseOperation(ctx context.Context, op OperationContext, epoch platform.Identifier) (*leaseOperation, error) {
	if epoch != op.DataEpoch {
		return nil, epochMismatch()
	}
	store := currentStore()
	if store == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	store.mu.RLock()
	if store.closed || store.leaseRuntime == nil {
		store.mu.RUnlock()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "任务租约运行权威尚未装配")
	}
	view, err := store.leaseRuntime(ctx)
	if err != nil {
		store.mu.RUnlock()
		return nil, err
	}
	if view.Runtime.DataEpoch != epoch {
		store.mu.RUnlock()
		return nil, epochMismatch()
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		store.mu.RUnlock()
		return nil, mapQueueDBError(err)
	}
	return &leaseOperation{store: store, tx: tx, runtime: view, now: time.Now().UTC()}, nil
}

func (work *leaseOperation) close() {
	_ = work.tx.Rollback()
	work.store.mu.RUnlock()
}

func (work *leaseOperation) receipt(ctx context.Context, op OperationContext, kind string, request []byte) ([]byte, error) {
	var hash string
	var result []byte
	err := work.tx.QueryRowContext(ctx, `SELECT request_hash,result_json FROM lease_operations
		WHERE owner=? AND data_epoch=? AND operation_kind=? AND operation_id=?`, op.CallerService, op.DataEpoch, kind, op.OperationID).Scan(&hash, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapQueueDBError(err)
	}
	if hash != platform.HashBytes(request) {
		return nil, platform.NewError(platform.CodeConflict, false, "租约操作键已绑定不同输入")
	}
	return result, nil
}

func (work *leaseOperation) saveReceipt(ctx context.Context, op OperationContext, kind string, request []byte, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "租约回执编码失败")
	}
	_, err = work.tx.ExecContext(ctx, `INSERT INTO lease_operations(owner,data_epoch,operation_kind,operation_id,request_hash,result_json)
		VALUES(?,?,?,?,?,?)`, op.CallerService, op.DataEpoch, kind, op.OperationID, platform.HashBytes(request), encoded)
	return mapQueueDBError(err)
}

func (work *leaseOperation) enqueueState(jobID, owner, epoch, state string, revision int64) error {
	payload, err := json.Marshal(jobStateChangedPayload{JobID: jobID, State: state, JobRevision: revision, Owner: owner})
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "任务状态事件编码失败")
	}
	return queue.EnqueueEvent(work.tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: jobEventType,
		EventVersion: jobEventVersion, Producer: "jobs", SubjectID: jobID, Revision: revision, OccurredAt: work.now.Format(time.RFC3339Nano), DataEpoch: epoch, Payload: payload})
}

func leaseLost() error {
	return platform.NewError(platform.CodeStaleResult, false, "任务租约已失效")
}
