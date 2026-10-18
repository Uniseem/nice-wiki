package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"strconv"
	"time"
)

type ExecutionRecord struct {
	JobID, JobType, ObjectID, WorkerID string
	InputRef                           platform.ObjectRef
	Input                              []byte
	Operation                          OperationContext
	Fence                              int64
	Attempt                            int
	LeaseUntil                         time.Time
	CancelRequested                    bool
	State                              string
	ResultJSON                         []byte
	ResultRef                          *platform.ObjectRef
	PaidRefs                           []string
}

// ExecutionLeaseID 只由jobs真实job/fence组成，不本地制造续租权限。
func ExecutionLeaseID(jobID string, fence int64) string {
	return jobID + "." + strconv.FormatInt(fence, 10)
}

// ApprovedJobInput 只从本域原批准派发读取完整payload/引用/root，不以jobId猜授权。
func ApprovedJobInput(ctx context.Context, tx *sql.Tx, jobType, objectID string) (ExecutionRecord, string, string, string, bool, error) {
	var record ExecutionRecord
	var ref, operation []byte
	var job sql.NullString
	var dispatch, jobsOperation, site, epoch string
	var cancelled int
	var expectedVersion int64
	var query string
	switch jobType {
	case "knowledge.build":
		query = `SELECT d.input_json,d.input_ref_json,d.operation_json,d.job_id,d.jobs_operation_id,d.dispatch_state,d.cancel_requested,b.site_id,b.data_epoch,b.generation FROM build_dispatches d JOIN builds b ON b.build_id=d.build_id WHERE d.build_id=?`
	case "knowledge.extract":
		query = `SELECT d.input_json,d.input_ref_json,d.operation_json,d.job_id,d.jobs_operation_id,d.dispatch_state,d.cancel_requested,e.site_id,e.data_epoch,1 FROM extraction_dispatches d JOIN extractions e ON e.extraction_id=d.extraction_id WHERE d.extraction_id=?`
	default:
		return record, "", "", "", false, platform.NewError(platform.CodeForbidden, false, "任务不属于本域登记类型")
	}
	err := tx.QueryRowContext(ctx, query, objectID).Scan(&record.Input, &ref, &operation, &job, &jobsOperation, &dispatch, &cancelled, &site, &epoch, &expectedVersion)
	if err != nil {
		return record, "", "", "", false, platform.NormalizeIOError(err, platform.CodeNotFound, false, "原批准任务输入不存在")
	}
	if json.Unmarshal(ref, &record.InputRef) != nil || json.Unmarshal(operation, &record.Operation) != nil || record.InputRef.Owner != "knowledge" || record.InputRef.ObjectID != objectID || record.InputRef.SHA256 != platform.HashBytes(record.Input) || record.InputRef.ByteSize != int64(len(record.Input)) || record.InputRef.MediaType != "application/json" || record.InputRef.Version != expectedVersion || record.Operation.Actor.SiteID != site || record.Operation.DataEpoch != epoch || record.Operation.CostClass != platform.CostClassBackground {
		return record, "", "", "", false, platform.NewError(platform.CodeIntegrityError, false, "原批准任务来源或完整输入引用损坏")
	}
	record.JobID = job.String
	record.JobType = jobType
	record.ObjectID = objectID
	return record, dispatch, jobsOperation, site, cancelled != 0, nil
}

// AdoptClaimedJob 认领后同事务接收真实job引用与fence镜像，Start/Heartbeat仍由平台执行。
func ResolveClaimedJobOperation(ctx context.Context, job platform.ClaimedJob) (OperationContext, error) {
	runtime, err := RuntimeFromContext(ctx)
	if err != nil {
		return OperationContext{}, err
	}
	read, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return OperationContext{}, storageError(err)
	}
	original, state, jobsOperation, site, _, err := ApprovedJobInput(ctx, read, job.JobType, job.InputRef.ObjectID)
	_ = read.Rollback()
	if err != nil {
		return OperationContext{}, err
	}
	if original.InputRef != job.InputRef || original.Operation.RootOperationID != job.RootOperationID || original.Operation.DataEpoch != job.DataEpoch || (original.JobID != "" && original.JobID != job.JobID) || (jobsOperation != "" && jobsOperation != job.OperationID) || original.Operation.OperationID != job.OperationID || (state != "dispatching" && state != "dispatched" && state != "completed") {
		return OperationContext{}, platform.NewError(platform.CodeIntegrityError, false, "领取任务与原批准输入/root/操作不一致")
	}
	if _, err = platform.ParseTimestamp(original.Operation.Deadline); err != nil {
		return OperationContext{}, platform.NewError(platform.CodeIntegrityError, false, "原批准任务截止损坏")
	}
	op := original.Operation
	op.CallerService = "knowledge"
	op.OperationID = job.OperationID
	op.RequestID = platform.NewIdentifier()
	op.Actor = platform.AuthContext{SubjectID: "knowledge", Kind: "service", SiteID: site, ExpiresAt: op.Deadline, Grants: []string{}}
	return op, nil
}

func AdoptClaimedJob(ctx context.Context, job platform.ClaimedJob) (OperationContext, error) {
	runtime, err := RuntimeFromContext(ctx)
	if err != nil {
		return OperationContext{}, err
	}
	read, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return OperationContext{}, storageError(err)
	}
	original, state, jobsOperation, site, cancelled, err := ApprovedJobInput(ctx, read, job.JobType, job.InputRef.ObjectID)
	_ = read.Rollback()
	if err != nil {
		return OperationContext{}, err
	}
	if original.InputRef != job.InputRef || original.Operation.RootOperationID != job.RootOperationID || original.Operation.DataEpoch != job.DataEpoch || (original.JobID != "" && original.JobID != job.JobID) || (jobsOperation != "" && jobsOperation != job.OperationID) || original.Operation.OperationID != job.OperationID || (state != "dispatching" && state != "dispatched" && state != "completed") {
		return OperationContext{}, platform.NewError(platform.CodeIntegrityError, false, "领取任务与原批准输入/root/操作不一致")
	}
	if _, err = platform.ParseTimestamp(original.Operation.Deadline); err != nil {
		return OperationContext{}, platform.NewError(platform.CodeIntegrityError, false, "原批准任务截止损坏")
	}
	op := original.Operation
	op.CallerService = "knowledge"
	op.OperationID = job.OperationID
	op.RequestID = platform.NewIdentifier()
	op.Actor = platform.AuthContext{SubjectID: "knowledge", Kind: "service", SiteID: site, ExpiresAt: op.Deadline, Grants: []string{}}
	control := op
	controlDeadline := time.Now().UTC().Add(30 * time.Second)
	if value, ok := ctx.Deadline(); ok && value.Before(controlDeadline) {
		controlDeadline = value
	}
	control.Deadline = controlDeadline.Format(time.RFC3339Nano)
	control.Actor.ExpiresAt = control.Deadline
	// 只保存真实claim控制事实；执行仍返回原批准截止，过期任务绝不续获付费权限。
	work, tx, cancel, err := BeginIndexWrite(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), control), control)
	if err != nil {
		return OperationContext{}, err
	}
	defer cancel()
	defer tx.Rollback()
	current, currentState, currentOperation, _, currentCancel, err := ApprovedJobInput(work, tx, job.JobType, job.InputRef.ObjectID)
	if err != nil {
		return OperationContext{}, err
	}
	if current.InputRef != job.InputRef || current.Operation.RootOperationID != job.RootOperationID || (current.JobID != "" && current.JobID != job.JobID) || (currentOperation != "" && currentOperation != job.OperationID) || currentCancel != cancelled || (currentState != "dispatching" && currentState != "dispatched" && currentState != "completed") {
		return OperationContext{}, platform.NewError(platform.CodeStaleResult, false, "任务接受来源已变化")
	}
	var oldFence int64
	var oldWorker string
	var oldAttempt int
	err = tx.QueryRowContext(work, `SELECT fence,worker_id,attempt FROM execution_records WHERE job_id=?`, job.JobID).Scan(&oldFence, &oldWorker, &oldAttempt)
	if err != nil && err != sql.ErrNoRows {
		return OperationContext{}, storageError(err)
	}
	if err == nil && (job.FencingToken < oldFence || (job.FencingToken == oldFence && (job.WorkerID != oldWorker || job.Attempt != oldAttempt)) || job.Attempt < oldAttempt) {
		return OperationContext{}, platform.NewError(platform.CodeStaleResult, false, "原任务fence或worker已失效")
	}
	if (err == sql.ErrNoRows || job.FencingToken > oldFence) && !currentCancel {
		deadline, _ := platform.ParseTimestamp(current.Operation.Deadline)
		if deadline.After(time.Now()) {
			table, key := "builds", "build_id"
			if job.JobType == "knowledge.extract" {
				table = "extractions"
				key = "extraction_id"
			}
			// 只有jobs真实新claim能恢复同原批准的失败阶段；续约回调不能复活失败或取消对象。
			if _, err = tx.ExecContext(work, "UPDATE "+table+" SET state='queued',failure_code='' WHERE "+key+"=? AND site_id=? AND data_epoch=? AND state='failed' AND cancelled=0", job.InputRef.ObjectID, site, job.DataEpoch); err != nil {
				return OperationContext{}, storageError(err)
			}
		}
	}
	refRaw, _ := platform.CanonicalJSON(job.InputRef)
	operationRaw, _ := platform.CanonicalJSON(op)
	adopted, err := tx.ExecContext(work, `INSERT INTO execution_records(job_id,site_id,data_epoch,job_type,object_id,input_ref_json,input_json,operation_json,worker_id,fence,attempt,lease_until,cancel_requested,state) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'running') ON CONFLICT(job_id) DO UPDATE SET worker_id=excluded.worker_id,fence=excluded.fence,attempt=excluded.attempt,lease_until=excluded.lease_until,cancel_requested=excluded.cancel_requested WHERE execution_records.data_epoch=excluded.data_epoch AND execution_records.object_id=excluded.object_id`, job.JobID, site, job.DataEpoch, job.JobType, job.InputRef.ObjectID, refRaw, current.Input, operationRaw, job.WorkerID, job.FencingToken, job.Attempt, job.ExpiresAt, cancelled)
	if err != nil {
		return OperationContext{}, storageError(err)
	}
	adoptedCount, err := adopted.RowsAffected()
	if err != nil {
		return OperationContext{}, storageError(err)
	}
	if adoptedCount != 1 {
		return OperationContext{}, platform.NewError(platform.CodeStaleResult, false, "原任务拥有者作用域CAS已变化")
	}
	if err = SaveExecutionMirror(work, tx, job, job.ExpiresAt, cancelled); err != nil {
		return OperationContext{}, err
	}
	if err = tx.Commit(); err != nil {
		return OperationContext{}, storageError(err)
	}
	return op, nil
}

// SaveExecutionMirror 私有镜像只来自真实claim/续约回执，原job/root/op核验由外层完成。
func SaveExecutionMirror(ctx context.Context, tx *sql.Tx, job platform.ClaimedJob, expires string, cancelled bool) error {
	var dispatch, object, key string
	switch job.JobType {
	case "knowledge.build":
		dispatch = "build_dispatches"
		object = "builds"
		key = "build_id"
	case "knowledge.extract":
		dispatch = "extraction_dispatches"
		object = "extractions"
		key = "extraction_id"
	default:
		return platform.NewError(platform.CodeForbidden, false, "任务类型不属于本域")
	}
	changed, err := tx.ExecContext(ctx, "UPDATE "+dispatch+" SET job_id=?,jobs_operation_id=?,dispatch_state=CASE WHEN dispatch_state='completed' THEN dispatch_state ELSE 'dispatched' END,updated_at=? WHERE "+key+"=? AND (job_id IS NULL OR job_id=?)", job.JobID, job.OperationID, platform.Now(), job.InputRef.ObjectID, job.JobID)
	if err != nil {
		return storageError(err)
	}
	count, err := changed.RowsAffected()
	if err != nil {
		return storageError(err)
	}
	if count != 1 {
		return platform.NewError(platform.CodeStaleResult, false, "任务派发归属已变化")
	}
	leaseID, until := ExecutionLeaseID(job.JobID, job.FencingToken), expires
	if cancelled {
		leaseID = ""
		until = ""
	}
	if _, err = tx.ExecContext(ctx, "UPDATE "+object+" SET lease_id=?,lease_until=? WHERE "+key+"=? AND cancelled=0 AND state NOT IN ('active','ready','unsupported','failed','cancelled','superseded')", leaseID, until, job.InputRef.ObjectID); err != nil {
		return storageError(err)
	}
	return nil
}

func LoadExecutionRecord(ctx context.Context, tx *sql.Tx, jobID string) (ExecutionRecord, error) {
	var record ExecutionRecord
	var ref, operation, resultRef, paid []byte
	var until string
	var cancelled int
	err := tx.QueryRowContext(ctx, `SELECT job_id,job_type,object_id,input_ref_json,input_json,operation_json,worker_id,fence,attempt,lease_until,cancel_requested,state,result_json,result_ref_json,paid_refs_json FROM execution_records WHERE job_id=?`, jobID).Scan(&record.JobID, &record.JobType, &record.ObjectID, &ref, &record.Input, &operation, &record.WorkerID, &record.Fence, &record.Attempt, &until, &cancelled, &record.State, &record.ResultJSON, &resultRef, &paid)
	if err != nil {
		return record, platform.NormalizeIOError(err, platform.CodeNotFound, false, "原任务拥有者记录不存在")
	}
	if json.Unmarshal(ref, &record.InputRef) != nil || json.Unmarshal(operation, &record.Operation) != nil {
		return record, platform.NewError(platform.CodeIntegrityError, false, "任务拥有者原输入或身份损坏")
	}
	if record.InputRef.Owner != "knowledge" || record.InputRef.ObjectID != record.ObjectID || record.InputRef.MediaType != "application/json" || record.InputRef.Version < 1 || record.InputRef.SHA256 != platform.HashBytes(record.Input) || record.InputRef.ByteSize != int64(len(record.Input)) || record.Operation.CostClass != platform.CostClassBackground {
		return record, platform.NewError(platform.CodeIntegrityError, false, "任务拥有者完整输入引用损坏")
	}
	record.LeaseUntil, err = platform.ParseTimestamp(until)
	if err != nil {
		return record, platform.NewError(platform.CodeIntegrityError, false, "任务拥有者租约时间损坏")
	}
	record.CancelRequested = cancelled != 0
	record.PaidRefs = []string{}
	if len(resultRef) > 0 {
		var value platform.ObjectRef
		if json.Unmarshal(resultRef, &value) != nil {
			return record, platform.NewError(platform.CodeIntegrityError, false, "原任务结果引用损坏")
		}
		record.ResultRef = &value
	}
	if len(paid) > 0 && json.Unmarshal(paid, &record.PaidRefs) != nil {
		return record, platform.NewError(platform.CodeIntegrityError, false, "原任务付费引用损坏")
	}
	if record.ResultRef != nil && (len(record.ResultJSON) > 16<<20 || record.ResultRef.Owner != "knowledge" || record.ResultRef.ObjectID != record.ObjectID+".result" || record.ResultRef.Version != record.InputRef.Version || record.ResultRef.MediaType != "application/json" || record.ResultRef.SHA256 != platform.HashBytes(record.ResultJSON) || record.ResultRef.ByteSize != int64(len(record.ResultJSON))) {
		return record, platform.NewError(platform.CodeIntegrityError, false, "原任务结果hash或完整长度不一致")
	}
	return record, nil
}

// ReadExecutionRecord 执行器只接受当前原job/worker/fence/attempt和完整inputRef，不重新解析HTTP授权。
func ReadExecutionRecord(ctx context.Context, op OperationContext, input platform.JobExecutionInput) (ExecutionRecord, error) {
	runtime, err := RuntimeFromContext(ctx)
	if err != nil {
		return ExecutionRecord{}, err
	}
	tx, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ExecutionRecord{}, storageError(err)
	}
	defer tx.Rollback()
	record, err := LoadExecutionRecord(ctx, tx, input.JobID)
	if err != nil {
		return record, err
	}
	if record.InputRef != input.InputRef || record.JobType != input.JobType || record.WorkerID != input.WorkerID || record.Fence != input.FencingToken || record.Attempt != input.Attempt || record.Operation.DataEpoch != input.DataEpoch || op.DataEpoch != input.DataEpoch || record.Operation.RootOperationID != op.RootOperationID || record.Operation.OperationID != op.OperationID || record.Operation.Actor.SiteID != op.Actor.SiteID {
		return record, platform.NewError(platform.CodeStaleResult, false, "执行器与原任务拥有者不一致")
	}
	if record.State != "completed" && (record.CancelRequested || !record.LeaseUntil.After(time.Now())) {
		return record, platform.NewError(platform.CodeCancelled, false, "原任务已取消或镜像租约失效")
	}
	return record, nil
}

// MirrorExecutionLease 仅消费平台已核权威回执，不CheckLease递归，不造本域心跳。
func MirrorExecutionLease(ctx context.Context, op OperationContext, job platform.ClaimedJob, info platform.HeartbeatInfo) error {
	// factory只读核原输入；首次真实续租后，已绑定私有执行scope内才可接管与持久镜像。
	job.ExpiresAt = info.ExpiresAt
	_, err := AdoptClaimedJob(ctx, job)
	return err
}

// ReadPaidResultRefs 返回原任务及已关联提取所有已知真实invocation引用，未知无ID仍由阶段状态阻止重派。
func ReadPaidResultRefs(ctx context.Context, tx *sql.Tx, op OperationContext, objectID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT invocation_id FROM model_steps WHERE site_id=? AND data_epoch=? AND invocation_id IS NOT NULL AND invocation_id<>'' AND (owner_id=? OR owner_id IN (SELECT extraction_id FROM extraction_origins WHERE build_id=?)) ORDER BY invocation_id`, op.Actor.SiteID, op.DataEpoch, objectID, objectID)
	if err != nil {
		return nil, storageError(err)
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, storageError(err)
		}
		if len(result) == 65536 {
			return nil, platform.NewError(platform.CodeBudgetExceeded, false, "原任务付费引用超过完整响应预算")
		}
		result = append(result, id)
	}
	if err = rows.Err(); err != nil {
		return nil, storageError(err)
	}
	return result, nil
}

// CommitExecutionResult 和本域最终业务状态共用事务，防止业务已完成而jobs回执未知后重算。
func CommitExecutionResult(ctx context.Context, tx *sql.Tx, op OperationContext, jobType, objectID, leaseID string, body any) (platform.JobExecutionResult, error) {
	var jobID string
	err := tx.QueryRowContext(ctx, `SELECT job_id FROM execution_records WHERE site_id=? AND data_epoch=? AND job_type=? AND object_id=?`, op.Actor.SiteID, op.DataEpoch, jobType, objectID).Scan(&jobID)
	if err != nil {
		return platform.JobExecutionResult{}, platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, false, "最终业务状态缺原任务拥有者记录")
	}
	record, err := LoadExecutionRecord(ctx, tx, jobID)
	if err != nil {
		return platform.JobExecutionResult{}, err
	}
	if record.Operation.OperationID != op.OperationID || record.Operation.RootOperationID != op.RootOperationID || ExecutionLeaseID(jobID, record.Fence) != leaseID || record.CancelRequested || !record.LeaseUntil.After(time.Now()) {
		return platform.JobExecutionResult{}, platform.NewError(platform.CodeStaleResult, false, "最终结果的原任务写资格已失效")
	}
	raw, err := platform.CanonicalJSON(body)
	if err != nil {
		return platform.JobExecutionResult{}, err
	}
	if len(raw) > 16<<20 {
		return platform.JobExecutionResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "完整最终任务结果超过16MiB持久读取预算")
	}
	ref := platform.ObjectRef{Owner: "knowledge", ObjectID: objectID + ".result", Version: record.InputRef.Version, SHA256: platform.HashBytes(raw), MediaType: "application/json", ByteSize: int64(len(raw))}
	if record.ResultRef != nil {
		if *record.ResultRef != ref {
			return platform.JobExecutionResult{}, platform.NewError(platform.CodeConflict, false, "原任务已保存不同最终结果")
		}
		return platform.JobExecutionResult{State: platform.JobSucceeded, ResultRef: record.ResultRef, PaidResultRefs: record.PaidRefs}, nil
	}
	paid, err := ReadPaidResultRefs(ctx, tx, op, objectID)
	if err != nil {
		return platform.JobExecutionResult{}, err
	}
	refRaw, _ := platform.CanonicalJSON(ref)
	paidRaw, _ := platform.CanonicalJSON(paid)
	changed, err := tx.ExecContext(ctx, `UPDATE execution_records SET state='completed',result_json=?,result_ref_json=?,paid_refs_json=? WHERE job_id=? AND site_id=? AND data_epoch=? AND worker_id=? AND fence=? AND result_ref_json IS NULL`, raw, refRaw, paidRaw, jobID, op.Actor.SiteID, op.DataEpoch, record.WorkerID, record.Fence)
	if err != nil {
		return platform.JobExecutionResult{}, storageError(err)
	}
	n, err := changed.RowsAffected()
	if err != nil {
		return platform.JobExecutionResult{}, storageError(err)
	}
	if n != 1 {
		return platform.JobExecutionResult{}, platform.NewError(platform.CodeStaleResult, false, "最终结果任务CAS已失效")
	}
	dispatch, key := "build_dispatches", "build_id"
	if jobType == "knowledge.extract" {
		dispatch = "extraction_dispatches"
		key = "extraction_id"
	}
	if _, err = tx.ExecContext(ctx, "UPDATE "+dispatch+" SET dispatch_state='completed',updated_at=? WHERE "+key+"=? AND job_id=?", platform.Now(), objectID, jobID); err != nil {
		return platform.JobExecutionResult{}, storageError(err)
	}
	return platform.JobExecutionResult{State: platform.JobSucceeded, ResultRef: &ref, PaidResultRefs: paid}, nil
}

// RecordExecutionCancellation 持久jobs真实取消观察；只关原fence写资格，不宣称在途费用已结算。
func RecordExecutionCancellation(ctx context.Context, op OperationContext, job platform.ClaimedJob) error {
	work, tx, cancel, err := BeginIndexWrite(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	record, err := LoadExecutionRecord(work, tx, job.JobID)
	if err != nil {
		return err
	}
	if record.Fence != job.FencingToken || record.WorkerID != job.WorkerID || record.Attempt != job.Attempt || record.InputRef != job.InputRef || record.JobType != job.JobType || record.Operation.RootOperationID != job.RootOperationID || record.Operation.OperationID != job.OperationID || record.Operation.Actor.SiteID != op.Actor.SiteID || record.Operation.DataEpoch != op.DataEpoch {
		return platform.NewError(platform.CodeStaleResult, false, "取消观察与原任务拥有者不一致")
	}
	if _, err = tx.ExecContext(work, `UPDATE execution_records SET cancel_requested=1 WHERE job_id=? AND fence=? AND worker_id=? AND data_epoch=?`, job.JobID, job.FencingToken, job.WorkerID, op.DataEpoch); err != nil {
		return storageError(err)
	}
	dispatch, object, key, state := "build_dispatches", "builds", "build_id", "cancelled"
	if job.JobType == "knowledge.extract" {
		dispatch = "extraction_dispatches"
		object = "extractions"
		key = "extraction_id"
		state = "failed"
	}
	if _, err = tx.ExecContext(work, "UPDATE "+dispatch+" SET cancel_requested=1,updated_at=? WHERE "+key+"=? AND job_id=?", platform.Now(), record.ObjectID, job.JobID); err != nil {
		return storageError(err)
	}
	if _, err = tx.ExecContext(work, "UPDATE "+object+" SET cancelled=1,lease_id='',lease_until='',state=?,failure_code=? WHERE "+key+"=? AND state NOT IN ('active','ready','unsupported','failed','cancelled','superseded')", state, platform.CodeCancelled, record.ObjectID); err != nil {
		return storageError(err)
	}
	if job.JobType == "knowledge.build" {
		if err = CancelUnusedExtractions(work, tx, op, record.ObjectID); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return storageError(err)
	}
	return nil
}

// FinishExecutionFailure 只写原worker/fence的真实失败诊断；控制窗口不延长原付费批准。
// 返回原错误及完整已知付费引用；若业务已原子完成则仅返回真实原成功结果。
func FinishExecutionFailure(ctx context.Context, op OperationContext, input platform.JobExecutionInput, cause error) (platform.JobExecutionResult, error) {
	control, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	deadline, _ := control.Deadline()
	metadata := op
	metadata.Deadline = deadline.UTC().Format(time.RFC3339Nano)
	metadata.Actor.ExpiresAt = metadata.Deadline
	work, tx, closeWrite, err := BeginIndexWrite(platform.WithPortalProof(control, ""), metadata)
	if err != nil {
		return platform.JobExecutionResult{}, cause
	}
	defer closeWrite()
	defer tx.Rollback()
	record, err := LoadExecutionRecord(work, tx, input.JobID)
	if err != nil {
		return platform.JobExecutionResult{}, cause
	}
	if record.InputRef != input.InputRef || record.Fence != input.FencingToken || record.WorkerID != input.WorkerID || record.Attempt != input.Attempt || record.JobType != input.JobType || record.Operation.DataEpoch != op.DataEpoch || record.Operation.RootOperationID != op.RootOperationID || record.Operation.OperationID != op.OperationID || record.Operation.Actor.SiteID != op.Actor.SiteID {
		return platform.JobExecutionResult{}, cause
	}
	if record.State == "completed" && record.ResultRef != nil {
		return platform.JobExecutionResult{State: platform.JobSucceeded, ResultRef: record.ResultRef, PaidResultRefs: record.PaidRefs}, nil
	}
	code := platform.CodeInternal
	var failure *platform.Error
	if errors.As(cause, &failure) {
		code = failure.Code
	} else if errors.Is(cause, context.DeadlineExceeded) {
		code = platform.CodeDeadlineExceeded
	} else if errors.Is(cause, context.Canceled) {
		code = platform.CodeCancelled
	}
	paid, err := ReadPaidResultRefs(work, tx, op, record.ObjectID)
	if err != nil {
		return platform.JobExecutionResult{}, cause
	}
	object, key := "builds", "build_id"
	if record.JobType == "knowledge.extract" {
		object = "extractions"
		key = "extraction_id"
	}
	if _, err = tx.ExecContext(work, "UPDATE "+object+" SET state='failed',failure_code=?,lease_id='',lease_until='' WHERE "+key+"=? AND site_id=? AND data_epoch=? AND lease_id=? AND cancelled=0 AND state NOT IN ('active','ready','unsupported','superseded')", code, record.ObjectID, op.Actor.SiteID, op.DataEpoch, ExecutionLeaseID(input.JobID, input.FencingToken)); err != nil {
		return platform.JobExecutionResult{PaidResultRefs: paid}, cause
	}
	if _, err = tx.ExecContext(work, `UPDATE execution_records SET state='failed' WHERE job_id=? AND worker_id=? AND fence=? AND result_ref_json IS NULL`, input.JobID, input.WorkerID, input.FencingToken); err != nil {
		return platform.JobExecutionResult{PaidResultRefs: paid}, cause
	}
	if err = tx.Commit(); err != nil {
		return platform.JobExecutionResult{PaidResultRefs: paid}, cause
	}
	return platform.JobExecutionResult{PaidResultRefs: paid}, cause
}
