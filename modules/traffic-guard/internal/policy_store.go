// Package trafficguard 内聚本域私有策略库的启动恢复边界。
//
// GetPolicy/UpdatePolicy 的对外绑定仍需等契约补齐存储注入、GET站点来源、
// identity授权客户端和幂等回执语义后实现；本文件先实现其无歧义的SQLite、
// 迁移和control journal基础。
package trafficguard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
)

const (
	policyDatabaseName = "traffic-guard.db"
	policyJournalName  = "traffic-guard-journal.jsonl"
)

// PolicyStore 是本域active区数据库和control区决定日志的句柄。
// control区日志不随恢复换入的业务目录替换。
type PolicyStore struct {
	db      *sql.DB
	journal *os.File
	jmu     sync.Mutex

	JournalRecovered  bool
	AppliedMigrations int64
}

type policyJournalEntry struct {
	ID      string             `json:"id"`
	Kind    string             `json:"kind"`
	State   string             `json:"state"`
	Payload json.RawMessage    `json:"payload,omitempty"`
	At      platform.Timestamp `json:"at"`
}

// policyMigration 与已登记的 migrations/0001_domain.sql 保持字节语义一致；
// 迁移文件是审阅和归档来源，执行时由本包内嵌合同常量避免依赖源码目录。
type policyMigration struct {
	Version int64
	Name    string
	SQL     string
}

const policyMigrationSQL = `CREATE TABLE guard_policy (
    site_id     TEXT PRIMARY KEY,
    revision    INTEGER NOT NULL CHECK (revision >= 0),
    policy_json TEXT NOT NULL CHECK (json_valid(policy_json)),
    data_epoch  TEXT NOT NULL
);

CREATE TABLE policy_operation_receipts (
    site_id       TEXT NOT NULL,
    operation_id  TEXT NOT NULL,
    input_hash    TEXT NOT NULL,
    revision      INTEGER NOT NULL CHECK (revision >= 0),
    data_epoch    TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    PRIMARY KEY (site_id, operation_id)
);

CREATE INDEX idx_policy_receipts_epoch
    ON policy_operation_receipts (data_epoch, created_at);

CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);`

// OpenPolicyStore 打开active区SQLite和control区journal，启用WAL、外键及
// 3000ms busy timeout；任一资源失败都会关闭已打开句柄。
func OpenPolicyStore(dataRoot string) (*PolicyStore, error) {
	if !filepath.IsAbs(dataRoot) {
		return nil, platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	active := filepath.Join(dataRoot, "active")
	control := filepath.Join(dataRoot, "control")
	if err := os.MkdirAll(active, 0o700); err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "建立业务数据目录失败")
	}
	if err := os.MkdirAll(control, 0o700); err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "建立控制数据目录失败")
	}

	dsn := "file:" + filepath.Join(active, policyDatabaseName) +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开本域业务库失败")
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域业务库不可用")
	}
	if err := outbox.EnsureSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}

	journal, err := os.OpenFile(filepath.Join(control, policyJournalName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeInternal, false, "打开本域决定日志失败")
	}
	return &PolicyStore{db: db, journal: journal}, nil
}

// RunMigrations 按编号升序执行只追加迁移；数据库中出现迁移集之外的
// 版本时拒绝启动，迁移与版本登记在同一SQLite事务内提交。
func (s *PolicyStore) RunMigrations() error {
	if s == nil || s.db == nil {
		return platform.NewError(platform.CodeValidation, false, "本域业务库句柄缺失")
	}
	all := []policyMigration{{Version: 1, Name: "0001_domain.sql", SQL: policyMigrationSQL}}

	applied := map[int64]string{}
	rows, err := s.db.Query(`SELECT version, name FROM schema_migrations`)
	if err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			return platform.NewError(platform.CodeIntegrityError, false, "读取本域迁移记录失败")
		}
	} else {
		defer rows.Close()
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "本域迁移记录损坏")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "读取本域迁移记录失败")
		}
	}
	for version := range applied {
		if version < 1 || version > int64(len(all)) {
			return platform.NewError(platform.CodeIntegrityError, false, "本域已应用迁移超出迁移集")
		}
	}
	for _, migration := range all {
		if applied[migration.Version] != "" {
			continue
		}
		tx, err := s.db.Begin()
		if err != nil {
			return platform.NewError(platform.CodeInternal, false, "开始本域迁移事务失败")
		}
		if _, err := tx.Exec(migration.SQL); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeIntegrityError, false, "执行本域迁移失败")
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			migration.Version, migration.Name, platform.Now()); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeInternal, false, "登记本域迁移失败")
		}
		if err := tx.Commit(); err != nil {
			return platform.NewError(platform.CodeInternal, false, "提交本域迁移失败")
		}
		s.AppliedMigrations = migration.Version
	}
	if s.AppliedMigrations == 0 {
		s.AppliedMigrations = int64(len(all))
	}
	return nil
}

// RecoverJournal 校验control区决定日志。存在损坏行或未完成决定时拒绝
// 启动，避免把未知持久状态当成已提交。
func (s *PolicyStore) RecoverJournal() error {
	if s == nil || s.journal == nil {
		return platform.NewError(platform.CodeValidation, false, "本域决定日志句柄缺失")
	}
	entries, err := readPolicyJournal(s.journal.Name())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.State == "open" {
			return platform.NewError(platform.CodeIntegrityError, false, "本域决定日志存在未完成条目")
		}
	}
	s.JournalRecovered = true
	return nil
}

// Close 先flush并关闭control journal，再关闭SQLite；重复调用安全。
func (s *PolicyStore) Close() error {
	if s == nil {
		return nil
	}
	s.jmu.Lock()
	defer s.jmu.Unlock()
	if s.journal != nil {
		if err := s.journal.Sync(); err != nil {
			return platform.NewError(platform.CodeInternal, false, "flush本域决定日志失败")
		}
		if err := s.journal.Close(); err != nil {
			return platform.NewError(platform.CodeInternal, false, "关闭本域决定日志失败")
		}
		s.journal = nil
	}
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		if err != nil {
			return platform.NewError(platform.CodeInternal, false, "关闭本域业务库失败")
		}
	}
	return nil
}

func readPolicyJournal(name string) ([]policyJournalEntry, error) {
	raw, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "读取本域决定日志失败")
	}
	var entries []policyJournalEntry
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		var entry policyJournalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil || entry.ID == "" || entry.State == "" {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "本域决定日志损坏")
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
