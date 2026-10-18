// Package inbox 实现事件处理函数的去重与事件订阅循环。
//
// 处理函数在本地事务内调用ClaimInbox完成eventId去重与低修订拒绝，
// 提交后由订阅循环确认；未确认投递到期由jobs重投。
package inbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"nice-wiki/libs/platform"
)

// SchemaSQL inbox表结构；各服务启动时以platform为准建表。
const SchemaSQL = `CREATE TABLE IF NOT EXISTS nw_inbox (
	event_id      TEXT PRIMARY KEY,
	event_type    TEXT NOT NULL,
	event_version INTEGER NOT NULL,
	producer      TEXT NOT NULL,
	subject_id    TEXT NOT NULL,
	revision      INTEGER NOT NULL,
	payload_hash  TEXT NOT NULL,
	state         TEXT NOT NULL DEFAULT 'applied',
	processed_at  TEXT NOT NULL
)`
const schemaIndexSQL = `CREATE INDEX IF NOT EXISTS idx_nw_inbox_stream ON nw_inbox (producer, event_type, event_version, subject_id, state, revision)`

// EnsureSchema 创建inbox表与索引；幂等。
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, SchemaSQL); err != nil {
		return platform.NewError(platform.CodeInternal, true, "创建inbox表失败")
	}
	if _, err := db.ExecContext(ctx, schemaIndexSQL); err != nil {
		return platform.NewError(platform.CodeInternal, true, "创建inbox索引失败")
	}
	return nil
}

const lookupSQL = `SELECT payload_hash, state, event_type, event_version, producer, subject_id, revision FROM nw_inbox WHERE event_id = ?`
const maxRevisionSQL = `SELECT MAX(revision) FROM nw_inbox WHERE producer = ? AND event_type = ? AND event_version = ? AND subject_id = ? AND state = 'applied'`
const insertSQL = `INSERT INTO nw_inbox
	(event_id, event_type, event_version, producer, subject_id, revision, payload_hash, state, processed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 'applied', ?)`
const quarantineSQL = `UPDATE nw_inbox SET state = 'quarantined' WHERE event_id = ?`

// ClaimInbox 兼容旧事务接口。冲突隔离仅在调用方提交事务后持久；
// 新业务处理应在任何业务写入前调用ClaimInboxBeforeWrite。
func ClaimInbox(tx platform.Tx, envelope platform.EventEnvelope) (bool, error) {
	fresh, _, err := claimInbox(context.Background(), tx, envelope)
	return fresh, err
}

// ClaimInboxBeforeWrite 必须是事务中的首项写操作；commitClaim只能提交此隔离写入。
// 隔离成功先提交再返回原完整性错误，避免通常的错误回滚把隔离标记抹掉。
func ClaimInboxBeforeWrite(ctx context.Context, tx platform.Tx, envelope platform.EventEnvelope, commitClaim func() error) (bool, error) {
	if commitClaim == nil {
		return false, platform.NewError(platform.CodeValidation, false, "缺少事件隔离提交回调")
	}
	fresh, quarantined, err := claimInbox(ctx, tx, envelope)
	if quarantined {
		if commitErr := commitClaim(); commitErr != nil {
			return false, commitErr
		}
	}
	return fresh, err
}

func claimInbox(ctx context.Context, tx platform.Tx, envelope platform.EventEnvelope) (bool, bool, error) {
	if err := validateEnvelope(envelope); err != nil {
		return false, false, err
	}
	payloadHash := platform.HashBytes(envelope.Payload)
	var existingHash, state, eventType, producer, subject string
	var version, revision int64
	err := tx.QueryRowContext(ctx, lookupSQL, envelope.EventID).Scan(&existingHash, &state, &eventType, &version, &producer, &subject, &revision)
	switch {
	case err == nil:
		if state == "quarantined" {
			return false, false, platform.NewError(platform.CodeIntegrityError, false, "事件已隔离，不能确认或继续处理")
		}
		if state != "applied" {
			return false, false, platform.NewError(platform.CodeIntegrityError, false, "事件处理状态损坏")
		}
		if existingHash != payloadHash || eventType != envelope.EventType || version != int64(envelope.EventVersion) || producer != envelope.Producer || subject != envelope.SubjectID || revision != envelope.Revision {
			if _, err := tx.ExecContext(ctx, quarantineSQL, envelope.EventID); err != nil {
				return false, false, platform.NewError(platform.CodeInternal, true, "隔离inbox行失败")
			}
			return false, true, platform.NewError(platform.CodeIntegrityError, false, "同eventId的事件身份或载荷不一致")
		}
		return false, false, nil
	case !errors.Is(err, sql.ErrNoRows):
		return false, false, platform.NewError(platform.CodeInternal, true, "读取inbox失败")
	}
	var maxRevision sql.NullInt64
	if err := tx.QueryRowContext(ctx, maxRevisionSQL, envelope.Producer, envelope.EventType, envelope.EventVersion, envelope.SubjectID).Scan(&maxRevision); err != nil {
		return false, false, platform.NewError(platform.CodeInternal, true, "读取inbox修订失败")
	}
	if maxRevision.Valid && envelope.Revision < maxRevision.Int64 {
		return false, false, nil
	}
	if _, err := tx.ExecContext(ctx, insertSQL, envelope.EventID, envelope.EventType, envelope.EventVersion, envelope.Producer, envelope.SubjectID, envelope.Revision, payloadHash, platform.Now()); err != nil {
		// 并发冲突不能未经核对就确认；保留原事件，交由jobs下一次投递读取完整身份。
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return false, false, platform.NewError(platform.CodeConflict, true, "事件并发认领尚未确认")
		}
		return false, false, platform.NewError(platform.CodeInternal, true, "写入inbox失败")
	}
	return true, false, nil
}

// validateEnvelope 校验信封字段与载荷；载荷schema由消费方配置校验。
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
