// Package siteconfig 实现 site-config 的私有运行状态控制库。
//
// control.db 是本域恢复协议的权威存储，位于 control/ 而非 active/，
// 因此不会被业务快照换目录覆盖。本文件只承载数据库句柄、迁移与
// RuntimeState 的持久读写，不实现站点设置或其他业务库。
package siteconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
	"nice-wiki/modules/site-config/internal/migrations"
)

const runtimeDatabaseName = "control.db"

// requiredRuntimeModules 是需要登记恢复切换/换回回执的有状态域。
// portal 是无状态入口，不在 ActivationReceipt.module 枚举中。
var requiredRuntimeModules = []string{
	"identity",
	"site-config",
	"content",
	"media",
	"public-site",
	"model-gateway",
	"knowledge",
	"qa",
	"localization",
	"memory",
	"traffic-guard",
	"jobs",
	"analytics",
	"data-protection",
}

// RuntimeStore 是本域控制库句柄；同一进程仅允许一个活动实例。
type RuntimeStore struct {
	db        *sql.DB
	closeOnce sync.Once
	closeErr  error
}

var activeStore struct {
	sync.RWMutex
	store *RuntimeStore
}

// OpenRuntimeStore 打开 control/control.db，设置 ADR0005 要求的 SQLite
// 参数，执行只追加迁移，并在首次建库时写入 running 初始状态。
func OpenRuntimeStore(ctx context.Context, dataRoot string) (*RuntimeStore, error) {
	if dataRoot == "" {
		return nil, platform.NewError(platform.CodeValidation, false, "数据根缺失")
	}
	path := filepath.Join(dataRoot, "control", runtimeDatabaseName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库目录不可用")
	}
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开运行控制库失败")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &RuntimeStore{db: db}
	closeOnError := func(openErr error) (*RuntimeStore, error) {
		_ = db.Close()
		return nil, openErr
	}
	if err := db.PingContext(ctx); err != nil {
		return closeOnError(platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库不可用"))
	}
	if err := store.runMigrations(ctx); err != nil {
		return closeOnError(err)
	}
	if err := store.ensureInitialState(ctx); err != nil {
		return closeOnError(err)
	}
	activeStore.Lock()
	activeStore.store = store
	activeStore.Unlock()
	return store, nil
}

// Close 关闭控制库；重复关闭安全，并在成功关闭时撤销本进程的活动句柄。
func (s *RuntimeStore) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		activeStore.Lock()
		if activeStore.store == s {
			activeStore.store = nil
		}
		activeStore.Unlock()
		s.closeErr = s.db.Close()
	})
	if s.closeErr != nil {
		return platform.NewError(platform.CodeInternal, false, "关闭运行控制库失败")
	}
	return nil
}

// ReadRuntimeView 读取供 platform lifecycle 使用的脱敏运行摘要。
func (s *RuntimeStore) ReadRuntimeView(ctx context.Context) (platform.RuntimeView, error) {
	state, err := s.ReadRuntimeState(ctx)
	if err != nil {
		return platform.RuntimeView{}, err
	}
	return platform.RuntimeView{
		DataEpoch:       platform.Identifier(state.DataEpoch),
		Phase:           string(state.Phase),
		RestoreID:       platform.Identifier(state.RestoreId),
		TargetDataEpoch: platform.Identifier(state.TargetDataEpoch),
		Maintenance:     state.Maintenance,
	}, nil
}

// ReadRuntimeState 读取完整的内部 RuntimeState，供本域就绪检查和未来
// x-handler 适配使用；调用方不得把控制库内部句柄或密钥写入响应。
func (s *RuntimeStore) ReadRuntimeState(ctx context.Context) (gen.RuntimeState, error) {
	return s.readState(ctx)
}

func activeRuntimeStore() *RuntimeStore {
	activeStore.RLock()
	defer activeStore.RUnlock()
	return activeStore.store
}

func (s *RuntimeStore) runMigrations(ctx context.Context) error {
	all, err := migrations.All()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "运行控制库迁移集不合法")
	}
	applied := map[int64]string{}
	rows, err := s.db.QueryContext(ctx, `SELECT version, name FROM schema_migrations`)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		if !isMissingTable(err) {
			return platform.NewError(platform.CodeIntegrityError, false, "读取运行控制库迁移记录失败")
		}
	} else if err == nil {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "运行控制库迁移记录损坏")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "读取运行控制库迁移记录失败")
		}
		rows.Close()
	}
	for version := range applied {
		if version < 1 || version > int64(len(all)) {
			return platform.NewError(platform.CodeIntegrityError, false, "运行控制库迁移版本超出迁移集")
		}
	}
	for _, item := range all {
		if _, ok := applied[item.Version]; ok {
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return platform.NewError(platform.CodeInternal, false, "开始运行控制库迁移事务失败")
		}
		if _, err := tx.ExecContext(ctx, item.SQL); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeIntegrityError, false, "运行控制库迁移执行失败")
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			item.Version, item.Name, string(platform.Now())); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeInternal, false, "登记运行控制库迁移失败")
		}
		if err := tx.Commit(); err != nil {
			return platform.NewError(platform.CodeInternal, false, "提交运行控制库迁移失败")
		}
	}
	return nil
}

func (s *RuntimeStore) ensureInitialState(ctx context.Context) error {
	modules, err := json.Marshal(requiredRuntimeModules)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "编码运行域集合失败")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "打开运行状态事务失败")
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM runtime_state`).Scan(&count); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "读取运行状态单例失败")
	}
	if count == 0 {
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO runtime_state
                (singleton, data_epoch, target_data_epoch, phase, maintenance,
                 restore_id, barrier_id, revision, required_modules, updated_at)
            VALUES (1, ?, NULL, 'running', 0, NULL, NULL, 1, ?, ?)`,
			string(platform.NewIdentifier()), string(modules), string(platform.Now())); err != nil {
			return platform.NewError(platform.CodeInternal, false, "初始化运行状态失败")
		}
	} else if count != 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "运行状态单例数量不正确")
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "提交初始运行状态失败")
	}
	_, err = s.readState(ctx)
	return err
}

func (s *RuntimeStore) readState(ctx context.Context) (gen.RuntimeState, error) {
	if s == nil || s.db == nil {
		return gen.RuntimeState{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库不可用")
	}
	return s.readStateFrom(ctx, s.db)
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *RuntimeStore) readStateFrom(ctx context.Context, q queryer) (gen.RuntimeState, error) {
	var (
		dataEpoch, phase, requiredJSON, updatedAt string
		targetEpoch, restoreID, barrierID           sql.NullString
		maintenance, revision                      int64
	)
	err := q.QueryRowContext(ctx, `
        SELECT data_epoch, target_data_epoch, phase, maintenance, restore_id,
               barrier_id, revision, required_modules, updated_at
          FROM runtime_state WHERE singleton = 1`).Scan(
		&dataEpoch, &targetEpoch, &phase, &maintenance, &restoreID,
		&barrierID, &revision, &requiredJSON, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.RuntimeState{}, platform.NewError(platform.CodeIntegrityError, false, "运行状态缺失")
	}
	if err != nil {
		return gen.RuntimeState{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取运行状态失败")
	}
	modules := []string{}
	if err := json.Unmarshal([]byte(requiredJSON), &modules); err != nil {
		return gen.RuntimeState{}, platform.NewError(platform.CodeIntegrityError, false, "运行域集合损坏")
	}
	if err := validateRequiredModules(modules); err != nil {
		return gen.RuntimeState{}, err
	}
	if !platform.ValidIdentifier(platform.Identifier(dataEpoch)) || revision < 1 {
		return gen.RuntimeState{}, platform.NewError(platform.CodeIntegrityError, false, "运行状态基础字段不合法")
	}
	if maintenance != 0 && maintenance != 1 {
		return gen.RuntimeState{}, platform.NewError(platform.CodeIntegrityError, false, "运行维护标志不合法")
	}
	parsedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return gen.RuntimeState{}, platform.NewError(platform.CodeIntegrityError, false, "运行状态时间不合法")
	}
	state := gen.RuntimeState{
		DataEpoch:       gen.Identifier(dataEpoch),
		TargetDataEpoch: nullableIdentifier(targetEpoch),
		Phase:           gen.RuntimePhase(phase),
		Maintenance:     maintenance == 1,
		RestoreId:       nullableIdentifier(restoreID),
		BarrierId:       nullableIdentifier(barrierID),
		Revision:        gen.Revision(revision),
		RequiredModules: append([]string(nil), modules...),
		UpdatedAt:       parsedAt.UTC(),
	}
	if err := validateRuntimeStateShape(state); err != nil {
		return gen.RuntimeState{}, err
	}
	receipts, err := s.readReceipts(ctx, q, state.RestoreId)
	if err != nil {
		return gen.RuntimeState{}, err
	}
	state.ActivationReceipts = receipts
	return state, nil
}

func (s *RuntimeStore) readReceipts(ctx context.Context, q queryer, restoreID gen.Identifier) ([]gen.ActivationReceipt, error) {
	if restoreID == "" {
		return []gen.ActivationReceipt{}, nil
	}
	rows, err := q.QueryContext(ctx, `
        SELECT module, stage_id, target_data_epoch, state, manifest_hash,
               recorded_at, signature
          FROM activation_receipts WHERE restore_id = ? ORDER BY module`, string(restoreID))
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "读取激活回执失败")
	}
	defer rows.Close()
	receipts := make([]gen.ActivationReceipt, 0)
	for rows.Next() {
		var module, stageID, targetEpoch, state, manifestHash, recordedAt, signature string
		if err := rows.Scan(&module, &stageID, &targetEpoch, &state, &manifestHash, &recordedAt, &signature); err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "激活回执损坏")
		}
		parsedAt, err := time.Parse(time.RFC3339Nano, recordedAt)
		if err != nil || !platform.ValidIdentifier(platform.Identifier(stageID)) || !platform.ValidIdentifier(platform.Identifier(targetEpoch)) || !platform.ValidHash(platform.Hash(manifestHash)) || signature == "" {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "激活回执字段不合法")
		}
		if !isRequiredModule(module) || (state != string(gen.Switched) && state != string(gen.RolledBack)) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "激活回执枚举不合法")
		}
		receipts = append(receipts, gen.ActivationReceipt{
			Module:          gen.ActivationReceiptModule(module),
			StageId:         gen.Identifier(stageID),
			TargetDataEpoch: gen.Identifier(targetEpoch),
			State:           gen.ActivationReceiptState(state),
			ManifestHash:    gen.Hash(manifestHash),
			RecordedAt:      parsedAt.UTC(),
			Signature:       signature,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "读取激活回执失败")
	}
	if len(receipts) > 30 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "激活回执数量超限")
	}
	return receipts, nil
}

func (s *RuntimeStore) readOperation(ctx context.Context, q queryer, operationID, kind, inputHash string) (gen.RuntimeState, bool, error) {
	if operationID == "" {
		return gen.RuntimeState{}, false, nil
	}
	var storedKind, storedHash, resultJSON string
	err := q.QueryRowContext(ctx, `
        SELECT kind, input_hash, result_json
          FROM runtime_operation_receipts WHERE operation_id = ?`, string(operationID)).Scan(
		&storedKind, &storedHash, &resultJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.RuntimeState{}, false, nil
	}
	if err != nil {
		return gen.RuntimeState{}, false, platform.NewError(platform.CodeDependencyUnavailable, true, "读取运行操作回执失败")
	}
	if storedKind != kind || storedHash != inputHash {
		return gen.RuntimeState{}, false, platform.NewError(platform.CodeConflict, false, "相同operationId对应不同请求")
	}
	var state gen.RuntimeState
	if err := json.Unmarshal([]byte(resultJSON), &state); err != nil {
		return gen.RuntimeState{}, false, platform.NewError(platform.CodeIntegrityError, false, "运行操作回执损坏")
	}
	if err := validateRuntimeStateShape(state); err != nil {
		return gen.RuntimeState{}, false, err
	}
	return state, true, nil
}

func (s *RuntimeStore) writeOperation(ctx context.Context, tx *sql.Tx, operationID, kind, inputHash string, state gen.RuntimeState) error {
	if operationID == "" {
		return nil
	}
	resultJSON, err := json.Marshal(state)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "编码运行操作回执失败")
	}
	_, err = tx.ExecContext(ctx, `
        INSERT INTO runtime_operation_receipts
            (operation_id, kind, input_hash, result_json, created_at)
		VALUES (?, ?, ?, ?, ?)`, string(operationID), kind, inputHash, string(resultJSON), string(platform.Now()))
	if err != nil {
		return platform.NewError(platform.CodeConflict, false, "保存运行操作回执失败")
	}
	return nil
}

func validateRequiredModules(modules []string) error {
	if len(modules) != len(requiredRuntimeModules) {
		return platform.NewError(platform.CodeIntegrityError, false, "requiredModules数量不符合14个有状态域")
	}
	for i, module := range modules {
		if module != requiredRuntimeModules[i] {
			return platform.NewError(platform.CodeIntegrityError, false, "requiredModules顺序或成员不合法")
		}
	}
	return nil
}

func isRequiredModule(module string) bool {
	for _, required := range requiredRuntimeModules {
		if required == module {
			return true
		}
	}
	return false
}

func validateRuntimeStateShape(state gen.RuntimeState) error {
	if !platform.ValidIdentifier(platform.Identifier(state.DataEpoch)) || state.Revision < 1 || len(state.RequiredModules) != len(requiredRuntimeModules) {
		return platform.NewError(platform.CodeIntegrityError, false, "运行状态不完整")
	}
	if err := validateRequiredModules(state.RequiredModules); err != nil {
		return err
	}
	switch state.Phase {
	case gen.Running, gen.BackupBarrier, gen.RestoreStaging, gen.RestoreSwitching, gen.RestoreRollingBack, gen.RestoreBlocked:
	default:
		return platform.NewError(platform.CodeIntegrityError, false, "运行phase不合法")
	}
	if state.Maintenance != (state.Phase != gen.Running) {
		return platform.NewError(platform.CodeIntegrityError, false, "运行phase与maintenance不一致")
	}
	if state.Phase == gen.Running {
		if state.TargetDataEpoch != "" || state.RestoreId != "" || state.BarrierId != "" {
			return platform.NewError(platform.CodeIntegrityError, false, "running状态包含恢复字段")
		}
		return nil
	}
	if state.BarrierId == "" {
		return platform.NewError(platform.CodeIntegrityError, false, "维护状态缺少barrierId")
	}
	if state.Phase != gen.BackupBarrier {
		if !platform.ValidIdentifier(platform.Identifier(state.RestoreId)) || !platform.ValidIdentifier(platform.Identifier(state.TargetDataEpoch)) {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复状态缺少restoreId或targetDataEpoch")
		}
	}
	for _, receipt := range state.ActivationReceipts {
		if string(receipt.TargetDataEpoch) != string(state.TargetDataEpoch) {
			return platform.NewError(platform.CodeIntegrityError, false, "激活回执代次与运行状态不一致")
		}
	}
	return nil
}

func nullableIdentifier(value sql.NullString) gen.Identifier {
	if !value.Valid {
		return ""
	}
	return gen.Identifier(value.String)
}

func isMissingTable(err error) bool {
	return err != nil && (errors.Is(err, sql.ErrNoRows) ||
		containsSQLiteMessage(err, "no such table"))
}

func containsSQLiteMessage(err error, fragment string) bool {
	return err != nil && len(err.Error()) >= len(fragment) &&
		stringContains(err.Error(), fragment)
}

func stringContains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
