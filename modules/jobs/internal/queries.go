package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

type ManagementPageRequest struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}
type ManagementPageInfo struct {
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
}
type JobProgressSummary struct {
	Seq            int64  `json:"seq"`
	Phase          string `json:"phase"`
	CompletedUnits int64  `json:"completedUnits"`
	TotalUnits     int64  `json:"totalUnits"`
}
type JobSummary struct {
	Trigger         string              `json:"trigger,omitempty"`
	TargetLanguage  string              `json:"targetLanguage,omitempty"`
	CreatedAt       string              `json:"createdAt,omitempty"`
	JobID           string              `json:"jobId"`
	OperationID     string              `json:"operationId"`
	RootOperationID *string             `json:"rootOperationId"`
	Owner           string              `json:"owner"`
	Type            string              `json:"type"`
	State           string              `json:"state"`
	Revision        int64               `json:"revision"`
	Attempt         int64               `json:"attempt"`
	LastErrorCode   string              `json:"lastErrorCode"`
	Progress        *JobProgressSummary `json:"progress,omitempty"`
}
type ListJobsInput struct {
	Page  ManagementPageRequest `json:"page"`
	State string                `json:"state"`
	Owner string                `json:"owner,omitempty"`
}
type ListJobsResult struct {
	Items    []JobSummary       `json:"items"`
	PageInfo ManagementPageInfo `json:"pageInfo"`
}
type DeliverySummary struct {
	DeliveryID    string `json:"deliveryId"`
	EventType     string `json:"eventType"`
	EventVersion  int    `json:"eventVersion"`
	Producer      string `json:"producer"`
	Consumer      string `json:"consumer"`
	Attempts      int64  `json:"attempts"`
	LastErrorCode string `json:"lastErrorCode"`
}
type ListDeliveriesInput struct {
	Page     ManagementPageRequest `json:"page"`
	Producer string                `json:"producer,omitempty"`
}
type ListDeliveriesResult struct {
	Items    []DeliverySummary  `json:"items"`
	PageInfo ManagementPageInfo `json:"pageInfo"`
}
type RetryJobInput struct {
	JobID string `json:"jobId"`
}
type RetryJobResult struct {
	Job JobSummary `json:"job"`
}

type managementCursor struct {
	ID string `json:"id"`
}
type managementKeyContext struct{}
type managementItem struct {
	id         string
	permission string
	job        JobSummary
	delivery   DeliverySummary
}
type retryCandidate struct {
	job     jobRecord
	request OwnerRecoveryRequest
	checker OwnerRecoveryChecker
}

// WithManagementKey 只注入可信启动配置；游标签名与验签均复用平台。
func WithManagementKey(ctx context.Context, key platform.SigningKey) context.Context {
	return context.WithValue(ctx, managementKeyContext{}, key)
}

// ListJobs 先逐资源核当前业务权限，再从有权条目计算分页。
func ListJobs(ctx context.Context, op OperationContext, input ListJobsInput) (ListJobsResult, error) {
	if input.Page.Limit == 0 {
		input.Page.Limit = 20
	}
	if input.State == "" {
		input.State = "active"
	}
	filters, _ := json.Marshal(struct {
		Owner, State string
		Limit        int
	}{input.Owner, input.State, input.Page.Limit})
	position, scope, err := managementPage(ctx, op, "jobs", string(filters), input.Page)
	if err != nil {
		return ListJobsResult{}, err
	}
	items := []managementItem{}
	for len(items) <= input.Page.Limit {
		batch, err := listJobBatch(ctx, op, input, position.ID)
		if err != nil {
			return ListJobsResult{}, err
		}
		if len(batch) == 0 {
			break
		}
		position.ID = batch[len(batch)-1].id
		allowed, err := filterManagementBatch(ctx, op, batch, input.Page.Limit+1-len(items))
		if err != nil {
			return ListJobsResult{}, err
		}
		items = append(items, allowed...)
		if len(batch) < 100 {
			break
		}
	}
	page, err := finishManagementPage(ctx, scope, items, input.Page.Limit)
	if err != nil {
		return ListJobsResult{}, err
	}
	if len(items) > input.Page.Limit {
		items = items[:input.Page.Limit]
	}
	result := ListJobsResult{Items: make([]JobSummary, 0, len(items)), PageInfo: page}
	for _, item := range items {
		result.Items = append(result.Items, item.job)
	}
	return result, nil
}

// ListDeliveries 只暴露经过生产方具体管理权限过滤的死信元数据。
func ListDeliveries(ctx context.Context, op OperationContext, input ListDeliveriesInput) (ListDeliveriesResult, error) {
	if input.Page.Limit == 0 {
		input.Page.Limit = 20
	}
	filters, _ := json.Marshal(struct {
		Producer string
		Limit    int
	}{input.Producer, input.Page.Limit})
	position, scope, err := managementPage(ctx, op, "deliveries", string(filters), input.Page)
	if err != nil {
		return ListDeliveriesResult{}, err
	}
	items := []managementItem{}
	for len(items) <= input.Page.Limit {
		batch, err := listDeliveryBatch(ctx, op, input, position.ID)
		if err != nil {
			return ListDeliveriesResult{}, err
		}
		if len(batch) == 0 {
			break
		}
		position.ID = batch[len(batch)-1].id
		allowed, err := filterManagementBatch(ctx, op, batch, input.Page.Limit+1-len(items))
		if err != nil {
			return ListDeliveriesResult{}, err
		}
		items = append(items, allowed...)
		if len(batch) < 100 {
			break
		}
	}
	page, err := finishManagementPage(ctx, scope, items, input.Page.Limit)
	if err != nil {
		return ListDeliveriesResult{}, err
	}
	if len(items) > input.Page.Limit {
		items = items[:input.Page.Limit]
	}
	result := ListDeliveriesResult{Items: make([]DeliverySummary, 0, len(items)), PageInfo: page}
	for _, item := range items {
		result.Items = append(result.Items, item.delivery)
	}
	return result, nil
}

func managementPage(ctx context.Context, op OperationContext, kind string, filters string, page ManagementPageRequest) (managementCursor, string, error) {
	if op.Actor.Kind != "admin" {
		return managementCursor{}, "", platform.NewError(platform.CodeForbidden, false, "管理列表仅接受当前后台主体")
	}
	key, ok := ctx.Value(managementKeyContext{}).(platform.SigningKey)
	if !ok {
		return managementCursor{}, "", platform.NewError(platform.CodeDependencyUnavailable, true, "管理游标签名器尚未装配")
	}
	binding, _ := json.Marshal(struct {
		Kind, Filters, Caller, Epoch, Subject, Site, Session string
		SessionRevision, PermissionRevision                  int64
	}{kind, filters, op.CallerService, op.DataEpoch, op.Actor.SubjectID, op.Actor.SiteID, op.Actor.SessionID, op.Actor.SessionRevision, op.Actor.PermissionRevision})
	scope := "jobs.management.v1:" + platform.HashBytes(binding)
	position := managementCursor{}
	if page.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(key, scope, page.Cursor)
		if err != nil {
			return managementCursor{}, "", err
		}
		if json.Unmarshal(raw, &position) != nil || position.ID == "" {
			return managementCursor{}, "", platform.NewError(platform.CodeValidation, false, "管理游标载荷不可用")
		}
	}
	return position, scope, nil
}

func finishManagementPage(ctx context.Context, scope string, items []managementItem, limit int) (ManagementPageInfo, error) {
	page := ManagementPageInfo{HasMore: len(items) > limit}
	if !page.HasMore {
		return page, nil
	}
	key, _ := ctx.Value(managementKeyContext{}).(platform.SigningKey)
	raw, _ := json.Marshal(managementCursor{ID: items[limit-1].id})
	cursor, err := serviceauth.SignCursor(key, scope, raw)
	if err != nil {
		return ManagementPageInfo{}, err
	}
	page.NextCursor = cursor
	return page, nil
}

func listJobBatch(ctx context.Context, op OperationContext, input ListJobsInput, after string) ([]managementItem, error) {
	work, err := beginLeaseOperation(ctx, op, op.DataEpoch)
	if err != nil {
		return nil, err
	}
	defer work.close()
	rows, err := work.tx.QueryContext(ctx, `SELECT job_id FROM jobs WHERE data_epoch=? AND (?='' OR (created_at,job_id)<((SELECT created_at FROM jobs WHERE job_id=? AND data_epoch=?),?)) AND (?='' OR owner=?)
        AND (?='all' OR state=? OR (?='active' AND state IN ('queued','leased','running','retry-wait','cancel-requested','orphan')) OR (?='failed' AND state IN ('retry-wait','dead-letter','orphan')))
        AND site_id=? ORDER BY created_at DESC,job_id DESC LIMIT 100`, op.DataEpoch, after, after, op.DataEpoch, after, input.Owner, input.Owner, input.State, input.State, input.State, input.State, op.Actor.SiteID)
	if err != nil {
		return nil, mapQueueDBError(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, mapQueueDBError(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, mapQueueDBError(err)
	}
	items := make([]managementItem, 0, len(ids))
	for _, id := range ids {
		summary, err := readJobSummary(ctx, work, id)
		if err != nil {
			return nil, err
		}
		permission, err := jobManagementPermission(summary.Owner, summary.Type)
		if err != nil {
			return nil, err
		}
		items = append(items, managementItem{id: id, permission: permission, job: summary})
	}
	return items, nil
}

func listDeliveryBatch(ctx context.Context, op OperationContext, input ListDeliveriesInput, after string) ([]managementItem, error) {
	q, err := currentEventQueue()
	if err != nil {
		return nil, err
	}
	letters, err := q.ListDeadLetters(ctx, op.Actor.SiteID, input.Producer, after, 100)
	if err != nil {
		return nil, err
	}
	items := make([]managementItem, 0, len(letters))
	for _, letter := range letters {
		summary := DeliverySummary{DeliveryID: letter.DeliveryID, EventType: letter.EventType, EventVersion: letter.EventVersion, Producer: letter.Producer, Consumer: letter.Consumer, Attempts: int64(letter.Attempts), LastErrorCode: letter.LastError}
		definition, err := findEventDefinition(summary.EventType, summary.EventVersion)
		if err != nil || definition.Producer != summary.Producer || definition.ManagementPermission == "" {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "死信生产方或版本目录不一致")
		}
		items = append(items, managementItem{id: summary.DeliveryID, permission: definition.ManagementPermission, delivery: summary})
	}
	return items, nil
}

func filterManagementBatch(ctx context.Context, op OperationContext, batch []managementItem, needed int) ([]managementItem, error) {
	store := currentStore()
	if store == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "管理存储尚未装配")
	}
	store.mu.RLock()
	client := store.identity
	closed := store.closed
	store.mu.RUnlock()
	if closed || client == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "当前业务授权尚未装配")
	}
	items := []managementItem{}
	for _, item := range batch {
		if err := authorizePermission(ctx, op, client, item.permission, item.id); err != nil {
			var failure *platform.Error
			if errors.As(err, &failure) && failure.Code == platform.CodeForbidden {
				continue
			}
			return nil, err
		}
		items = append(items, item)
		if len(items) == needed {
			break
		}
	}
	return items, nil
}

func readJobSummary(ctx context.Context, work *leaseOperation, jobID string) (JobSummary, error) {
	var result JobSummary
	var root string
	err := work.tx.QueryRowContext(ctx, `SELECT job_id,operation_id,root_operation_id,owner,type,state,revision,attempt,last_error_code,trigger_event,target_language,created_at
        FROM jobs WHERE job_id=? AND data_epoch=?`, jobID, work.runtime.Runtime.DataEpoch).Scan(&result.JobID, &result.OperationID, &root, &result.Owner, &result.Type, &result.State, &result.Revision, &result.Attempt, &result.LastErrorCode, &result.Trigger, &result.TargetLanguage, &result.CreatedAt)
	if err != nil {
		return JobSummary{}, mapQueueDBError(err)
	}
	if root != "" {
		result.RootOperationID = &root
	}
	var progress JobProgressSummary
	err = work.tx.QueryRowContext(ctx, `SELECT seq,phase,completed_units,total_units FROM progress WHERE job_id=? ORDER BY seq DESC LIMIT 1`, jobID).Scan(&progress.Seq, &progress.Phase, &progress.CompletedUnits, &progress.TotalUnits)
	if err == nil {
		result.Progress = &progress
	} else if !errors.Is(err, sql.ErrNoRows) {
		return JobSummary{}, mapQueueDBError(err)
	}
	return result, nil
}

// RetryJob 先回原回执；只有owner证明无未决费用时才沿原付费根重排。
func RetryJob(ctx context.Context, op OperationContext, input RetryJobInput) (RetryJobResult, error) {
	authorized, err := authorizeJobManagement(ctx, op, input.JobID)
	if err != nil {
		return RetryJobResult{}, err
	}
	candidate, prior, err := inspectManualRetry(ctx, op, input, authorized)
	if err != nil {
		return RetryJobResult{}, err
	}
	if prior != nil {
		return *prior, nil
	}
	recovery, err := candidate.checker(ctx, op, candidate.request)
	if err != nil {
		return RetryJobResult{}, err
	}
	if recovery.Status != "completed" && recovery.Status != "safeToRetry" {
		return RetryJobResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "owner尚未确认任务结果或费用安全")
	}
	if recovery.Status == "safeToRetry" && candidate.request.RootOperationID == "" {
		return RetryJobResult{}, platform.NewError(platform.CodeConflict, false, "历史任务付费根未知，不能人工重排")
	}
	return applyManualRetry(ctx, op, input, candidate, recovery)
}

func inspectManualRetry(ctx context.Context, op OperationContext, input RetryJobInput, authorized jobRecord) (retryCandidate, *RetryJobResult, error) {
	work, err := beginLeaseOperation(ctx, op, op.DataEpoch)
	if err != nil {
		return retryCandidate{}, nil, err
	}
	defer work.close()
	job, err := work.readJobRecord(ctx, input.JobID, op.DataEpoch)
	if err != nil {
		return retryCandidate{}, nil, err
	}
	if job.owner != authorized.owner || job.jobType != authorized.jobType || job.epoch != authorized.epoch {
		return retryCandidate{}, nil, platform.NewError(platform.CodeConflict, false, "任务授权作用域已变化")
	}
	prior, err := readRetryReceipt(ctx, op, work, input)
	if err != nil || prior != nil {
		return retryCandidate{}, prior, err
	}
	if job.state != "dead-letter" || job.cancelRequested {
		return retryCandidate{}, nil, platform.NewError(platform.CodeConflict, false, "只能人工处理未取消的失败终局")
	}
	request := OwnerRecoveryRequest{JobID: job.id, Owner: job.owner, Type: job.jobType, DataEpoch: job.epoch, ExpiredFence: job.fence, Attempt: job.attempt}
	var encoded []byte
	if err := work.tx.QueryRowContext(ctx, `SELECT immutable_input_ref,root_operation_id FROM jobs WHERE job_id=?`, job.id).Scan(&encoded, &request.RootOperationID); err != nil {
		return retryCandidate{}, nil, mapQueueDBError(err)
	}
	if json.Unmarshal(encoded, &request.InputRef) != nil || request.InputRef.Owner != job.owner {
		return retryCandidate{}, nil, platform.NewError(platform.CodeIntegrityError, false, "原任务输入引用不可用")
	}
	var leaseCount int
	if err := work.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM leases WHERE job_id=?`, job.id).Scan(&leaseCount); err != nil {
		return retryCandidate{}, nil, mapQueueDBError(err)
	}
	if leaseCount != 0 {
		return retryCandidate{}, nil, platform.NewError(platform.CodeIntegrityError, false, "失败终局仍有执行租约")
	}
	if work.store.recovery == nil {
		return retryCandidate{}, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "owner持久恢复确认尚未装配")
	}
	return retryCandidate{job: job, request: request, checker: work.store.recovery}, nil, nil
}

func applyManualRetry(ctx context.Context, op OperationContext, input RetryJobInput, candidate retryCandidate, recovery OwnerRecoveryResult) (RetryJobResult, error) {
	work, err := beginLeaseOperation(ctx, op, op.DataEpoch)
	if err != nil {
		return RetryJobResult{}, err
	}
	defer work.close()
	job, err := work.readJobRecord(ctx, input.JobID, op.DataEpoch)
	if err != nil {
		return RetryJobResult{}, err
	}
	if job.owner != candidate.job.owner || job.jobType != candidate.job.jobType || job.epoch != candidate.job.epoch {
		return RetryJobResult{}, platform.NewError(platform.CodeConflict, false, "任务授权作用域已变化")
	}
	prior, err := readRetryReceipt(ctx, op, work, input)
	if err != nil {
		return RetryJobResult{}, err
	}
	if prior != nil {
		return *prior, nil
	}
	if job.state != "dead-letter" || job.cancelRequested || job.revision != candidate.job.revision || job.fence != candidate.job.fence {
		return RetryJobResult{}, platform.NewError(platform.CodeConflict, false, "owner确认期间任务状态已变化")
	}
	if err := preserveRetryResults(ctx, work, job, recovery); err != nil {
		return RetryJobResult{}, err
	}
	state := "queued"
	attempt := int64(0)
	if recovery.Status == "completed" {
		state, attempt = "succeeded", job.attempt
	}
	updated, err := work.tx.ExecContext(ctx, `UPDATE jobs SET state=?,attempt=?,fencing_counter=fencing_counter+1,revision=revision+1,last_error_code='',not_before=?,updated_at=?
        WHERE job_id=? AND data_epoch=? AND state='dead-letter' AND cancel_requested=0 AND revision=? AND fencing_counter=?`,
		state, attempt, work.now.Format(time.RFC3339Nano), work.now.Format(time.RFC3339Nano), job.id, job.epoch, job.revision, job.fence)
	if err != nil {
		return RetryJobResult{}, mapQueueDBError(err)
	}
	if count, err := updated.RowsAffected(); err != nil || count != 1 {
		return RetryJobResult{}, platform.NewError(platform.CodeConflict, false, "失败终局已被其他操作处理")
	}
	if err := work.enqueueState(job.id, job.owner, job.epoch, state, job.revision+1); err != nil {
		return RetryJobResult{}, err
	}
	summary, err := readJobSummary(ctx, work, job.id)
	if err != nil {
		return RetryJobResult{}, err
	}
	result := RetryJobResult{Job: summary}
	encoded, _ := json.Marshal(result)
	if _, err := work.tx.ExecContext(ctx, `INSERT INTO job_management_operations(caller,data_epoch,operation_id,request_hash,result_json,created_at) VALUES(?,?,?,?,?,?)`,
		op.CallerService, op.DataEpoch, op.OperationID, retryRequestHash(input, op.Actor.SubjectID, op.Actor.SiteID), encoded, work.now.Format(time.RFC3339Nano)); err != nil {
		return RetryJobResult{}, mapQueueDBError(err)
	}
	if err := work.tx.Commit(); err != nil {
		return RetryJobResult{}, mapQueueDBError(err)
	}
	return result, nil
}

func readRetryReceipt(ctx context.Context, op OperationContext, work *leaseOperation, input RetryJobInput) (*RetryJobResult, error) {
	var hash string
	var encoded []byte
	err := work.tx.QueryRowContext(ctx, `SELECT request_hash,result_json FROM job_management_operations WHERE caller=? AND data_epoch=? AND operation_id=?`, op.CallerService, op.DataEpoch, op.OperationID).Scan(&hash, &encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapQueueDBError(err)
	}
	if hash != retryRequestHash(input, op.Actor.SubjectID, op.Actor.SiteID) {
		return nil, platform.NewError(platform.CodeConflict, false, "人工操作键已绑定其他输入或主体")
	}
	var result RetryJobResult
	if json.Unmarshal(encoded, &result) != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "人工任务操作回执损坏")
	}
	return &result, nil
}

func retryRequestHash(input RetryJobInput, subjectID string, siteID string) string {
	encoded, _ := json.Marshal(struct{ JobID, SubjectID, SiteID string }{input.JobID, subjectID, siteID})
	return platform.HashBytes(encoded)
}

func preserveRetryResults(ctx context.Context, work *leaseOperation, job jobRecord, recovery OwnerRecoveryResult) error {
	if recovery.ResultRef != nil && recovery.ResultRef.Owner != job.owner {
		return platform.NewError(platform.CodeIntegrityError, false, "owner确认结果不属于原任务")
	}
	var encoded, hash string
	err := work.tx.QueryRowContext(ctx, `SELECT result_ref,result_hash FROM job_results WHERE job_id=?`, job.id).Scan(&encoded, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return mapQueueDBError(err)
	}
	if encoded != "" {
		var old ObjectRef
		if json.Unmarshal([]byte(encoded), &old) != nil || old.Sha256 != hash || old.Owner != job.owner {
			return platform.NewError(platform.CodeIntegrityError, false, "原任务完整结果引用损坏")
		}
		if recovery.ResultRef != nil && old != *recovery.ResultRef {
			return platform.NewError(platform.CodeIntegrityError, false, "owner确认与原完整结果引用矛盾")
		}
	}
	return work.mergePaidResults(ctx, job.id, recovery.PaidResultRefs, recovery.ResultRef)
}
