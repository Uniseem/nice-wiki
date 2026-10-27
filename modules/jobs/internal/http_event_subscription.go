package internal

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"strconv"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/jobs/internal/gen"
)

type SubscribeEventsInput struct {
	ConsumerID  string
	LastEventID string
	Mode        string
	DataEpoch   platform.Identifier
}
type SubscribeEventsResult struct{ Stream platform.EventStream }

type eventSubscription struct {
	store    *Store
	op       OperationContext
	consumer string
	mu       sync.Mutex
	lastID   string
	closed   bool
	stop     chan struct{}
}

// SubscribeEvents 只建立登记服务的durable增量流；portal原payload广播保持关闭。
func SubscribeEvents(ctx context.Context, op OperationContext, in SubscribeEventsInput) (SubscribeEventsResult, error) {
	if in.Mode == "ephemeral" {
		return SubscribeEventsResult{}, platform.NewError(platform.CodeUnsupportedCapability, false, "浏览器事件安全投影尚未开放")
	}
	if in.Mode != "durable" || in.ConsumerID != op.CallerService || in.ConsumerID == "portal" {
		return SubscribeEventsResult{}, platform.NewError(platform.CodeForbidden, false, "只能订阅本服务登记的durable事件")
	}
	registered := false
	for _, definition := range gen.EventTypes {
		if consumerHasEvent(in.ConsumerID, definition.Name, definition.Version) {
			registered = true
			break
		}
	}
	if !registered {
		return SubscribeEventsResult{}, platform.NewError(platform.CodeForbidden, false, "服务未登记任何事件订阅")
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return SubscribeEventsResult{}, err
	}
	defer work.close()
	if in.LastEventID != "" {
		cursor, err := strconv.ParseInt(in.LastEventID, 10, 64)
		if err != nil || cursor < 0 {
			return SubscribeEventsResult{}, platform.NewError(platform.CodeValidation, false, "事件续传游标不合法")
		}
		var latest int64
		if err := work.tx.QueryRowContext(ctx, `SELECT last_seq FROM event_sequence WHERE singleton=1`).Scan(&latest); err != nil {
			return SubscribeEventsResult{}, mapQueueDBError(err)
		}
		if cursor > latest {
			return SubscribeEventsResult{}, platform.NewError(platform.CodeNotFound, false, "事件续传水位不存在")
		}
	}
	store := work.store
	store.eventMu.Lock()
	defer store.eventMu.Unlock()
	if store.eventStreams == nil {
		store.eventStreams = make(map[string]int)
	}
	if store.eventStreams[in.ConsumerID] >= 2 {
		return SubscribeEventsResult{}, platform.NewError(platform.CodeRateLimited, true, "该消费者事件连接已达上限")
	}
	store.eventStreams[in.ConsumerID]++
	return SubscribeEventsResult{Stream: &eventSubscription{store: store, op: op, consumer: in.ConsumerID, lastID: in.LastEventID, stop: make(chan struct{})}}, nil
}

// Next 每次只交付一个事件；没有可见行时不持数据库事务等待。
func (stream *eventSubscription) Next(ctx context.Context) (platform.EventEnvelope, error) {
	for {
		select {
		case <-stream.stop:
			return platform.EventEnvelope{}, io.EOF
		default:
		}
		event, seq, found, err := stream.nextDelivery(ctx)
		if err != nil {
			_ = stream.Close()
			return platform.EventEnvelope{}, err
		}
		if found {
			stream.mu.Lock()
			if stream.closed {
				stream.mu.Unlock()
				return platform.EventEnvelope{}, io.EOF
			}
			stream.lastID = strconv.FormatInt(seq, 10)
			stream.mu.Unlock()
			return event, nil
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = stream.Close()
			return platform.EventEnvelope{}, ctx.Err()
		case <-stream.stop:
			timer.Stop()
			return platform.EventEnvelope{}, io.EOF
		case <-timer.C:
		}
	}
}

func (stream *eventSubscription) LastID() string {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return stream.lastID
}

func (stream *eventSubscription) Close() error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed {
		return nil
	}
	stream.closed = true
	close(stream.stop)
	stream.store.eventMu.Lock()
	stream.store.eventStreams[stream.consumer]--
	stream.store.eventMu.Unlock()
	return nil
}

func (stream *eventSubscription) nextDelivery(ctx context.Context) (platform.EventEnvelope, int64, bool, error) {
	work, err := beginLeaseOperation(ctx, stream.op, stream.op.DataEpoch)
	if err != nil {
		return platform.EventEnvelope{}, 0, false, err
	}
	defer work.close()
	var id, eventID, retryAt string
	var revision int64
	err = work.tx.QueryRowContext(ctx, `SELECT d.delivery_id,d.event_id,d.revision,d.retry_at FROM deliveries d JOIN events e ON e.event_id=d.event_id
		WHERE d.consumer_id=? AND d.state='queued' AND e.data_epoch=? AND julianday(d.retry_at)<=julianday(?)
		AND NOT EXISTS(SELECT 1 FROM deliveries prior JOIN events previous ON previous.event_id=prior.event_id
			WHERE prior.consumer_id=d.consumer_id AND previous.data_epoch=e.data_epoch AND previous.producer=e.producer
			AND previous.subject_id=e.subject_id AND previous.event_seq<e.event_seq AND prior.state<>'acknowledged')
		ORDER BY e.event_seq LIMIT 1`, stream.consumer, stream.op.DataEpoch, work.now.Format(time.RFC3339Nano)).Scan(&id, &eventID, &revision, &retryAt)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.EventEnvelope{}, 0, false, nil
	}
	if err != nil {
		return platform.EventEnvelope{}, 0, false, mapQueueDBError(err)
	}
	visibleAt, err := time.Parse(time.RFC3339Nano, retryAt)
	if err != nil {
		return platform.EventEnvelope{}, 0, false, platform.NewError(platform.CodeIntegrityError, false, "事件可见时间损坏")
	}
	if visibleAt.After(work.now) {
		return platform.EventEnvelope{}, 0, false, nil
	}
	record, err := work.readEventRecord(ctx, eventID)
	if err != nil {
		return platform.EventEnvelope{}, 0, false, err
	}
	if !consumerHasEvent(stream.consumer, record.envelope.EventType, record.envelope.EventVersion) {
		return platform.EventEnvelope{}, 0, false, platform.NewError(platform.CodeForbidden, false, "持久投递不在消费者登记范围")
	}
	updated, err := work.tx.ExecContext(ctx, `UPDATE deliveries SET state='delivering',lease_expires_at=?,revision=revision+1
		WHERE delivery_id=? AND consumer_id=? AND state='queued' AND revision=?`, work.now.Add(30*time.Second).Format(time.RFC3339Nano), id, stream.consumer, revision)
	if err != nil {
		return platform.EventEnvelope{}, 0, false, mapQueueDBError(err)
	}
	if n, err := updated.RowsAffected(); err != nil || n != 1 {
		return platform.EventEnvelope{}, 0, false, platform.NewError(platform.CodeConflict, false, "事件投递已被其他连接领取")
	}
	if err := work.tx.Commit(); err != nil {
		return platform.EventEnvelope{}, 0, false, mapQueueDBError(err)
	}
	return record.envelope, record.seq, true, nil
}
