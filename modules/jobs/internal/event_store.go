package internal

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/jobs/internal/gen"
)

type PublishEventInput struct{ Event platform.EventEnvelope }
type PublishEventResult = gen.PublishEventResult

type eventRecord struct {
	envelope platform.EventEnvelope
	seq      int64
	hash     string
}

// PublishEvent 只在完整信封、全部consumer投递与回执durable后确认生产方outbox。
func PublishEvent(ctx context.Context, op OperationContext, in PublishEventInput) (PublishEventResult, error) {
	if _, err := ValidateEnvelope(ctx, op, ValidateEnvelopeInput{Event: in.Event}); err != nil {
		return PublishEventResult{}, err
	}
	work, err := beginLeaseOperation(ctx, op, in.Event.DataEpoch)
	if err != nil {
		return PublishEventResult{}, err
	}
	defer work.close()
	encoded, hash := encodeEvent(in.Event)
	var result PublishEventResult
	found, err := work.eventReceipt(ctx, op, "publish", encoded, &result)
	if err != nil {
		return PublishEventResult{}, err
	}
	if found {
		return result, nil
	}
	prior, err := work.readEventRecord(ctx, in.Event.EventID)
	if err == nil {
		if prior.hash != hash {
			return PublishEventResult{}, platform.NewError(platform.CodeConflict, false, "事件ID已绑定不同信封")
		}
		result = PublishEventResult{Accepted: true, EventId: in.Event.EventID}
	} else {
		var problem *platform.Error
		if !errors.As(err, &problem) || problem.Code != platform.CodeNotFound {
			return PublishEventResult{}, err
		}
		if err := work.requireEventWrite(true); err != nil {
			return PublishEventResult{}, err
		}
		var seq int64
		if err := work.tx.QueryRowContext(ctx, `UPDATE event_sequence SET last_seq=last_seq+1 WHERE singleton=1 RETURNING last_seq`).Scan(&seq); err != nil {
			return PublishEventResult{}, mapQueueDBError(err)
		}
		if _, err := work.tx.ExecContext(ctx, `INSERT INTO events(event_id,event_seq,envelope_json,source_revision,data_epoch,producer,event_type,event_version,subject_id,envelope_hash,site_id)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`, in.Event.EventID, seq, encoded, in.Event.Revision, in.Event.DataEpoch,
			in.Event.Producer, in.Event.EventType, in.Event.EventVersion, in.Event.SubjectID, hash, in.Event.SiteID); err != nil {
			return PublishEventResult{}, mapQueueDBError(err)
		}
		definition, _ := findEventDefinition(in.Event.EventType, in.Event.EventVersion)
		for _, consumer := range definition.Consumers {
			if _, err := work.tx.ExecContext(ctx, `INSERT INTO deliveries(event_id,consumer_id,state,attempt,retry_at,delivery_id)
				VALUES(?,?,'queued',0,?,?)`, in.Event.EventID, consumer, work.now.Format(time.RFC3339Nano), platform.NewIdentifier()); err != nil {
				return PublishEventResult{}, mapQueueDBError(err)
			}
		}
		result = PublishEventResult{Accepted: true, EventId: in.Event.EventID}
	}
	if err := work.requireEventWrite(true); err != nil {
		return PublishEventResult{}, err
	}
	if err := work.saveEventReceipt(ctx, op, "publish", encoded, result); err != nil {
		return PublishEventResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return PublishEventResult{}, mapQueueDBError(err)
	}
	return result, nil
}

func encodeEvent(event platform.EventEnvelope) ([]byte, platform.Hash) {
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(event.Payload))
	decoder.UseNumber()
	_ = decoder.Decode(&payload)
	event.Payload, _ = json.Marshal(payload)
	encoded, _ := json.Marshal(event)
	return encoded, platform.HashBytes(encoded)
}

func (work *leaseOperation) readEventRecord(ctx context.Context, eventID string) (eventRecord, error) {
	var record eventRecord
	var encoded []byte
	var epoch, producer, name, subject string
	var version int
	var revision int64
	err := work.tx.QueryRowContext(ctx, `SELECT event_seq,envelope_json,envelope_hash,data_epoch,producer,event_type,event_version,subject_id,source_revision FROM events WHERE event_id=?`, eventID).
		Scan(&record.seq, &encoded, &record.hash, &epoch, &producer, &name, &version, &subject, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return eventRecord{}, platform.NewError(platform.CodeNotFound, false, "事件不存在")
	}
	if err != nil {
		return eventRecord{}, mapQueueDBError(err)
	}
	if epoch != work.runtime.Runtime.DataEpoch {
		return eventRecord{}, epochMismatch()
	}
	if err := json.Unmarshal(encoded, &record.envelope); err != nil {
		return eventRecord{}, platform.NewError(platform.CodeIntegrityError, false, "持久事件信封损坏")
	}
	event := record.envelope
	if event.EventID != eventID || event.DataEpoch != epoch || event.Producer != producer || event.EventType != name || event.EventVersion != version || event.SubjectID != subject || event.Revision != revision {
		return eventRecord{}, platform.NewError(platform.CodeIntegrityError, false, "事件信封与索引不一致")
	}
	if record.hash == "" {
		definition, err := findEventDefinition(name, version)
		if err != nil {
			return eventRecord{}, platform.NewError(platform.CodeIntegrityError, false, "旧事件规格未登记")
		}
		if definition.Producer != event.Producer || !platform.ValidIdentifier(event.EventID) || !platform.ValidIdentifier(event.SubjectID) || !platform.ValidIdentifier(event.DataEpoch) || event.Revision < 0 {
			return eventRecord{}, platform.NewError(platform.CodeIntegrityError, false, "旧事件生产方或信封标识不满足登记约束")
		}
		if _, err := platform.ParseTimestamp(event.OccurredAt); err != nil {
			return eventRecord{}, platform.NewError(platform.CodeIntegrityError, false, "旧事件发生时间不合法")
		}
		if err := validateEventPayload(definition, event.Payload); err != nil {
			return eventRecord{}, platform.NewError(platform.CodeIntegrityError, false, "旧事件载荷不满足当前版本规格")
		}
		_, record.hash = encodeEvent(event)
	} else if platform.HashBytes(encoded) != record.hash {
		return eventRecord{}, platform.NewError(platform.CodeIntegrityError, false, "事件信封哈希不一致")
	}
	return record, nil
}

func (work *leaseOperation) requireEventWrite(draining bool) error {
	return nil
}

func (work *leaseOperation) eventReceipt(ctx context.Context, op OperationContext, kind string, request []byte, result any) (bool, error) {
	var hash string
	var encoded []byte
	err := work.tx.QueryRowContext(ctx, `SELECT request_hash,result_json FROM event_operations WHERE caller=? AND data_epoch=? AND operation_kind=? AND operation_id=?`, op.CallerService, op.DataEpoch, kind, op.OperationID).Scan(&hash, &encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, mapQueueDBError(err)
	}
	if hash != platform.HashBytes(request) {
		return false, platform.NewError(platform.CodeConflict, false, "事件操作键已绑定不同输入")
	}
	if err := json.Unmarshal(encoded, result); err != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "事件操作回执损坏")
	}
	return true, nil
}

func (work *leaseOperation) saveEventReceipt(ctx context.Context, op OperationContext, kind string, request []byte, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "事件操作回执编码失败")
	}
	_, err = work.tx.ExecContext(ctx, `INSERT INTO event_operations(caller,data_epoch,operation_kind,operation_id,request_hash,result_json,created_at) VALUES(?,?,?,?,?,?,?)`,
		op.CallerService, op.DataEpoch, kind, op.OperationID, platform.HashBytes(request), encoded, work.now.Format(time.RFC3339Nano))
	return mapQueueDBError(err)
}

// EventPublisher 由生成的self JobsClient经platform适配，不补无关JobsClient假方法。
type EventPublisher = platform.EventPublisher

// RelayOutbox 只装配本域库与运行权威，领取、重试和隔离由platform统一执行。
func RelayOutbox(ctx context.Context, op OperationContext, client EventPublisher) error {
	if op.CallerService != "jobs" {
		return platform.NewError(platform.CodeForbidden, false, "仅jobs生命周期可以启动本域事件转发")
	}
	if client == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "jobs自指事件发布客户端尚未装配")
	}
	store := currentStore()
	if store == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	store.mu.RLock()
	if store.closed || store.leaseRuntime == nil || store.backgroundAdmit == nil || store.backgroundEventScope == nil {
		store.mu.RUnlock()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "任务后台业务scope尚未装配")
	}
	db, provider, admit, bindEventScope := store.db, store.leaseRuntime, store.backgroundAdmit, store.backgroundEventScope
	store.mu.RUnlock()
	return outbox.RunRelay(platform.WithOperationContext(ctx, op), outbox.NewStore(db), client, platform.RelayConfig{
		BatchLimit: 100, PollMillis: 1000, AdmitBatch: admit, BindEventScope: bindEventScope, Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			view, err := provider(ctx)
			return view.Runtime, err
		},
		WriteWindow: func(ctx context.Context) (time.Time, error) {
			view, err := provider(ctx)
			if err != nil {
				return time.Time{}, err
			}
			_ = view
			return time.Time{}, nil
		},
	})
}
