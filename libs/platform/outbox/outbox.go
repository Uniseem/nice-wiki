// Package outbox 在业务事务内登记outbox事件并提供转发循环。
//
// 事件只经jobs中转：生产方在业务事务内写outbox，事务提交后由
// RunRelay转交jobs:publishEvent，确认后才标记relayed。
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
)

// SchemaSQL outbox表结构；各服务在启动时以platform为准建表，
// 不在SQL迁移中复制定义，避免与库行为漂移。
const SchemaSQL = `CREATE TABLE IF NOT EXISTS nw_outbox (
	event_id        TEXT PRIMARY KEY,
	event_type      TEXT NOT NULL,
	event_version   INTEGER NOT NULL,
	producer        TEXT NOT NULL,
	subject_id      TEXT NOT NULL,
	revision        INTEGER NOT NULL,
	occurred_at     TEXT NOT NULL,
	data_epoch      TEXT NOT NULL,
	payload         BLOB NOT NULL,
	payload_hash    TEXT NOT NULL,
	state           TEXT NOT NULL DEFAULT 'pending',
	attempts        INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TEXT NOT NULL,
	lease_until     TEXT NOT NULL DEFAULT '',
	quarantine_note TEXT NOT NULL DEFAULT ''
)`
const schemaIndexSQL = `CREATE INDEX IF NOT EXISTS idx_nw_outbox_due ON nw_outbox (state, next_attempt_at)`

// EnsureSchema 创建outbox表与索引；幂等。
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, SchemaSQL); err != nil {
		return platform.NewError(platform.CodeInternal, true, "创建outbox表失败")
	}
	if _, err := db.ExecContext(ctx, schemaIndexSQL); err != nil {
		return platform.NewError(platform.CodeInternal, true, "创建outbox索引失败")
	}
	if _, err := db.ExecContext(ctx, observationSchemaSQL); err != nil {
		return platform.NewError(platform.CodeInternal, true, "创建outbox真实活动水位失败")
	}
	return nil
}

// insertSQL 与业务事务共用同一连接提交；事务回滚则事件不存在。
const insertSQL = `INSERT INTO nw_outbox
	(event_id, event_type, event_version, producer, subject_id, revision, occurred_at, data_epoch, payload, payload_hash, state, attempts, next_attempt_at, lease_until, quarantine_note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', 0, ?, '', '')`

// EnqueueEvent 在业务事务内登记outbox事件：
// 校验信封与载荷不含正文、密钥、IP，按eventId唯一插入pending行，
// 随调用方业务事务一起提交。同eventId同hash幂等成功，异hash返回CONFLICT。
func EnqueueEvent(tx platform.Tx, envelope platform.EventEnvelope) error {
	if err := validateEnvelope(envelope); err != nil {
		return err
	}
	payloadHash := platform.HashBytes(envelope.Payload)
	now := platform.Now()
	insert := func() error {
		_, err := tx.ExecContext(context.Background(), insertSQL,
			envelope.EventID, envelope.EventType, envelope.EventVersion, envelope.Producer,
			envelope.SubjectID, envelope.Revision, envelope.OccurredAt, envelope.DataEpoch,
			[]byte(envelope.Payload), payloadHash, now)
		return err
	}
	if err := insert(); err != nil {
		if !isUniqueViolation(err) {
			return platform.NewError(platform.CodeInternal, true, "写入outbox失败")
		}
	}
	existing, err := storedHash(tx, envelope.EventID)
	if err != nil {
		return err
	}
	if existing != payloadHash {
		return platform.NewError(platform.CodeConflict, false, "同eventId的事件已存在且内容不同")
	}
	return nil
}

const storedHashSQL = `SELECT payload_hash FROM nw_outbox WHERE event_id = ?`

// storedHash 读取已登记事件的载荷哈希；行不存在返回空哈希。
func storedHash(tx platform.Tx, eventID platform.Identifier) (platform.Hash, error) {
	var existing platform.Hash
	err := tx.QueryRowContext(context.Background(), storedHashSQL, eventID).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", platform.NewError(platform.CodeInternal, true, "读取outbox失败")
	}
	return existing, nil
}

// validateEnvelope 校验信封字段完整且载荷为合法JSON；
// 载荷必须符合生产方events.json登记的schema，由生产方装配侧保证。
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

// Store 是OutboxStore的SQLite默认实现；与业务库同库。
type Store struct {
	ensure sync.Once
	db     *sql.DB
}

// NewStore 构造默认outbox存储。
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// claimSQL 按提交顺序领取到期pending行并写入30秒行租约。
const claimSQL = `UPDATE nw_outbox SET lease_until = ?
WHERE event_id IN (
	SELECT event_id FROM nw_outbox
	WHERE state = 'pending' AND next_attempt_at <= ?
		AND (lease_until = '' OR lease_until <= ?)
	ORDER BY rowid LIMIT ?
) RETURNING event_id, event_type, event_version, producer, subject_id, revision,
	occurred_at, data_epoch, payload, payload_hash, state, attempts, next_attempt_at`

func (s *Store) ClaimBatch(ctx context.Context, limit int, lease time.Duration) ([]platform.OutboxRow, error) {
	now := platform.Now()
	until := platform.Timestamp(time.Now().Add(lease).UTC().Format(time.RFC3339Nano))
	rows, err := s.db.QueryContext(ctx, claimSQL, until, now, now, limit)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, true, "领取outbox失败")
	}
	defer rows.Close()
	var claimed []platform.OutboxRow
	for rows.Next() {
		var row platform.OutboxRow
		var payload []byte
		if err := rows.Scan(&row.EventID, &row.EventType, &row.EventVersion, &row.Producer,
			&row.SubjectID, &row.Revision, &row.OccurredAt, &row.DataEpoch,
			&payload, &row.PayloadHash, &row.State, &row.Attempts, &row.NextAttemptAt); err != nil {
			return nil, platform.NewError(platform.CodeInternal, true, "读取outbox行失败")
		}
		row.Payload = payload
		claimed = append(claimed, row)
	}
	if err := rows.Err(); err != nil {
		return nil, platform.NewError(platform.CodeInternal, true, "遍历outbox失败")
	}
	return claimed, nil
}

const markRelayedSQL = `UPDATE nw_outbox SET state = 'relayed', lease_until = '' WHERE event_id IN `

// MarkRelayed 批量确认已提交给jobs的行。
func (s *Store) MarkRelayed(ctx context.Context, eventIDs []platform.Identifier) error {
	if len(eventIDs) == 0 {
		return nil
	}
	query := markRelayedSQL + placeholders(len(eventIDs))
	args := make([]any, 0, len(eventIDs))
	for _, id := range eventIDs {
		args = append(args, id)
	}
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return platform.NewError(platform.CodeInternal, true, "标记relayed失败")
	}
	return nil
}

const releaseSQL = `UPDATE nw_outbox
SET lease_until = '', attempts = ?, next_attempt_at = ?
WHERE event_id = ?`

// ReleaseForRetry 释放租约并按退避安排下次转发。
func (s *Store) ReleaseForRetry(ctx context.Context, eventID platform.Identifier, attempts int, next time.Duration) error {
	at := platform.Timestamp(time.Now().Add(next).UTC().Format(time.RFC3339Nano))
	if _, err := s.db.ExecContext(ctx, releaseSQL, attempts, at, eventID); err != nil {
		return platform.NewError(platform.CodeInternal, true, "更新outbox重试计划失败")
	}
	return nil
}

const quarantineSQL = `UPDATE nw_outbox SET state = 'quarantined', lease_until = '', quarantine_note = ? WHERE event_id = ?`

// Quarantine 隔离同eventId异hash或信封非法的行，不丢弃。
func (s *Store) Quarantine(ctx context.Context, eventID platform.Identifier, reason string) error {
	if _, err := s.db.ExecContext(ctx, quarantineSQL, reason, eventID); err != nil {
		return platform.NewError(platform.CodeInternal, true, "隔离outbox行失败")
	}
	return nil
}

const discardSQL = `UPDATE nw_outbox SET state = 'discarded', lease_until = ''
WHERE state IN ('pending','quarantined') AND data_epoch <> ?`

// DiscardOtherEpochs 撤销旧代pending/quarantine活动资格并保留完整行审计；恢复换入后调用。
func (s *Store) DiscardOtherEpochs(ctx context.Context, epoch platform.Identifier) (int64, error) {
	// 转发循环总是先走这里；表还不存在（本域尚无事件产出）时顺手建好。
	s.ensure.Do(func() { _ = EnsureSchema(ctx, s.db) })
	result, err := s.db.ExecContext(ctx, discardSQL, epoch)
	if err != nil {
		return 0, platform.NewError(platform.CodeInternal, true, "隔离旧代outbox失败")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, platform.NewError(platform.CodeInternal, true, "统计旧代outbox失败")
	}
	return count, nil
}

const discardOneSQL = `UPDATE nw_outbox SET state = 'discarded', lease_until = ''
WHERE event_id = ? AND state = 'pending' AND data_epoch <> ?`

// Discard 丢弃单个不属于当前代的pending行。
func (s *Store) Discard(ctx context.Context, eventID, epoch platform.Identifier) error {
	if _, err := s.db.ExecContext(ctx, discardOneSQL, eventID, epoch); err != nil {
		return platform.NewError(platform.CodeInternal, true, "丢弃旧代outbox行失败")
	}
	return nil
}

// RegisterWorkTable将真实业务表纳入统一outbox control revision与sealed gate。
// 只接受启动装配时登记的简单SQLite表名；staging迁移完成前不得调用封口。
func RegisterWorkTable(ctx context.Context, db *sql.DB, table string) error {
	table = strings.ToLower(table)
	if db == nil || !validWorkTableName(table) {
		return platform.NewError(platform.CodeValidation, false, "业务表名不合法")
	}
	if err := EnsureSchema(ctx, db); err != nil {
		return err
	}
	triggerPrefix := "nw_work_" + platform.HashBytes([]byte(table))
	quotedTable := `"` + table + `"`
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "建立业务写门事务失败")
	}
	defer tx.Rollback()
	for _, action := range []string{"insert", "update", "delete"} {
		gateSQL := `CREATE TRIGGER IF NOT EXISTS "` + triggerPrefix + `_gate_` + action + `" BEFORE ` + strings.ToUpper(action) + ` ON ` + quotedTable + ` WHEN (SELECT sealed_hash FROM nw_outbox_control WHERE id=1)<>'' BEGIN SELECT RAISE(ABORT,'outbox sealed'); END;`
		revisionSQL := `CREATE TRIGGER IF NOT EXISTS "` + triggerPrefix + `_revision_` + action + `" AFTER ` + strings.ToUpper(action) + ` ON ` + quotedTable + ` BEGIN UPDATE nw_outbox_control SET revision=revision+1 WHERE id=1; END;`
		if _, err = tx.ExecContext(ctx, gateSQL); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "安装业务写门失败")
		}
		if _, err = tx.ExecContext(ctx, revisionSQL); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "安装业务水位触发器失败")
		}
	}
	if err = tx.Commit(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "提交业务写门失败")
	}
	return nil
}

func validWorkTableName(table string) bool {
	table = strings.ToLower(table)
	if table == "" || len(table) > 128 || strings.HasPrefix(table, "sqlite_") || table == "nw_outbox" || table == "nw_outbox_control" {
		return false
	}
	for index, char := range table {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_' || (index > 0 && char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func placeholders(n int) string {
	out := "("
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ","
		}
		out += "?"
	}
	return out + ")"
}

// isUniqueViolation 识别唯一约束冲突；SQLite驱动错误串均含此标记。
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
