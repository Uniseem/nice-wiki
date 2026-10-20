package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/jobs/internal/gen"
)

const (
	scheduleEventType    = "jobs.schedule-changed.v1"
	scheduleEventVersion = 1
	maxSchedulesPerEpoch = 1000
	maxScheduleList      = 256
	schedulePermission   = "job.manage"
)

// CreateScheduleInput 是创建定时的内部输入。dataEpoch只来自可信操作上下文。
type CreateScheduleInput struct {
	Owner     string
	Type      string
	InputRef  ObjectRef
	Cron      string
	Timezone  string
	DataEpoch platform.Identifier
}

// CreateScheduleResult 与生成的创建结果一致。
type CreateScheduleResult = gen.CreateScheduleResult

// UpdateScheduleInput 是修订比较并交换的内部输入。
type UpdateScheduleInput struct {
	ScheduleID       string
	ExpectedRevision int64
	Cron             string
	Timezone         string
	Enabled          bool
	DataEpoch        platform.Identifier
}

// UpdateScheduleResult 与生成的修改结果一致。
type UpdateScheduleResult = gen.UpdateScheduleResult

// ListSchedulesInput 没有筛选字段。
type ListSchedulesInput struct{}

// ListSchedulesResult 与生成的列表结果一致。
type ListSchedulesResult = gen.ListSchedulesResult

type scheduleRow struct {
	id        string
	owner     string
	jobType   string
	inputRef  ObjectRef
	cron      string
	timezone  string
	revision  int64
	enabled   bool
	nextAt    time.Time
	dataEpoch string
}

type scheduleChangedPayload struct {
	ScheduleID string `json:"scheduleId"`
	Owner      string `json:"owner"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
	Revision   int64  `json:"revision"`
	NextAt     string `json:"nextAt"`
}

type scheduleCreateHash struct {
	Owner     string    `json:"owner"`
	Type      string    `json:"type"`
	InputRef  ObjectRef `json:"inputRef"`
	Cron      string    `json:"cron"`
	Timezone  string    `json:"timezone"`
	DataEpoch string    `json:"dataEpoch"`
}

type scheduleUpdateHash struct {
	ScheduleID       string `json:"scheduleId"`
	ExpectedRevision int64  `json:"expectedRevision"`
	Cron             string `json:"cron"`
	Timezone         string `json:"timezone"`
	Enabled          bool   `json:"enabled"`
	DataEpoch        string `json:"dataEpoch"`
}

// CreateSchedule 由后台管理员登记任意白名单任务，或由owner服务登记本域系统默认定时。
// 系统定时使用固定标识；同一代次重复登记直接返回原记录。
func CreateSchedule(ctx context.Context, op OperationContext, in CreateScheduleInput) (CreateScheduleResult, error) {
	if err := contextError(ctx); err != nil {
		return CreateScheduleResult{}, err
	}
	if op.Actor.Kind == "service" {
		return createSystemSchedule(ctx, op, in)
	}
	return createAdminSchedule(ctx, op, in)
}

// UpdateSchedule 按revision替换以后的触发时间。已排队任务保留，停用只是不再产生新tick。
func UpdateSchedule(ctx context.Context, op OperationContext, in UpdateScheduleInput) (UpdateScheduleResult, error) {
	if err := contextError(ctx); err != nil {
		return UpdateScheduleResult{}, err
	}
	if err := authorizeScheduleAdmin(ctx, op, in.ScheduleID); err != nil {
		return UpdateScheduleResult{}, err
	}
	if op.CallerService == "" || !platform.ValidIdentifier(op.OperationID) {
		return UpdateScheduleResult{}, platform.NewError(platform.CodeValidation, false, "定时操作缺少幂等键")
	}
	validated, err := ValidateCron(ctx, op, ValidateCronInput{Expression: in.Cron, Timezone: in.Timezone})
	if err != nil {
		return UpdateScheduleResult{}, err
	}
	next, err := NextFireAt(ctx, op, NextFireAtInput{Expression: validated.Normalized, Timezone: in.Timezone, After: time.Now().UTC()})
	if err != nil {
		return UpdateScheduleResult{}, err
	}
	requestHash, err := scheduleHash(scheduleUpdateHash{
		ScheduleID: in.ScheduleID, ExpectedRevision: in.ExpectedRevision, Cron: validated.Normalized,
		Timezone: in.Timezone, Enabled: in.Enabled, DataEpoch: string(in.DataEpoch),
	})
	if err != nil {
		return UpdateScheduleResult{}, err
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return UpdateScheduleResult{}, err
	}
	defer work.close()
	prior, err := readScheduleReceipt(ctx, work, op, requestHash)
	if err != nil {
		return UpdateScheduleResult{}, err
	}
	if prior != nil {
		var result UpdateScheduleResult
		if json.Unmarshal(prior, &result) != nil {
			return UpdateScheduleResult{}, platform.NewError(platform.CodeIntegrityError, false, "定时回执损坏")
		}
		return result, nil
	}
	row, found, err := readSchedule(ctx, work.tx, in.ScheduleID)
	if err != nil {
		return UpdateScheduleResult{}, err
	}
	if !found || row.dataEpoch != string(in.DataEpoch) || row.revision != in.ExpectedRevision {
		return UpdateScheduleResult{}, scheduleConflict()
	}
	nextAt := next.NextAt.UTC()
	updated, err := work.tx.ExecContext(ctx, `UPDATE schedules SET cron=?, timezone=?, enabled=?, next_fire_at=?, revision=revision+1
		WHERE schedule_id=? AND data_epoch=? AND revision=?`,
		validated.Normalized, in.Timezone, boolInt(in.Enabled), nextAt.Format(time.RFC3339Nano), in.ScheduleID, in.DataEpoch, in.ExpectedRevision)
	if err != nil {
		return UpdateScheduleResult{}, mapQueueDBError(err)
	}
	if count, err := updated.RowsAffected(); err != nil || count != 1 {
		return UpdateScheduleResult{}, scheduleConflict()
	}
	row.cron = validated.Normalized
	row.timezone = in.Timezone
	row.enabled = in.Enabled
	row.nextAt = nextAt
	row.revision++
	if err := enqueueScheduleChanged(work, row); err != nil {
		return UpdateScheduleResult{}, err
	}
	revision, err := scheduleRevision(row.revision)
	if err != nil {
		return UpdateScheduleResult{}, err
	}
	result := UpdateScheduleResult{Revision: revision, NextAt: nextAt}
	if err := saveScheduleReceipt(ctx, work, op, requestHash, result); err != nil {
		return UpdateScheduleResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return UpdateScheduleResult{}, mapQueueDBError(err)
	}
	return result, nil
}

// ListSchedules 返回当前代次的系统定时和管理员定时。超过结果上限时失败关闭，不截断。
func ListSchedules(ctx context.Context, op OperationContext, input ListSchedulesInput) (ListSchedulesResult, error) {
	if err := contextError(ctx); err != nil {
		return ListSchedulesResult{}, err
	}
	if err := authorizeScheduleAdmin(ctx, op, "schedules"); err != nil {
		return ListSchedulesResult{}, err
	}
	work, err := beginLeaseOperation(ctx, op, op.DataEpoch)
	if err != nil {
		return ListSchedulesResult{}, err
	}
	defer work.close()
	rows, err := work.tx.QueryContext(ctx, `SELECT schedule_id, owner, job_type, input_ref, cron, timezone, revision, enabled, next_fire_at, data_epoch
		FROM schedules WHERE data_epoch=? ORDER BY schedule_id LIMIT ?`, op.DataEpoch, maxScheduleList+1)
	if err != nil {
		return ListSchedulesResult{}, mapQueueDBError(err)
	}
	defer rows.Close()
	items := make([]gen.Schedule, 0)
	for rows.Next() {
		if len(items) == maxScheduleList {
			return ListSchedulesResult{}, platform.NewError(platform.CodeValidation, false, "定时列表超过结果上限")
		}
		item, err := scanSchedule(rows)
		if err != nil {
			return ListSchedulesResult{}, err
		}
		if _, err = scheduleRevision(item.revision); err != nil {
			return ListSchedulesResult{}, err
		}
		items = append(items, gen.Schedule{
			Owner: item.owner, Type: item.jobType, InputRef: item.inputRef, Cron: item.cron, Timezone: item.timezone,
			DataEpoch: item.dataEpoch, ScheduleId: item.id, Revision: gen.Revision(item.revision), NextAt: item.nextAt, Enabled: item.enabled,
		})
	}
	if err := rows.Err(); err != nil {
		return ListSchedulesResult{}, mapQueueDBError(err)
	}
	return ListSchedulesResult{Schedules: items}, nil
}

// NewScheduleHandlers 绑定已有的定时管理路由。其余接口仍由统一入口失败关闭。
func NewScheduleHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"createSchedule": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.CreateScheduleRequestBody
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "定时创建请求绑定失败"), 0)
				return
			}
			result, err := CreateSchedule(r.Context(), op, CreateScheduleInput{
				Owner: body.Owner, Type: body.Type, InputRef: body.InputRef, Cron: body.Cron, Timezone: body.Timezone, DataEpoch: op.DataEpoch,
			})
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"updateSchedule": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.UpdateScheduleRequestBody
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "定时修改请求绑定失败"), 0)
				return
			}
			result, err := UpdateSchedule(r.Context(), op, UpdateScheduleInput{
				ScheduleID: r.PathValue("scheduleId"), ExpectedRevision: int64(body.ExpectedRevision),
				Cron: body.Cron, Timezone: body.Timezone, Enabled: body.Enabled, DataEpoch: op.DataEpoch,
			})
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"listSchedules": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := ListSchedules(r.Context(), op, ListSchedulesInput{})
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
	}
}

func createSystemSchedule(ctx context.Context, op OperationContext, in CreateScheduleInput) (CreateScheduleResult, error) {
	if op.CallerService == "" || op.CallerService != in.Owner {
		return CreateScheduleResult{}, platform.NewError(platform.CodeForbidden, false, "只能登记本服务的系统定时")
	}
	catalog, ok := catalogSchedule(in.Owner, in.Type)
	if !ok {
		return CreateScheduleResult{}, platform.NewError(platform.CodeForbidden, false, "该任务类型没有系统默认定时")
	}
	scheduleID, err := systemScheduleID(in.Owner, in.Type)
	if err != nil {
		return CreateScheduleResult{}, err
	}
	existing, found, err := lookupSchedule(ctx, op, in.DataEpoch, scheduleID)
	if err != nil {
		return CreateScheduleResult{}, err
	}
	if found && (existing.owner != in.Owner || existing.jobType != in.Type) {
		return CreateScheduleResult{}, platform.NewError(platform.CodeIntegrityError, false, "系统定时记录与目录不一致")
	}
	if found && existing.dataEpoch == string(in.DataEpoch) {
		return scheduleCreateResult(existing)
	}
	requested, err := ValidateCron(ctx, op, ValidateCronInput{Expression: in.Cron, Timezone: in.Timezone})
	if err != nil {
		return CreateScheduleResult{}, err
	}
	catalogNorm, err := ValidateCron(ctx, op, ValidateCronInput{Expression: catalog, Timezone: in.Timezone})
	if err != nil {
		return CreateScheduleResult{}, err
	}
	if requested.Normalized != catalogNorm.Normalized {
		return CreateScheduleResult{}, invalidCron("系统定时表达式必须与目录一致")
	}
	if err := validateInputRef(in.Owner, in.InputRef); err != nil {
		return CreateScheduleResult{}, err
	}
	now := time.Now().UTC()
	if found {
		return moveSystemSchedule(ctx, op, in, existing, now)
	}
	next, err := NextFireAt(ctx, op, NextFireAtInput{Expression: requested.Normalized, Timezone: in.Timezone, After: now})
	if err != nil {
		return CreateScheduleResult{}, err
	}
	return insertSchedule(ctx, op, in.DataEpoch, scheduleRow{
		id: scheduleID, owner: in.Owner, jobType: in.Type, inputRef: in.InputRef, cron: requested.Normalized,
		timezone: in.Timezone, revision: 1, enabled: true, nextAt: next.NextAt.UTC(), dataEpoch: string(in.DataEpoch),
	})
}

func createAdminSchedule(ctx context.Context, op OperationContext, in CreateScheduleInput) (CreateScheduleResult, error) {
	if err := authorizeScheduleAdmin(ctx, op, "schedules"); err != nil {
		return CreateScheduleResult{}, err
	}
	if op.CallerService == "" || !platform.ValidIdentifier(op.OperationID) {
		return CreateScheduleResult{}, platform.NewError(platform.CodeValidation, false, "定时操作缺少幂等键")
	}
	if !knownJobType(in.Owner, in.Type) {
		return CreateScheduleResult{}, platform.NewError(platform.CodeValidation, false, "任务类型不在白名单")
	}
	if err := validateInputRef(in.Owner, in.InputRef); err != nil {
		return CreateScheduleResult{}, err
	}
	validated, err := ValidateCron(ctx, op, ValidateCronInput{Expression: in.Cron, Timezone: in.Timezone})
	if err != nil {
		return CreateScheduleResult{}, err
	}
	next, err := NextFireAt(ctx, op, NextFireAtInput{Expression: validated.Normalized, Timezone: in.Timezone, After: time.Now().UTC()})
	if err != nil {
		return CreateScheduleResult{}, err
	}
	requestHash, err := scheduleHash(scheduleCreateHash{
		Owner: in.Owner, Type: in.Type, InputRef: in.InputRef, Cron: validated.Normalized,
		Timezone: in.Timezone, DataEpoch: string(in.DataEpoch),
	})
	if err != nil {
		return CreateScheduleResult{}, err
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return CreateScheduleResult{}, err
	}
	defer work.close()
	prior, err := readScheduleReceipt(ctx, work, op, requestHash)
	if err != nil {
		return CreateScheduleResult{}, err
	}
	if prior != nil {
		var result CreateScheduleResult
		if json.Unmarshal(prior, &result) != nil {
			return CreateScheduleResult{}, platform.NewError(platform.CodeIntegrityError, false, "定时回执损坏")
		}
		return result, nil
	}
	row := scheduleRow{
		id: string(platform.NewIdentifier()), owner: in.Owner, jobType: in.Type, inputRef: in.InputRef,
		cron: validated.Normalized, timezone: in.Timezone, revision: 1, enabled: true,
		nextAt: next.NextAt.UTC(), dataEpoch: string(in.DataEpoch),
	}
	if err := persistSchedule(ctx, work, row); err != nil {
		return CreateScheduleResult{}, mapScheduleWrite(err)
	}
	result, err := scheduleCreateResult(row)
	if err != nil {
		return CreateScheduleResult{}, err
	}
	if err := saveScheduleReceipt(ctx, work, op, requestHash, result); err != nil {
		return CreateScheduleResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return CreateScheduleResult{}, mapQueueDBError(err)
	}
	return result, nil
}

func moveSystemSchedule(ctx context.Context, op OperationContext, in CreateScheduleInput, existing scheduleRow, now time.Time) (CreateScheduleResult, error) {
	next, err := NextFireAt(ctx, op, NextFireAtInput{Expression: existing.cron, Timezone: existing.timezone, After: now})
	if err != nil {
		return CreateScheduleResult{}, err
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return CreateScheduleResult{}, err
	}
	defer work.close()
	row, found, err := readSchedule(ctx, work.tx, existing.id)
	if err != nil {
		return CreateScheduleResult{}, err
	}
	if !found || row.owner != in.Owner || row.jobType != in.Type {
		return CreateScheduleResult{}, platform.NewError(platform.CodeIntegrityError, false, "系统定时记录与目录不一致")
	}
	if row.dataEpoch == string(in.DataEpoch) {
		return scheduleCreateResult(row)
	}
	count, err := countSchedules(ctx, work.tx, string(in.DataEpoch))
	if err != nil {
		return CreateScheduleResult{}, err
	}
	if count >= maxSchedulesPerEpoch {
		return CreateScheduleResult{}, platform.NewError(platform.CodeValidation, false, "定时数量已达每站点上限")
	}
	if row.revision != existing.revision {
		return CreateScheduleResult{}, scheduleConflict()
	}
	nextAt := next.NextAt.UTC()
	updated, err := work.tx.ExecContext(ctx, `UPDATE schedules SET data_epoch=?, next_fire_at=?, revision=revision+1
		WHERE schedule_id=? AND revision=?`, string(in.DataEpoch), nextAt.Format(time.RFC3339Nano), row.id, row.revision)
	if err != nil {
		return CreateScheduleResult{}, mapQueueDBError(err)
	}
	if count, err := updated.RowsAffected(); err != nil || count != 1 {
		return CreateScheduleResult{}, scheduleConflict()
	}
	row.dataEpoch = string(in.DataEpoch)
	row.nextAt = nextAt
	row.revision++
	if err := enqueueScheduleChanged(work, row); err != nil {
		return CreateScheduleResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return CreateScheduleResult{}, mapQueueDBError(err)
	}
	return scheduleCreateResult(row)
}

func insertSchedule(ctx context.Context, op OperationContext, epoch platform.Identifier, row scheduleRow) (CreateScheduleResult, error) {
	work, err := beginLeaseOperation(ctx, op, epoch)
	if err != nil {
		return CreateScheduleResult{}, err
	}
	defer work.close()
	current, found, err := readSchedule(ctx, work.tx, row.id)
	if err != nil {
		return CreateScheduleResult{}, err
	}
	if found && current.dataEpoch == string(epoch) && current.owner == row.owner && current.jobType == row.jobType {
		return scheduleCreateResult(current)
	}
	if err := persistSchedule(ctx, work, row); err != nil {
		if isUniqueConstraint(err) {
			current, found, readErr := readSchedule(ctx, work.tx, row.id)
			if readErr != nil {
				return CreateScheduleResult{}, readErr
			}
			if found && current.dataEpoch == string(epoch) && current.owner == row.owner && current.jobType == row.jobType {
				return scheduleCreateResult(current)
			}
		}
		return CreateScheduleResult{}, mapScheduleWrite(err)
	}
	if err := work.tx.Commit(); err != nil {
		return CreateScheduleResult{}, mapQueueDBError(err)
	}
	return scheduleCreateResult(row)
}

func persistSchedule(ctx context.Context, work *leaseOperation, row scheduleRow) error {
	count, err := countSchedules(ctx, work.tx, row.dataEpoch)
	if err != nil {
		return err
	}
	if count >= maxSchedulesPerEpoch {
		return platform.NewError(platform.CodeValidation, false, "定时数量已达每站点上限")
	}
	refJSON, err := json.Marshal(row.inputRef)
	if err != nil {
		return platform.NewError(platform.CodeValidation, false, "定时输入不可编码")
	}
	_, err = work.tx.ExecContext(ctx, `INSERT INTO schedules (
		schedule_id, owner, cron, timezone, revision, enabled, next_fire_at, last_tick, job_type, input_ref, data_epoch
	) VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?)`,
		row.id, row.owner, row.cron, row.timezone, row.revision, boolInt(row.enabled), row.nextAt.UTC().Format(time.RFC3339Nano),
		row.jobType, string(refJSON), row.dataEpoch)
	if err != nil {
		return err
	}
	return enqueueScheduleChanged(work, row)
}

func lookupSchedule(ctx context.Context, op OperationContext, epoch platform.Identifier, id string) (scheduleRow, bool, error) {
	work, err := beginLeaseOperation(ctx, op, epoch)
	if err != nil {
		return scheduleRow{}, false, err
	}
	defer work.close()
	return readSchedule(ctx, work.tx, id)
}

func readSchedule(ctx context.Context, tx *sql.Tx, id string) (scheduleRow, bool, error) {
	row := tx.QueryRowContext(ctx, `SELECT schedule_id, owner, job_type, input_ref, cron, timezone, revision, enabled, next_fire_at, data_epoch
		FROM schedules WHERE schedule_id=?`, id)
	item, err := scanSchedule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return scheduleRow{}, false, nil
	}
	if err != nil {
		return scheduleRow{}, false, err
	}
	return item, true, nil
}

func scanSchedule(row interface{ Scan(...any) error }) (scheduleRow, error) {
	var item scheduleRow
	var rawRef, nextText string
	var enabled int
	err := row.Scan(&item.id, &item.owner, &item.jobType, &rawRef, &item.cron, &item.timezone, &item.revision, &enabled, &nextText, &item.dataEpoch)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return scheduleRow{}, err
		}
		return scheduleRow{}, mapQueueDBError(err)
	}
	item.enabled = enabled != 0
	item.nextAt, err = time.Parse(time.RFC3339Nano, nextText)
	if err != nil {
		return scheduleRow{}, platform.NewError(platform.CodeIntegrityError, false, "定时触发时刻损坏")
	}
	if rawRef == "" || json.Unmarshal([]byte(rawRef), &item.inputRef) != nil {
		return scheduleRow{}, platform.NewError(platform.CodeIntegrityError, false, "定时输入引用损坏")
	}
	return item, nil
}

func countSchedules(ctx context.Context, tx *sql.Tx, epoch string) (int, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM schedules WHERE data_epoch=?`, epoch).Scan(&count); err != nil {
		return 0, mapQueueDBError(err)
	}
	return count, nil
}

func authorizeScheduleAdmin(ctx context.Context, op OperationContext, resourceID string) error {
	if op.Actor.Kind != "admin" || op.Actor.SessionID == "" || op.Actor.SiteID == "" {
		return platform.NewError(platform.CodeForbidden, false, "定时管理需要后台管理员")
	}
	store := currentStore()
	if store == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	store.mu.RLock()
	client := store.identity
	closed := store.closed
	store.mu.RUnlock()
	if closed || client == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "定时管理授权尚未装配")
	}
	if resourceID == "" {
		resourceID = "schedules"
	}
	return authorizePermission(ctx, op, client, schedulePermission, resourceID)
}

func readScheduleReceipt(ctx context.Context, work *leaseOperation, op OperationContext, requestHash platform.Hash) ([]byte, error) {
	var hash string
	var result []byte
	err := work.tx.QueryRowContext(ctx, `SELECT request_hash, result_json FROM schedule_operations
		WHERE caller=? AND data_epoch=? AND operation_id=?`, op.CallerService, op.DataEpoch, op.OperationID).Scan(&hash, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapQueueDBError(err)
	}
	if hash != string(requestHash) {
		return nil, platform.NewError(platform.CodeConflict, false, "定时操作键已绑定不同输入")
	}
	return result, nil
}

func saveScheduleReceipt(ctx context.Context, work *leaseOperation, op OperationContext, requestHash platform.Hash, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "定时回执编码失败")
	}
	_, err = work.tx.ExecContext(ctx, `INSERT INTO schedule_operations (
		caller, data_epoch, operation_id, request_hash, result_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?)`, op.CallerService, op.DataEpoch, op.OperationID, string(requestHash), encoded, work.now.Format(time.RFC3339Nano))
	return mapQueueDBError(err)
}

func enqueueScheduleChanged(work *leaseOperation, row scheduleRow) error {
	payload, err := json.Marshal(scheduleChangedPayload{
		ScheduleID: row.id, Owner: row.owner, Type: row.jobType, Enabled: row.enabled,
		Revision: row.revision, NextAt: row.nextAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "定时事件编码失败")
	}
	return outbox.EnqueueEvent(work.tx, platform.EventEnvelope{
		EventID: platform.NewIdentifier(), EventType: scheduleEventType, EventVersion: scheduleEventVersion,
		Producer: "jobs", SubjectID: row.id, Revision: row.revision, OccurredAt: work.now.Format(time.RFC3339Nano),
		DataEpoch: row.dataEpoch, Payload: payload,
	})
}

func scheduleHash(value any) (platform.Hash, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", platform.NewError(platform.CodeValidation, false, "定时输入不可编码")
	}
	return platform.HashBytes(raw), nil
}

func scheduleCreateResult(row scheduleRow) (CreateScheduleResult, error) {
	revision, err := scheduleRevision(row.revision)
	if err != nil {
		return CreateScheduleResult{}, err
	}
	return CreateScheduleResult{ScheduleId: row.id, Revision: revision}, nil
}

func scheduleRevision(value int64) (gen.Revision, error) {
	if value < 0 || value > int64(math.MaxInt) {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "定时修订损坏")
	}
	return gen.Revision(value), nil
}

func catalogSchedule(owner, jobType string) (string, bool) {
	for _, entry := range gen.JobTypes {
		if entry.Owner == owner && entry.Type == jobType && entry.Schedule != "" {
			return entry.Schedule, true
		}
	}
	return "", false
}

func knownJobType(owner, jobType string) bool {
	registered, ok := registeredJobTypes[owner]
	return ok && containsJobType(registered, jobType)
}

func systemScheduleID(owner, jobType string) (string, error) {
	id := "system:" + owner + ":" + jobType
	if !platform.ValidIdentifier(id) {
		return "", platform.NewError(platform.CodeIntegrityError, false, "系统定时标识无法编码")
	}
	return id, nil
}

func scheduleConflict() error {
	return platform.NewError(platform.CodeConflict, false, "定时修订不匹配或记录不存在")
}

func mapScheduleWrite(err error) error {
	if isUniqueConstraint(err) {
		return platform.NewError(platform.CodeConflict, false, "定时标识已被占用")
	}
	var problem *platform.Error
	if errors.As(err, &problem) {
		return err
	}
	return mapQueueDBError(err)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
