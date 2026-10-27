package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
)

const (
	// maxAttempts 是一条投递最多被读取的次数，超过后进死信。
	maxAttempts = 10
	// pollInterval 是没有到期投递时再看一次的间隔。
	pollInterval = 400 * time.Millisecond
)

// EventKey 是订阅登记里的事件标识：名称@版本。
func EventKey(eventType string, version int) string { return eventType + "@" + itoa(version) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

const insertEventSQL = `INSERT INTO queue.events
	(event_id, event_type, event_version, producer, subject_id, revision, occurred_at, data_epoch, site_id, payload, payload_hash)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?, ''), COALESCE(current_setting('nw.site', true), '')), ?, ?)
ON CONFLICT (event_id) DO NOTHING`

// EnqueueEvent 是生产方的入口：校验信封后在业务事务内写进队列。载荷必须符合生产方 events.json 登记的 schema，由生产方装配侧保证。
func EnqueueEvent(tx platform.Tx, envelope platform.EventEnvelope) error {
	if err := validateEnvelope(envelope); err != nil {
		return err
	}
	return Publish(context.Background(), tx, envelope)
}

// validateEnvelope 校验信封字段完整且载荷为合法 JSON。
func validateEnvelope(envelope platform.EventEnvelope) error {
	switch {
	case !platform.ValidIdentifier(envelope.EventID),
		envelope.EventType == "",
		envelope.EventVersion < 1,
		envelope.Producer == "",
		!platform.ValidIdentifier(envelope.SubjectID),
		envelope.Revision < 0,
		envelope.DataEpoch == "":
		return platform.NewError(platform.CodeValidation, false, "事件信封不完整")
	}
	if _, err := platform.ParseTimestamp(envelope.OccurredAt); err != nil {
		return platform.NewError(platform.CodeValidation, false, "事件发生时间不合法")
	}
	if !json.Valid(envelope.Payload) {
		return platform.NewError(platform.CodeValidation, false, "事件载荷不是合法JSON")
	}
	return nil
}

// Publish 在生产方的业务事务内把事件写进队列，并按订阅者逐个生成待投递行；事务回滚则事件不存在。
// 同 eventId 同内容幂等成功，内容不同返回 CONFLICT。tx 是组件自己的数据库事务（经方言翻译层）。
func Publish(ctx context.Context, tx platform.Tx, envelope platform.EventEnvelope) error {
	hash := platform.HashBytes(envelope.Payload)
	result, err := tx.ExecContext(ctx, insertEventSQL,
		envelope.EventID, envelope.EventType, envelope.EventVersion, envelope.Producer, envelope.SubjectID,
		envelope.Revision, envelope.OccurredAt, envelope.DataEpoch, string(envelope.SiteID), []byte(envelope.Payload), hash)
	if err != nil {
		return platform.NewError(platform.CodeInternal, true, "写入事件队列失败")
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return platform.NewError(platform.CodeInternal, true, "读取事件写入结果失败")
	}
	if inserted == 0 {
		var existing platform.Hash
		if err := tx.QueryRowContext(ctx, `SELECT payload_hash FROM queue.events WHERE event_id = ?`, envelope.EventID).Scan(&existing); err != nil {
			return platform.NewError(platform.CodeInternal, true, "读取已有事件失败")
		}
		if existing != hash {
			return platform.NewError(platform.CodeConflict, false, "同eventId的事件已存在且内容不同")
		}
		return nil
	}
	// 给此刻登记了订阅的每个消费者各生成一条待投递行；之后才登记的消费者由 Register 补发近期事件。
	if _, err := tx.ExecContext(ctx, `INSERT INTO queue.deliveries (event_id, consumer)
		SELECT ?, consumer FROM queue.subscriptions WHERE event_key = ? ON CONFLICT DO NOTHING`,
		envelope.EventID, EventKey(envelope.EventType, envelope.EventVersion)); err != nil {
		return platform.NewError(platform.CodeInternal, true, "写入事件投递失败")
	}
	return nil
}

// backfillWindow 是消费者首次登记时补发的事件范围。
const backfillWindow = 7 * 24 * time.Hour

// Register 登记消费者订阅的事件（名称@版本），并为近期已发布但还没有它的投递行的事件补上投递，
// 这样消费者比生产方晚启动也不会漏事件。订阅循环启动时调用。
func (q *Queue) Register(ctx context.Context, consumer string, eventKeys []string) error {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "登记事件订阅失败")
	}
	defer tx.Rollback()
	for _, key := range eventKeys {
		if _, err := tx.ExecContext(ctx, `INSERT INTO queue.subscriptions (consumer, event_key) VALUES ($1, $2) ON CONFLICT DO NOTHING`, consumer, key); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "登记事件订阅失败")
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO queue.deliveries (event_id, consumer)
		SELECT e.event_id, $1 FROM queue.events e
		WHERE e.event_type || '@' || e.event_version = ANY($2) AND e.created_at > $3
		ON CONFLICT DO NOTHING`, consumer, eventKeys, time.Now().Add(-backfillWindow)); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "补发事件投递失败")
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "登记事件订阅失败")
	}
	return nil
}

// Consumer 返回给订阅循环用的事件消费客户端。
func (q *Queue) Consumer() platform.EventConsumerClient { return consumerClient{q} }

type consumerClient struct{ q *Queue }

// Register 让订阅循环在启动时登记订阅（见 Queue.Register）。
func (c consumerClient) Register(ctx context.Context, consumer string, eventKeys []string) error {
	return c.q.Register(ctx, consumer, eventKeys)
}

func (c consumerClient) SubscribeEvents(_ context.Context, consumerID, _ string) (platform.EventStream, error) {
	return &eventStream{q: c.q, consumer: consumerID}, nil
}

func (c consumerClient) AcknowledgeEvent(ctx context.Context, consumerID string, eventID platform.Identifier) error {
	result, err := c.q.db.ExecContext(ctx, `UPDATE queue.deliveries SET state = 'done', updated_at = now() WHERE event_id = $1 AND consumer = $2`, eventID, consumerID)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "确认事件失败")
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return platform.NewError(platform.CodeNotFound, false, "事件投递不存在")
	}
	return nil
}

type eventStream struct {
	q        *Queue
	consumer string
	last     string
}

func (s *eventStream) LastID() string { return s.last }
func (s *eventStream) Close() error   { return nil }

// Next 读取该消费者下一条到期的投递；读到时把它的可见时间推到未来（租约），到点没确认就会再被读到。
func (s *eventStream) Next(ctx context.Context) (platform.EventEnvelope, error) {
	for {
		envelope, found, err := s.q.claimEvent(ctx, s.consumer)
		if err != nil {
			return platform.EventEnvelope{}, err
		}
		if found {
			s.last = envelope.EventID
			return envelope, nil
		}
		select {
		case <-ctx.Done():
			return platform.EventEnvelope{}, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

const deadLetterSQL = `UPDATE queue.deliveries SET state = 'dead-letter', last_error = 'attempts exceeded', updated_at = now()
WHERE consumer = $1 AND state = 'queued' AND visible_at <= now() AND attempts >= $2`

// claimSQL 按事件顺序取最早到期的一条；租约 45 秒起、每读一次翻倍、最长 5 分钟，同时就是失败后的退避。
const claimSQL = `UPDATE queue.deliveries d
SET attempts = d.attempts + 1, visible_at = now() + make_interval(secs => LEAST(300, 45 * power(2, d.attempts))), updated_at = now()
FROM (
	SELECT x.event_id FROM queue.deliveries x JOIN queue.events e ON e.event_id = x.event_id
	WHERE x.consumer = $1 AND x.state = 'queued' AND x.visible_at <= now()
	ORDER BY e.seq LIMIT 1 FOR UPDATE OF x SKIP LOCKED
) due
WHERE d.event_id = due.event_id AND d.consumer = $1
RETURNING d.event_id`

const loadEventSQL = `SELECT event_type, event_version, producer, subject_id, revision, occurred_at, data_epoch, site_id, payload FROM queue.events WHERE event_id = $1`

func (q *Queue) claimEvent(ctx context.Context, consumer string) (platform.EventEnvelope, bool, error) {
	if _, err := q.db.ExecContext(ctx, deadLetterSQL, consumer, maxAttempts); err != nil {
		return platform.EventEnvelope{}, false, platform.NewError(platform.CodeDependencyUnavailable, true, "整理死信失败")
	}
	var eventID string
	err := q.db.QueryRowContext(ctx, claimSQL, consumer).Scan(&eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.EventEnvelope{}, false, nil
	}
	if err != nil {
		return platform.EventEnvelope{}, false, platform.NewError(platform.CodeDependencyUnavailable, true, "读取事件失败")
	}
	envelope := platform.EventEnvelope{EventID: eventID}
	var payload []byte
	err = q.db.QueryRowContext(ctx, loadEventSQL, eventID).Scan(&envelope.EventType, &envelope.EventVersion, &envelope.Producer,
		&envelope.SubjectID, &envelope.Revision, &envelope.OccurredAt, &envelope.DataEpoch, &envelope.SiteID, &payload)
	if err != nil {
		return platform.EventEnvelope{}, false, platform.NewError(platform.CodeDependencyUnavailable, true, "读取事件内容失败")
	}
	envelope.Payload = payload
	return envelope, true, nil
}

// DeadLetter 是一条死信的元数据（不含载荷）。
type DeadLetter struct {
	DeliveryID   string
	EventID      string
	EventType    string
	EventVersion int
	Producer     string
	Consumer     string
	Attempts     int
	LastError    string
	SiteID       string
}

// ListDeadLetters 列出某站点的死信，按投递编号排序；producer 为空表示不限。
func (q *Queue) ListDeadLetters(ctx context.Context, siteID, producer, afterDeliveryID string, limit int) ([]DeadLetter, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT d.delivery_id, d.event_id, e.event_type, e.event_version, e.producer, d.consumer, d.attempts, d.last_error
		FROM queue.deliveries d JOIN queue.events e ON e.event_id = d.event_id
		WHERE d.state = 'dead-letter' AND e.site_id = $1 AND ($2 = '' OR e.producer = $2) AND d.delivery_id > $3
		ORDER BY d.delivery_id LIMIT $4`, siteID, producer, afterDeliveryID, limit)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "读取死信失败")
	}
	defer rows.Close()
	var out []DeadLetter
	for rows.Next() {
		d := DeadLetter{SiteID: siteID}
		if err := rows.Scan(&d.DeliveryID, &d.EventID, &d.EventType, &d.EventVersion, &d.Producer, &d.Consumer, &d.Attempts, &d.LastError); err != nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "读取死信失败")
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeadLetterByID 返回一条死信（含所属站点）；不是死信返回 NOT_FOUND。
func (q *Queue) DeadLetterByID(ctx context.Context, deliveryID string) (DeadLetter, error) {
	var d DeadLetter
	d.DeliveryID = deliveryID
	err := q.db.QueryRowContext(ctx, `SELECT d.event_id, e.event_type, e.event_version, e.producer, d.consumer, d.attempts, d.last_error, e.site_id
		FROM queue.deliveries d JOIN queue.events e ON e.event_id = d.event_id
		WHERE d.delivery_id = $1 AND d.state = 'dead-letter'`, deliveryID).Scan(&d.EventID, &d.EventType, &d.EventVersion, &d.Producer, &d.Consumer, &d.Attempts, &d.LastError, &d.SiteID)
	if errors.Is(err, sql.ErrNoRows) {
		return d, platform.NewError(platform.CodeNotFound, false, "死信不存在")
	}
	if err != nil {
		return d, platform.NewError(platform.CodeDependencyUnavailable, true, "读取死信失败")
	}
	return d, nil
}

// Replay 把一条死信重新排队（次数清零、立即可见）。
func (q *Queue) Replay(ctx context.Context, deliveryID string) error {
	result, err := q.db.ExecContext(ctx, `UPDATE queue.deliveries SET state = 'queued', attempts = 0, visible_at = now(), last_error = '', updated_at = now()
		WHERE delivery_id = $1 AND state = 'dead-letter'`, deliveryID)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "重放死信失败")
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return platform.NewError(platform.CodeNotFound, false, "死信不存在")
	}
	return nil
}

// PurgeEvents 删除早于 before 且已全部处理完（没有待投递或死信行）的事件，返回删除条数。
func (q *Queue) PurgeEvents(ctx context.Context, before time.Time) (int64, error) {
	result, err := q.db.ExecContext(ctx, `DELETE FROM queue.events e WHERE e.created_at < $1
		AND NOT EXISTS (SELECT 1 FROM queue.deliveries d WHERE d.event_id = e.event_id AND d.state <> 'done')`, before)
	if err != nil {
		return 0, platform.NewError(platform.CodeDependencyUnavailable, true, "清理事件失败")
	}
	n, _ := result.RowsAffected()
	return n, nil
}
