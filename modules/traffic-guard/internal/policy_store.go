// Package trafficguard 内聚本域私有策略库的启动恢复边界。
//
// 登录保护与策略管理通过可信上下文注入本库；策略、回执及outbox共用短事务。
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
	guardmigrations "nice-wiki/modules/traffic-guard/migrations"
)

const (
	policyDatabaseName = "traffic-guard.db"
	policyJournalName  = "traffic-guard-journal.jsonl"
)

// PolicyStore 是本域active区数据库和control区决定日志的句柄。
// control区日志不随恢复换入的业务目录替换。
type PolicyStore struct {
	mu              sync.RWMutex
	openedDataEpoch Identifier
	db              *sql.DB
	journal         *os.File
	jmu             sync.Mutex

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

// OpenPolicyStore 打开active区SQLite和control区journal，启用WAL、外键及
// 3000ms busy timeout；任一资源失败都会关闭已打开句柄。
func OpenPolicyStore(dataRoot string, openedDataEpoch Identifier) (*PolicyStore, error) {
	if openedDataEpoch == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "本域数据代次尚未建立")
	}
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
	return &PolicyStore{db: db, journal: journal, openedDataEpoch: openedDataEpoch}, nil
}

type policyStoreContextKey struct{}

// WithPolicyStore 只供可信装配注入已绑定active代次的本域库。
func WithPolicyStore(ctx context.Context, store *PolicyStore) context.Context {
	return context.WithValue(ctx, policyStoreContextKey{}, store)
}

// RunMigrations 按编号升序执行只追加迁移；数据库中出现迁移集之外的
// 版本时拒绝启动，迁移与版本登记在同一SQLite事务内提交。
func (s *PolicyStore) RunMigrations() error {
	if s == nil || s.db == nil {
		return platform.NewError(platform.CodeValidation, false, "本域业务库句柄缺失")
	}
	all, err := guardmigrations.All()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "本域迁移集不合法")
	}
	applied := map[int64]string{}
	rows, err := s.db.Query(`SELECT version, name FROM schema_migrations`)
	if err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			return platform.NewError(platform.CodeIntegrityError, false, "读取本域迁移记录失败")
		}
	} else {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				_ = rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "本域迁移记录损坏")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "读取本域迁移记录失败")
		}
		_ = rows.Close()
	}
	for version, name := range applied {
		if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
			return platform.NewError(platform.CodeIntegrityError, false, "本域已应用迁移超出迁移集")
		}
	}
	for _, migration := range all {
		if _, ok := applied[migration.Version]; ok {
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
	s.mu.Lock()
	defer s.mu.Unlock()
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

// readPolicyJournal 按条目ID聚合决定日志：同ID后写的done覆盖open，返回每个
// 决定的最终状态；任何损坏行都按完整性错误拒绝启动。
func readPolicyJournal(name string) ([]policyJournalEntry, error) {
	raw, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "读取本域决定日志失败")
	}
	latest := map[string]policyJournalEntry{}
	order := make([]string, 0)
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		var entry policyJournalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil || entry.ID == "" || entry.State == "" {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "本域决定日志损坏")
		}
		if _, seen := latest[entry.ID]; !seen {
			order = append(order, entry.ID)
		}
		latest[entry.ID] = entry
	}
	entries := make([]policyJournalEntry, 0, len(order))
	for _, id := range order {
		entries = append(entries, latest[id])
	}
	return entries, nil
}

// GetPolicy 的站点和代次只来自可信上下文，不接受GET请求自选站点。
type GetPolicyInput struct { SiteID string; DataEpoch string }
type GetPolicyResult struct { PolicyJSON GuardPolicy `json:"policyJSON"`; Revision int64 `json:"revision"` }
type UpdatePolicyInput struct { SiteID string `json:"siteId"`; ExpectedRevision int64 `json:"expectedRevision"`; PolicyJSON GuardPolicy `json:"policyJSON"`; DataEpoch string `json:"-"` }
type UpdatePolicyResult struct { Revision int64 `json:"revision"` }

func GetPolicy(ctx context.Context,op OperationContext,in GetPolicyInput)(GetPolicyResult,error){
 if in.SiteID!=op.Actor.SiteID||in.DataEpoch!=op.DataEpoch{return GetPolicyResult{},platform.NewError(platform.CodeForbidden,false,"策略作用域必须匹配可信站点")}
 if !(op.Actor.Kind=="service"&&op.CallerService=="traffic-guard"){if err:=authorizeGuardManagement(ctx,op,"getPolicy");err!=nil{return GetPolicyResult{},err}}
 t,err:=beginGuardTransaction(ctx,op,false);if err!=nil{return GetPolicyResult{},err};defer t.close()
 policy,revision,err:=readGuardPolicy(t);if err!=nil{return GetPolicyResult{},err};return GetPolicyResult{policy,revision},nil
}

// UpdatePolicy 在同一短事务中保存策略、原修订回执及outbox；首次仅允许0→1。
func UpdatePolicy(ctx context.Context,op OperationContext,in UpdatePolicyInput)(UpdatePolicyResult,error){
 if in.SiteID!=op.Actor.SiteID||in.DataEpoch!=op.DataEpoch{return UpdatePolicyResult{},platform.NewError(platform.CodeForbidden,false,"策略作用域必须匹配可信站点")}
 if err:=authorizeGuardManagement(ctx,op,"updatePolicy");err!=nil{return UpdatePolicyResult{},err}
 validated,err:=ValidatePolicy(ctx,op,ValidatePolicyInput{in.PolicyJSON});if err!=nil{return UpdatePolicyResult{},err}
 t,err:=beginGuardTransaction(ctx,op,true);if err!=nil{return UpdatePolicyResult{},err};defer t.close()
 var result UpdatePolicyResult;if found,err:=readGuardReceipt(t,"policy.update",in,&result);err!=nil{return UpdatePolicyResult{},err}else if found{return result,nil}
 var revision int64;var epoch string
 err=t.tx.QueryRowContext(t.ctx,`SELECT revision,data_epoch FROM guard_policy WHERE site_id=?`,t.site).Scan(&revision,&epoch)
 if err!=nil&&!errors.Is(err,sql.ErrNoRows){return UpdatePolicyResult{},loginDBError()}
 exists:=err==nil
 if exists&&epoch!=op.DataEpoch{return UpdatePolicyResult{},platform.NewError(platform.CodeVersionMismatch,false,"风控策略代次已变化")}
 if revision!=in.ExpectedRevision||revision==9223372036854775807{return UpdatePolicyResult{},platform.NewError(platform.CodeVersionMismatch,false,"风控策略修订已变化")}
 raw,err:=json.Marshal(validated.NormalizedJSON);if err!=nil{return UpdatePolicyResult{},platform.NewError(platform.CodeInternal,false,"策略无法编码")}
 result.Revision=revision+1
 if exists{_,err=t.tx.ExecContext(t.ctx,`UPDATE guard_policy SET revision=?,policy_json=? WHERE site_id=? AND data_epoch=? AND revision=?`,result.Revision,raw,t.site,op.DataEpoch,revision)}else{_,err=t.tx.ExecContext(t.ctx,`INSERT INTO guard_policy(site_id,revision,policy_json,data_epoch) VALUES(?,?,?,?)`,t.site,result.Revision,raw,op.DataEpoch)}
 if err!=nil{return UpdatePolicyResult{},loginDBError()}
 if err=saveGuardReceipt(t,"policy.update",in,result);err!=nil{return UpdatePolicyResult{},err}
 if err=enqueueGuardEvent(t,"traffic-guard.policy-changed.v1",t.site,result.Revision,map[string]any{"policyRevision":result.Revision,"changedBy":op.Actor.SubjectID});err!=nil{return UpdatePolicyResult{},err}
 if err=t.commit();err!=nil{return UpdatePolicyResult{},err};return result,nil
}
