// Package identity 内聚本域私有持久化与业务规则；本文件是active区
// 业务库打开、迁移执行与control区决定日志恢复的机制层，不放产品算法。
package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite" // 纯Go驱动，注册"sqlite"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/identity/migrations"
)

const (
	databaseName = "identity.db"
	journalName  = "identity-journal.jsonl"
)

// Store 是本域active区业务库与control区决定日志的句柄。
type Store struct {
	db      *sql.DB
	journal *os.File
	jmu     sync.Mutex
	// JournalRecovered 本次启动恢复后是否已无未完成决定。
	JournalRecovered bool
	// AppliedMigrations 本次启动后已应用的迁移数。
	AppliedMigrations int64
}

// journalEntry 决定日志一行；同一id可追加多行，最后一行状态生效。
type journalEntry struct {
	ID      string             `json:"id"`
	Kind    string             `json:"kind"`
	State   string             `json:"state"`
	Payload json.RawMessage    `json:"payload,omitempty"`
	At      platform.Timestamp `json:"at"`
}

// OpenDatabase 打开active区业务库（WAL、foreign_keys、busy_timeout=3000ms）
// 与control区决定日志。control区不随业务恢复替换。
func OpenDatabase(dataRoot string) (*Store, error) {
	path := filepath.Join(dataRoot, "active", databaseName)
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开业务库失败")
	}
	// 单连接串行化写入，避免池化连接互相busy；读负载由本域规模决定可接受。
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "业务库不可用")
	}
	journal, err := os.OpenFile(filepath.Join(dataRoot, "control", journalName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		db.Close()
		return nil, platform.NewError(platform.CodeInternal, false, "打开决定日志失败")
	}
	return &Store{db: db, journal: journal}, nil
}

// RunMigrations 按编号升序在事务中应用未执行的迁移；数据库中出现
// 迁移集之外的已应用版本视为schema未知，拒绝启动。
func (s *Store) RunMigrations() error {
	all, err := migrations.All()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "迁移集不合法")
	}
	applied := map[int64]string{}
	rows, err := s.db.Query(`SELECT version, name FROM schema_migrations`)
	if err != nil {
		// 首个迁移自举schema_migrations表，缺表视为空集。
		if !strings.Contains(err.Error(), "no such table") {
			return platform.NewError(platform.CodeIntegrityError, false, "读取迁移记录失败")
		}
	} else {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "迁移记录损坏")
			}
			applied[version] = name
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "迁移记录读取失败")
		}
	}
	for version := range applied {
		if version < 1 || version > int64(len(all)) {
			return platform.NewError(platform.CodeIntegrityError, false, "已应用迁移版本超出迁移集")
		}
	}
	for _, item := range all {
		if _, done := applied[item.Version]; done {
			continue
		}
		tx, err := s.db.Begin()
		if err != nil {
			return platform.NewError(platform.CodeInternal, false, "开始迁移事务失败")
		}
		if _, err := tx.Exec(item.SQL); err != nil {
			tx.Rollback()
			return platform.NewError(platform.CodeIntegrityError, false, "迁移执行失败")
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			item.Version, item.Name, string(platform.Now())); err != nil {
			tx.Rollback()
			return platform.NewError(platform.CodeInternal, false, "登记迁移失败")
		}
		if err := tx.Commit(); err != nil {
			return platform.NewError(platform.CodeInternal, false, "提交迁移失败")
		}
		s.AppliedMigrations = item.Version
	}
	if s.AppliedMigrations == 0 {
		s.AppliedMigrations = int64(len(all))
	}
	return nil
}

// RecoverJournal 扫描决定日志并使未完成决定可见：存在open决定时返回
// INTEGRITY_ERROR隔离启动，由业务函数落地各自补完成逻辑。
func (s *Store) RecoverJournal() error {
	entries, err := readJournal(s.journal)
	if err != nil {
		return err
	}
	open := 0
	for _, entry := range entries {
		if entry.State == "open" {
			open++
		}
	}
	if open > 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "决定日志存在未完成条目")
	}
	s.JournalRecovered = true
	return nil
}

// AppendDecision 登记一条open决定并fsync；业务事务提交后应尽快Complete。
func (s *Store) AppendDecision(kind string, payload any) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", platform.NewError(platform.CodeValidation, false, "决定载荷不可编码")
	}
	id := string(platform.NewIdentifier())
	if err := appendJournalLine(s.journal, &s.jmu, journalEntry{ID: id, Kind: kind, State: "open", Payload: raw, At: platform.Now()}); err != nil {
		return "", err
	}
	return id, nil
}

// CompleteDecision 追加done行完结决定；未知id静默成功由恢复语义兜底。
func (s *Store) CompleteDecision(id string) error {
	return appendJournalLine(s.journal, &s.jmu, journalEntry{ID: id, State: "done", At: platform.Now()})
}

// HasSuperadmin 报告唯一超管是否已注册（就绪检查用，只读）。
func (s *Store) HasSuperadmin(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE is_superadmin = 1`).Scan(&count); err != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "账户权威表不可读")
	}
	return count > 0, nil
}

// Close flush决定日志并关闭全部句柄；重复调用安全。
func (s *Store) Close() error {
	if s.journal != nil {
		_ = s.journal.Sync()
		_ = s.journal.Close()
		s.journal = nil
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// readJournal 按行读取决定日志，同一id最后一行状态生效。
func readJournal(file *os.File) (map[string]journalEntry, error) {
	raw, err := os.ReadFile(file.Name())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, platform.NewError(platform.CodeInternal, false, "决定日志不可读")
	}
	states := map[string]journalEntry{}
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		var entry journalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil || entry.ID == "" || entry.State == "" {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "决定日志损坏")
		}
		states[entry.ID] = entry
	}
	return states, nil
}

// appendJournalLine 串行追加一行并fsync。
func appendJournalLine(file *os.File, mu *sync.Mutex, entry journalEntry) error {
	raw, err := json.Marshal(entry)
	if err != nil {
		return platform.NewError(platform.CodeValidation, false, "决定行不可编码")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return platform.NewError(platform.CodeInternal, false, "决定日志写入失败")
	}
	if err := file.Sync(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "决定日志fsync失败")
	}
	return nil
}
