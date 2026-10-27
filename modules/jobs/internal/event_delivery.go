package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/jobs/internal/gen"
)

type AcknowledgeEventInput struct {
	EventID    string              `json:"eventId"`
	ConsumerID string              `json:"consumerId"`
	DataEpoch  platform.Identifier `json:"dataEpoch"`
}
type AcknowledgeEventResult = gen.AcknowledgeEventResult

type ReplayDeliveryInput struct {
	DeliveryID string `json:"deliveryId"`
}
type ReplayDeliveryResult = gen.ReplayDeliveryResult

type RequeueExpiredDeliveriesInput struct {
	Limit     int
	DataEpoch platform.Identifier
}
type RequeueExpiredDeliveriesResult struct {
	Requeued     int64
	DeadLettered int64
}

type deliveryRecord struct {
	id             string
	eventID        string
	consumer       string
	epoch          string
	producer       string
	name           string
	version        int
	state          string
	attempt        int64
	revision       int64
	seq            int64
	leaseExpiresAt sql.NullString
}

func (work *leaseOperation) readDeliveryRecord(ctx context.Context, deliveryID string, eventID string, consumer string) (deliveryRecord, error) {
	var delivery deliveryRecord
	err := work.tx.QueryRowContext(ctx, `SELECT d.delivery_id,d.event_id,d.consumer_id,d.state,d.attempt,d.revision,d.lease_expires_at,
		e.data_epoch,e.producer,e.event_type,e.event_version,e.event_seq FROM deliveries d JOIN events e ON e.event_id=d.event_id
		WHERE (?<>'' AND d.delivery_id=?) OR (?='' AND d.event_id=? AND d.consumer_id=?)`, deliveryID, deliveryID, deliveryID, eventID, consumer).
		Scan(&delivery.id, &delivery.eventID, &delivery.consumer, &delivery.state, &delivery.attempt, &delivery.revision, &delivery.leaseExpiresAt,
			&delivery.epoch, &delivery.producer, &delivery.name, &delivery.version, &delivery.seq)
	if errors.Is(err, sql.ErrNoRows) {
		return deliveryRecord{}, platform.NewError(platform.CodeNotFound, false, "事件投递不存在")
	}
	if err != nil {
		return deliveryRecord{}, mapQueueDBError(err)
	}
	if delivery.epoch != work.runtime.Runtime.DataEpoch {
		return deliveryRecord{}, epochMismatch()
	}
	return delivery, nil
}

// AcknowledgeEvent 的确认只覆盖本consumer；不把传输游标当已提交业务水位。
func AcknowledgeEvent(ctx context.Context, op OperationContext, in AcknowledgeEventInput) (AcknowledgeEventResult, error) {
	if op.CallerService != in.ConsumerID {
		return AcknowledgeEventResult{}, platform.NewError(platform.CodeForbidden, false, "只能确认本服务的事件投递")
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return AcknowledgeEventResult{}, err
	}
	defer work.close()
	delivery, err := work.readDeliveryRecord(ctx, "", in.EventID, in.ConsumerID)
	if err != nil {
		return AcknowledgeEventResult{}, err
	}
	if !consumerHasEvent(in.ConsumerID, delivery.name, delivery.version) {
		return AcknowledgeEventResult{}, platform.NewError(platform.CodeForbidden, false, "消费者未登记该事件版本")
	}
	request, _ := json.Marshal(in)
	var result AcknowledgeEventResult
	found, err := work.eventReceipt(ctx, op, "ack", request, &result)
	if err != nil {
		return AcknowledgeEventResult{}, err
	}
	if found {
		return result, nil
	}
	if delivery.state != "acknowledged" {
		if (delivery.state != "delivering" && delivery.state != "queued") || !delivery.leaseExpiresAt.Valid {
			return AcknowledgeEventResult{}, platform.NewError(platform.CodeStaleResult, false, "投递尚未发出或已进入死信")
		}
		if err := work.requireEventWrite(true); err != nil {
			return AcknowledgeEventResult{}, err
		}
		updated, err := work.tx.ExecContext(ctx, `UPDATE deliveries SET state='acknowledged',revision=revision+1,last_error_code=''
			WHERE delivery_id=? AND state=? AND revision=?`, delivery.id, delivery.state, delivery.revision)
		if err != nil {
			return AcknowledgeEventResult{}, mapQueueDBError(err)
		}
		if n, err := updated.RowsAffected(); err != nil || n != 1 {
			return AcknowledgeEventResult{}, platform.NewError(platform.CodeConflict, false, "投递状态已变化")
		}
		if err := work.advanceConsumerOffset(ctx, in.ConsumerID, in.DataEpoch); err != nil {
			return AcknowledgeEventResult{}, err
		}
	}
	if err := work.requireEventWrite(true); err != nil {
		return AcknowledgeEventResult{}, err
	}
	result = AcknowledgeEventResult{Acknowledged: true}
	if err := work.saveEventReceipt(ctx, op, "ack", request, result); err != nil {
		return AcknowledgeEventResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return AcknowledgeEventResult{}, mapQueueDBError(err)
	}
	return result, nil
}

func (work *leaseOperation) advanceConsumerOffset(ctx context.Context, consumer string, epoch platform.Identifier) error {
	var watermark int64
	err := work.tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT MIN(e.event_seq)-1 FROM deliveries d JOIN events e ON e.event_id=d.event_id
		WHERE d.consumer_id=? AND e.data_epoch=? AND d.state<>'acknowledged'),(SELECT last_seq FROM event_sequence WHERE singleton=1))`, consumer, epoch).Scan(&watermark)
	if err != nil {
		return mapQueueDBError(err)
	}
	_, err = work.tx.ExecContext(ctx, `INSERT INTO consumer_offsets(consumer_id,data_epoch,acknowledged_seq) VALUES(?,?,?)
		ON CONFLICT(consumer_id,data_epoch) DO UPDATE SET acknowledged_seq=MAX(consumer_offsets.acknowledged_seq,excluded.acknowledged_seq)`, consumer, epoch, watermark)
	return mapQueueDBError(err)
}

// RequeueExpiredDeliveries 不阻塞其他consumer；同一投递到期只计数一次。
func RequeueExpiredDeliveries(ctx context.Context, op OperationContext, in RequeueExpiredDeliveriesInput) (RequeueExpiredDeliveriesResult, error) {
	if op.CallerService != "jobs" {
		return RequeueExpiredDeliveriesResult{}, platform.NewError(platform.CodeForbidden, false, "仅jobs后台可以回收事件确认租约")
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return RequeueExpiredDeliveriesResult{}, err
	}
	defer work.close()
	rows, err := work.tx.QueryContext(ctx, `SELECT d.delivery_id,d.attempt,d.revision,d.lease_expires_at FROM deliveries d JOIN events e ON e.event_id=d.event_id
		WHERE e.data_epoch=? AND d.state='delivering' AND (d.lease_expires_at IS NULL OR julianday(d.lease_expires_at)<=julianday(?))
		ORDER BY julianday(d.lease_expires_at),e.event_seq,d.consumer_id LIMIT ?`, in.DataEpoch, work.now.Format(time.RFC3339Nano), in.Limit)
	if err != nil {
		return RequeueExpiredDeliveriesResult{}, mapQueueDBError(err)
	}
	expired := []deliveryRecord{}
	for rows.Next() {
		var delivery deliveryRecord
		if err := rows.Scan(&delivery.id, &delivery.attempt, &delivery.revision, &delivery.leaseExpiresAt); err != nil {
			_ = rows.Close()
			return RequeueExpiredDeliveriesResult{}, mapQueueDBError(err)
		}
		if !delivery.leaseExpiresAt.Valid {
			_ = rows.Close()
			return RequeueExpiredDeliveriesResult{}, platform.NewError(platform.CodeIntegrityError, false, "活动事件投递缺少确认租约")
		}
		expires, err := time.Parse(time.RFC3339Nano, delivery.leaseExpiresAt.String)
		if err != nil {
			_ = rows.Close()
			return RequeueExpiredDeliveriesResult{}, platform.NewError(platform.CodeIntegrityError, false, "事件确认租约截止损坏")
		}
		if !expires.After(work.now) {
			expired = append(expired, delivery)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return RequeueExpiredDeliveriesResult{}, mapQueueDBError(err)
	}
	_ = rows.Close()
	result := RequeueExpiredDeliveriesResult{}
	for _, delivery := range expired {
		attempt := delivery.attempt + 1
		state := "queued"
		delay := min(time.Second<<uint(min(attempt-1, 9)), 300*time.Second)
		if attempt > 10 {
			state = "dead-letter"
			result.DeadLettered++
		} else {
			result.Requeued++
		}
		updated, err := work.tx.ExecContext(ctx, `UPDATE deliveries SET state=?,attempt=?,retry_at=?,last_error_code=?,revision=revision+1
			WHERE delivery_id=? AND state='delivering' AND attempt=? AND revision=? AND lease_expires_at=?`,
			state, attempt, work.now.Add(delay).Format(time.RFC3339Nano), platform.CodeDeadlineExceeded, delivery.id, delivery.attempt, delivery.revision, delivery.leaseExpiresAt.String)
		if err != nil {
			return RequeueExpiredDeliveriesResult{}, mapQueueDBError(err)
		}
		if n, err := updated.RowsAffected(); err != nil || n != 1 {
			return RequeueExpiredDeliveriesResult{}, platform.NewError(platform.CodeConflict, false, "事件确认租约已变化")
		}
	}
	if err := work.tx.Commit(); err != nil {
		return RequeueExpiredDeliveriesResult{}, mapQueueDBError(err)
	}
	return result, nil
}

func authorizeDeliveryManagement(ctx context.Context, op OperationContext, deliveryID string) (deliveryRecord, error) {
	work, err := beginLeaseOperation(ctx, op, op.DataEpoch)
	if err != nil {
		return deliveryRecord{}, err
	}
	delivery, err := work.readDeliveryRecord(ctx, deliveryID, "", "")
	client := work.store.identity
	var site string
	if err == nil {
		if scanErr := work.tx.QueryRowContext(ctx, `SELECT site_id FROM events WHERE event_id=?`, delivery.eventID).Scan(&site); scanErr != nil {
			err = mapQueueDBError(scanErr)
		}
	}
	work.close()
	if err != nil {
		return deliveryRecord{}, err
	}
	if client == nil {
		return deliveryRecord{}, platform.NewError(platform.CodeDependencyUnavailable, true, "事件重放授权尚未装配")
	}
	if site != op.Actor.SiteID {
		return deliveryRecord{}, platform.NewError(platform.CodeNotFound, false, "事件投递不存在")
	}
	definition, err := findEventDefinition(delivery.name, delivery.version)
	if err != nil || definition.Producer != delivery.producer || definition.ManagementPermission == "" {
		return deliveryRecord{}, platform.NewError(platform.CodeIntegrityError, false, "持久事件缺少登记生产方或管理权限")
	}
	if !consumerHasEvent(delivery.consumer, delivery.name, delivery.version) {
		return deliveryRecord{}, platform.NewError(platform.CodeForbidden, false, "消费者未登记该事件版本")
	}
	if err := authorizePermission(ctx, op, client, definition.ManagementPermission, delivery.id); err != nil {
		return deliveryRecord{}, err
	}
	return delivery, nil
}

// ReplayDelivery 只重排原死信，不能覆盖已确认或正在处理的投递。
func ReplayDelivery(ctx context.Context, op OperationContext, in ReplayDeliveryInput) (ReplayDeliveryResult, error) {
	authorized, err := authorizeDeliveryManagement(ctx, op, in.DeliveryID)
	if err != nil {
		return ReplayDeliveryResult{}, err
	}
	work, err := beginLeaseOperation(ctx, op, op.DataEpoch)
	if err != nil {
		return ReplayDeliveryResult{}, err
	}
	defer work.close()
	delivery, err := work.readDeliveryRecord(ctx, in.DeliveryID, "", "")
	if err != nil {
		return ReplayDeliveryResult{}, err
	}
	if delivery.eventID != authorized.eventID || delivery.consumer != authorized.consumer || delivery.epoch != authorized.epoch || delivery.producer != authorized.producer || delivery.name != authorized.name || delivery.version != authorized.version {
		return ReplayDeliveryResult{}, platform.NewError(platform.CodeConflict, false, "投递授权作用域已变化")
	}
	request, _ := json.Marshal(struct {
		DeliveryID string `json:"deliveryId"`
		Actor      string `json:"actor"`
		Site       string `json:"site"`
	}{in.DeliveryID, op.Actor.SubjectID, op.Actor.SiteID})
	var result ReplayDeliveryResult
	found, err := work.eventReceipt(ctx, op, "replay", request, &result)
	if err != nil {
		return ReplayDeliveryResult{}, err
	}
	if found {
		return result, nil
	}
	if delivery.state != "dead-letter" {
		return ReplayDeliveryResult{}, platform.NewError(platform.CodeConflict, false, "只能重放死信投递")
	}
	if err := work.requireEventWrite(false); err != nil {
		return ReplayDeliveryResult{}, err
	}
	updated, err := work.tx.ExecContext(ctx, `UPDATE deliveries SET state='queued',attempt=0,retry_at=?,lease_expires_at=NULL,last_error_code='',revision=revision+1
		WHERE delivery_id=? AND state='dead-letter' AND revision=?`, work.now.Format(time.RFC3339Nano), delivery.id, delivery.revision)
	if err != nil {
		return ReplayDeliveryResult{}, mapQueueDBError(err)
	}
	if n, err := updated.RowsAffected(); err != nil || n != 1 {
		return ReplayDeliveryResult{}, platform.NewError(platform.CodeConflict, false, "死信投递已变化")
	}
	result = ReplayDeliveryResult{Queued: true}
	if err := work.saveEventReceipt(ctx, op, "replay", request, result); err != nil {
		return ReplayDeliveryResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return ReplayDeliveryResult{}, mapQueueDBError(err)
	}
	return result, nil
}
