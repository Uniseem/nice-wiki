package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"strings"
	"time"
)

// AggregateJobBody是本域持久不可变元数据对象，不包含原访客元数据。
type AggregateJobBody struct {
	SchemaVersion   int    `json:"schemaVersion"`
	Kind            string `json:"kind"`
	Period          string `json:"period"`
	Timezone        string `json:"timezone"`
	SourceWatermark int64  `json:"sourceWatermark"`
	SiteID          string `json:"siteId"`
	DataEpoch       string `json:"dataEpoch"`
}
type aggregateIntent struct {
	ID, Kind, Key, Operation, Root string
	Body                           AggregateJobBody
	Ref                            platform.ObjectRef
}
type JobSubmitter interface {
	CreateJob(context.Context, any, string) (json.RawMessage, error)
}

// SchedulePageAggregates选择真实变动的已封闭周期；每分钟有界推进，不补造来源水位。
func SchedulePageAggregates(ctx context.Context, op OperationContext) error {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return err
	}
	if err = admitAggregateScheduling(ctx, rt); err != nil {
		return err
	}
	if rt.CountingLocation == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "可信报表时区未配置")
	}
	now := time.Now()
	hour := now.UTC().Truncate(time.Hour).Add(-time.Hour)
	local := now.In(rt.CountingLocation)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, rt.CountingLocation)
	candidates := map[string]string{}
	if now.Sub(hour.Add(time.Hour)) >= 5*time.Minute {
		candidates["hourly/"+hour.Format(analyticsTimeLayout)] = "hourly"
	}
	if now.Sub(today) >= 10*time.Minute {
		candidates["daily/"+today.AddDate(0, 0, -1).Format("2006-01-02")+"/"+rt.CountingLocation.String()] = "daily"
	}
	rows, err := rt.DB.QueryContext(ctx, `SELECT p.kind,p.period_key FROM beacon_period_watermarks p LEFT JOIN aggregate_runs a ON a.kind=p.kind AND a.period_key=p.period_key WHERE p.period_key>=? AND (a.source_watermark IS NULL OR a.source_watermark<p.revision) ORDER BY p.period_key,p.kind LIMIT 500`, now.AddDate(0, 0, -30).Format("2006-01-02"))
	if err != nil {
		return storageError(err)
	}
	for rows.Next() {
		var kind, key string
		if err = rows.Scan(&kind, &key); err != nil {
			break
		}
		if kind == "hourly" || kind == "daily" {
			candidates[kind+"/"+key] = kind
		}
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return storageError(err)
	}
	for combined, kind := range candidates {
		key := combined[len(kind)+1:]
		period := key
		zone := ""
		var end time.Time
		if kind == "hourly" {
			start, e := time.Parse(analyticsTimeLayout, key)
			if e != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "小时来源期键损坏")
			}
			end = start.Add(time.Hour)
			if end.Add(5*time.Minute).After(now) || start.Before(now.AddDate(0, 0, -30)) {
				continue
			}
		} else {
			suffix := "/" + rt.CountingLocation.String()
			if !strings.HasSuffix(key, suffix) {
				return platform.NewError(platform.CodeVersionMismatch, false, "日来源期键与可信时区不同")
			}
			period = strings.TrimSuffix(key, suffix)
			start, e := time.ParseInLocation("2006-01-02", period, rt.CountingLocation)
			if e != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "日报来源期键损坏")
			}
			zone = rt.CountingLocation.String()
			end = start.AddDate(0, 0, 1)
			if end.Add(10*time.Minute).After(now) || start.Before(today.AddDate(0, 0, -30)) {
				continue
			}
		}
		if err = ensureAggregateIntent(ctx, op, kind, key, period, zone); err != nil {
			return err
		}
	}
	return submitAggregateIntents(ctx, op)
}

func ensureAggregateIntent(ctx context.Context, op OperationContext, kind, key, period, zone string) error {
	work, rt, tx, cancel, err := beginWrite(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	var source int64
	err = tx.QueryRowContext(work, `SELECT revision FROM beacon_period_watermarks WHERE kind=? AND period_key=?`, kind, key).Scan(&source)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if err != nil {
		return storageError(err)
	}
	var done int64
	err = tx.QueryRowContext(work, `SELECT source_watermark FROM aggregate_runs WHERE kind=? AND period_key=?`, kind, key).Scan(&done)
	if err == nil && done == source {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return storageError(err)
	}
	body := AggregateJobBody{SchemaVersion: 1, Kind: kind, Period: period, Timezone: zone, SourceWatermark: source, SiteID: rt.SiteID, DataEpoch: rt.Epoch}
	raw, err := platform.CanonicalJSON(body)
	if err != nil {
		return err
	}
	hash := platform.HashBytes(raw)
	id := "aggregate-input." + hash
	ref := platform.ObjectRef{Owner: "analytics", ObjectID: id, Version: 1, SHA256: hash, MediaType: "application/json", ByteSize: int64(len(raw))}
	refRaw, err := platform.CanonicalJSON(ref)
	if err != nil {
		return err
	}
	operation := "aggregate-operation." + hash
	result, err := tx.ExecContext(work, `INSERT OR IGNORE INTO aggregate_jobs(input_id,job_type,period_key,source_watermark,input_json,input_hash,input_ref,operation_id,root_operation_id,data_epoch) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, "analytics.aggregate-"+kind, key, source, string(raw), hash, string(refRaw), operation, operation, rt.Epoch)
	if err != nil {
		return storageError(err)
	}
	added, err := result.RowsAffected()
	if err != nil {
		return storageError(err)
	}
	if added != 0 {
		if _, err = advanceWatermark(work, tx, "aggregate-intents"); err != nil {
			return err
		}
	}
	return storageError(tx.Commit())
}

func submitAggregateIntents(ctx context.Context, op OperationContext) error {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return err
	}
	if rt.Submitter == nil || rt.ValidateCreateJob == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "任务提交端口未装配")
	}
	rows, err := rt.DB.QueryContext(ctx, `SELECT input_id,job_type,operation_id,root_operation_id,input_ref FROM aggregate_jobs WHERE submit_receipt='' AND data_epoch=? ORDER BY input_id LIMIT 100`, rt.Epoch)
	if err != nil {
		return storageError(err)
	}
	intents := []aggregateIntent{}
	for rows.Next() {
		var intent aggregateIntent
		var raw string
		if err = rows.Scan(&intent.ID, &intent.Kind, &intent.Operation, &intent.Root, &raw); err != nil {
			break
		}
		if json.Unmarshal([]byte(raw), &intent.Ref) != nil {
			err = platform.NewError(platform.CodeIntegrityError, false, "任务输入引用损坏")
			break
		}
		intents = append(intents, intent)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return storageError(err)
	}
	for _, intent := range intents {
		if err = admitAggregateScheduling(ctx, rt); err != nil {
			return err
		}
		call, cancel, callOp := backgroundOperation(ctx, rt, intent.Operation, intent.Root, 30*time.Second)
		raw, e := rt.Submitter.CreateJob(platform.WithOperationContext(call, callOp), struct {
			Owner  string             `json:"owner"`
			Type   string             `json:"type"`
			Ref    platform.ObjectRef `json:"inputRef"`
			Dedupe string             `json:"dedupeKey"`
			Epoch  string             `json:"dataEpoch"`
		}{"analytics", intent.Kind, intent.Ref, intent.ID, rt.Epoch}, intent.ID)
		cancel()
		if e != nil {
			return e
		}
		if e = rt.ValidateCreateJob(raw); e != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "jobs创建响应不符合唯一生产合同")
		}
		var result struct {
			Job struct {
				ID        string `json:"jobId"`
				Operation string `json:"operationId"`
				State     string `json:"state"`
			} `json:"job"`
		}
		if json.Unmarshal(raw, &result) != nil || result.Job.Operation != intent.Operation || result.Job.State != "queued" {
			return platform.NewError(platform.CodeIntegrityError, false, "jobs创建回执与原任务输入不符")
		}
		work, _, tx, done, e := beginWrite(ctx, op)
		if e != nil {
			return e
		}
		updated, e := tx.ExecContext(work, `UPDATE aggregate_jobs SET job_id=?,submit_receipt=? WHERE input_id=? AND data_epoch=? AND (job_id='' OR job_id=?)`, result.Job.ID, string(raw), intent.ID, rt.Epoch, result.Job.ID)
		if e == nil {
			var count int64
			count, e = updated.RowsAffected()
			if e == nil && count != 1 {
				e = platform.NewError(platform.CodeConflict, false, "jobs回执与已领取任务不一致")
			}
		}
		if e == nil {
			_, e = advanceWatermark(work, tx, "aggregate-submit")
		}
		if e == nil {
			e = tx.Commit()
		}
		_ = tx.Rollback()
		done()
		if e != nil {
			return storageError(e)
		}
	}
	return nil
}

// backgroundOperation清用户证明并为每次IO分配截止，保留精确原任务操作与根。
func backgroundOperation(ctx context.Context, rt *Runtime, operation, root string, duration time.Duration) (context.Context, context.CancelFunc, OperationContext) {
	work, cancel := context.WithTimeout(platform.WithPortalProof(ctx, ""), duration)
	deadline, _ := work.Deadline()
	if operation == "" {
		operation = platform.NewIdentifier()
	}
	if root == "" {
		root = operation
	}
	op := OperationContext{RequestID: platform.NewIdentifier(), OperationID: operation, RootOperationID: root, CallerService: "analytics", Actor: platform.AuthContext{Kind: "service", SubjectID: "analytics", SiteID: rt.SiteID, ExpiresAt: deadline.UTC().Format(time.RFC3339Nano), Grants: []string{}}, DataEpoch: rt.Epoch, Deadline: deadline.UTC().Format(time.RFC3339Nano)}
	return WithRuntime(platform.WithOperationContext(work, op), rt), cancel, op
}

func aggregateJobOperation(ctx context.Context, rt *Runtime, job platform.ClaimedJob) (OperationContext, error) {
	var operation, root, ref, kind, epoch, id string
	err := rt.DB.QueryRowContext(ctx, `SELECT operation_id,root_operation_id,input_ref,job_type,data_epoch,job_id FROM aggregate_jobs WHERE input_id=?`, job.InputRef.ObjectID).Scan(&operation, &root, &ref, &kind, &epoch, &id)
	if err != nil {
		return OperationContext{}, storageError(err)
	}
	expected, _ := platform.CanonicalJSON(job.InputRef)
	if ref != string(expected) || epoch != rt.Epoch || job.DataEpoch != epoch || operation != job.OperationID || root != job.RootOperationID || kind != job.JobType || (id != "" && id != job.JobID) {
		return OperationContext{}, platform.NewError(platform.CodeConflict, false, "领取任务与原不可变业务输入不同")
	}
	_, cancel, op := backgroundOperation(ctx, rt, operation, root, 30*time.Minute)
	cancel()
	return op, nil
}

// recordAggregateLease只持久平台真实续租回执；较旧fence不能覆盖新执行权。
func recordAggregateLease(ctx context.Context, op OperationContext, job platform.ClaimedJob, info platform.HeartbeatInfo) error {
	expires, err := time.Parse(time.RFC3339Nano, info.ExpiresAt)
	if err != nil || !expires.After(time.Now()) {
		return platform.NewError(platform.CodeLocked, false, "真实任务租约已过期")
	}
	work, _, tx, cancel, err := beginWrite(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	ref, _ := platform.CanonicalJSON(job.InputRef)
	result, err := tx.ExecContext(work, `UPDATE aggregate_jobs SET job_id=?,worker_id=?,fence=?,attempt=?,lease_expires_at=?,cancelled=0 WHERE input_id=? AND input_ref=? AND operation_id=? AND root_operation_id=? AND data_epoch=? AND (job_id='' OR job_id=?) AND (fence<? OR (fence=? AND worker_id=? AND cancelled=0))`, job.JobID, job.WorkerID, job.FencingToken, job.Attempt, expires.UTC().Format(analyticsTimeLayout), job.InputRef.ObjectID, string(ref), job.OperationID, job.RootOperationID, op.DataEpoch, job.JobID, job.FencingToken, job.FencingToken, job.WorkerID)
	if err != nil {
		return storageError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return storageError(err)
	}
	if count != 1 {
		return platform.NewError(platform.CodeLocked, false, "任务租约镜像已撤销或被新fence接管")
	}
	return storageError(tx.Commit())
}

func cancelAggregateLease(ctx context.Context, op OperationContext, job platform.ClaimedJob) error {
	work, _, tx, cancel, err := beginWrite(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	_, err = tx.ExecContext(work, `UPDATE aggregate_jobs SET cancelled=1 WHERE input_id=? AND job_id=? AND worker_id=? AND fence=? AND data_epoch=?`, job.InputRef.ObjectID, job.JobID, job.WorkerID, job.FencingToken, op.DataEpoch)
	if err != nil {
		return storageError(err)
	}
	return storageError(tx.Commit())
}

func checkAggregateLease(ctx context.Context, execution *platform.JobExecutionInput) error {
	if execution == nil || execution.CheckLease == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "聚合执行缺少真实jobs租约")
	}
	return execution.CheckLease(ctx)
}
func verifyAggregateLease(ctx context.Context, tx *sql.Tx, op OperationContext, execution *platform.JobExecutionInput) error {
	if execution == nil {
		return platform.NewError(platform.CodeLocked, false, "聚合执行租约缺失")
	}
	var expires string
	ref, _ := platform.CanonicalJSON(execution.InputRef)
	err := tx.QueryRowContext(ctx, `SELECT lease_expires_at FROM aggregate_jobs WHERE input_id=? AND input_ref=? AND job_id=? AND worker_id=? AND fence=? AND attempt=? AND data_epoch=? AND cancelled=0`, execution.InputRef.ObjectID, string(ref), execution.JobID, execution.WorkerID, execution.FencingToken, execution.Attempt, op.DataEpoch).Scan(&expires)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.NewError(platform.CodeLocked, false, "聚合执行镜像与当前fence不同")
	}
	if err != nil {
		return storageError(err)
	}
	at, err := time.Parse(analyticsTimeLayout, expires)
	if err != nil || !at.After(time.Now()) {
		return platform.NewError(platform.CodeLocked, false, "聚合提交超过真实租约截止")
	}
	return nil
}

func executePageAggregate(ctx context.Context, rt *Runtime, execution platform.JobExecutionInput) (platform.JobExecutionResult, error) {
	op, ok := platform.OperationContextFrom(ctx)
	if !ok {
		return platform.JobExecutionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "任务上下文缺失")
	}
	ctx = WithRuntime(ctx, rt)
	var raw, hash string
	if err := rt.DB.QueryRowContext(ctx, `SELECT input_json,input_hash FROM aggregate_jobs WHERE input_id=? AND data_epoch=?`, execution.InputRef.ObjectID, rt.Epoch).Scan(&raw, &hash); err != nil {
		return platform.JobExecutionResult{}, storageError(err)
	}
	if platform.HashBytes([]byte(raw)) != hash || hash != execution.InputRef.SHA256 || int64(len(raw)) != execution.InputRef.ByteSize {
		return platform.JobExecutionResult{}, platform.NewError(platform.CodeIntegrityError, false, "不可变任务输入哈希不一致")
	}
	var body AggregateJobBody
	if json.Unmarshal([]byte(raw), &body) != nil || body.SchemaVersion != 1 || body.SiteID != rt.SiteID || body.DataEpoch != rt.Epoch {
		return platform.JobExecutionResult{}, platform.NewError(platform.CodeIntegrityError, false, "聚合任务输入不符合原站点代次")
	}
	var err error
	switch execution.JobType {
	case "analytics.aggregate-hourly":
		_, err = AggregateHourly(ctx, op, AggregateHourlyInput{Hour: body.Period, DataEpoch: rt.Epoch, Execution: &execution, SourceWatermark: body.SourceWatermark})
	case "analytics.aggregate-daily":
		_, err = AggregateDaily(ctx, op, AggregateDailyInput{Day: body.Period, Timezone: body.Timezone, DataEpoch: rt.Epoch, Execution: &execution, SourceWatermark: body.SourceWatermark})
	default:
		err = platform.NewError(platform.CodeValidation, false, "聚合任务类型未登记")
	}
	if err != nil {
		return platform.JobExecutionResult{}, err
	}
	return platform.JobExecutionResult{State: platform.JobSucceeded}, nil
}

// admitAggregateScheduling在每次外部提交前重读真实运行权威，屏障期间不创建新任务。
func admitAggregateScheduling(ctx context.Context, rt *Runtime) error {
	if rt.Window.Runtime == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "任务运行权威未装配")
	}
	view, err := rt.Window.Runtime(ctx)
	if err != nil {
		return err
	}
	if view.DataEpoch != rt.Epoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "任务计划所属代次已改变")
	}
	if view.Maintenance || view.Phase != platform.PhaseRunning {
		return platform.NewError(platform.CodeLocked, true, "维护屏障停止新统计任务")
	}
	return nil
}

// BackgroundOperation供受信启动装配进行本域journal恢复，不携带历史用户证明。
func BackgroundOperation(ctx context.Context, rt *Runtime, duration time.Duration) (context.Context, context.CancelFunc, OperationContext) {
	return backgroundOperation(ctx, rt, "", "", duration)
}
