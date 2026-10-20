package trafficguard

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/traffic-guard/internal/gen"
	guardmigrations "nice-wiki/modules/traffic-guard/migrations"
)

// GuardRuntime 只由本域可信装配提供；预算key独立于可轮换的服务签名key。
// 预算key及前期别名必须覆盖30日ban过渡，不随业务数据快照替换。
type GuardRuntime struct {
	Store                 *PolicyStore
	Identity              platform.AuthorizationClient
	WriteWindow           platform.WriteWindowConfig
	Runtime               func(context.Context) (platform.RuntimeView, error)
	Snapshot              lifecycle.SnapshotAPI
	SigningKey            platform.SigningKey
	BudgetKey             platform.SigningKey
	PreviousBudgetKeys    []platform.SigningKey
	Anchors               platform.TrustAnchors
	ValidateIdentityEvent func(json.RawMessage) error
	Turnstile             TurnstileConfig
	Jobs                  *gen.JobsClient
}
type guardRuntimeKey struct{}
type guardTransactionKey struct{}
type guardAuthorizationKey struct{}
type guardAuthorizationMark struct {
	operationID string
	action      string
}

var guardWorkTables = [...]string{
	"guard_policy",
	"policy_operation_receipts",
	"login_attempts",
	"login_attempt_networks",
	"login_failures",
	"login_locks",
	"login_finish_receipts",
	"guard_operation_receipts",
	"guard_subjects",
	"guard_network_aliases",
	"guard_buckets",
	"guard_permits",
	"guard_challenges",
	"guard_bans",
	"guard_question_signals",
	"guard_risk_events",
	"guard_waiters",
	"guard_identity_revisions",
	"guard_permit_networks",
}

var guardPendingWorkQueries = [...]string{
	"SELECT COUNT(*) FROM login_attempts WHERE outcome='pending'",
	"SELECT COUNT(*) FROM guard_permits WHERE state='active'",
	"SELECT COUNT(*) FROM guard_challenges WHERE state='issued'",
	"SELECT COUNT(*) FROM guard_waiters",
}

func WithGuardRuntime(ctx context.Context, runtime GuardRuntime) context.Context {
	return context.WithValue(WithPolicyStore(ctx, runtime.Store), guardRuntimeKey{}, runtime)
}

type guardTransaction struct {
	runtime GuardRuntime
	store   *PolicyStore
	tx      *sql.Tx
	ctx     context.Context
	op      OperationContext
	site    string
	now     time.Time
	owned   bool
	cancel  context.CancelFunc
}

// beginGuardTransaction 在任何本地锁之前建立当前写窗口；组合准入复用同一事务。
func beginGuardTransaction(ctx context.Context, op OperationContext, write bool) (*guardTransaction, error) {
	if existing, ok := ctx.Value(guardTransactionKey{}).(*guardTransaction); ok {
		nested := *existing
		nested.owned = false
		return &nested, nil
	}
	runtime, ok := ctx.Value(guardRuntimeKey{}).(GuardRuntime)
	if !ok || runtime.Store == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "风控运行依赖未装配")
	}
	if op.Actor.SiteID == "" {
		return nil, platform.NewError(platform.CodeForbidden, false, "可信站点上下文缺失")
	}
	work := ctx
	cancel := context.CancelFunc(func() {})
	if write {
		if runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "风控写窗口未装配")
		}
		var view platform.RuntimeView
		var err error
		work, view, cancel, err = lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
		if err != nil {
			return nil, err
		}
		if view.DataEpoch != op.DataEpoch {
			cancel()
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "风控写入代次已变化")
		}
	}
	store := runtime.Store
	store.mu.RLock()
	if store.db == nil || !store.JournalRecovered {
		store.mu.RUnlock()
		cancel()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "风控私有库尚未恢复")
	}
	if store.openedDataEpoch != op.DataEpoch {
		store.mu.RUnlock()
		cancel()
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "风控打开代次已变化")
	}
	tx, err := store.db.BeginTx(work, nil)
	if err != nil {
		store.mu.RUnlock()
		cancel()
		return nil, loginDBError()
	}
	transaction := &guardTransaction{runtime: runtime, store: store, tx: tx, ctx: work, op: op, site: op.Actor.SiteID, now: time.Now().UTC(), owned: true, cancel: cancel}
	transaction.ctx = context.WithValue(work, guardTransactionKey{}, transaction)
	return transaction, nil
}
func (t *guardTransaction) close() {
	if t.owned {
		_ = t.tx.Rollback()
		t.store.mu.RUnlock()
		t.cancel()
	}
}
func (t *guardTransaction) commit() error {
	if !t.owned {
		return nil
	}
	if err := t.tx.Commit(); err != nil {
		return loginDBError()
	}
	return nil
}

func authorizeGuardManagement(ctx context.Context, op OperationContext, action string) error {
	if checked, ok := ctx.Value(guardAuthorizationKey{}).(guardAuthorizationMark); ok && checked.operationID == op.OperationID && checked.action == action {
		return nil
	}
	runtime, ok := ctx.Value(guardRuntimeKey{}).(GuardRuntime)
	if !ok || runtime.Identity == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "风控即时授权客户端未装配")
	}
	if op.Actor.Kind != "admin" || op.Actor.SessionID == "" {
		return platform.NewError(platform.CodeForbidden, false, "需要当前后台管理权限")
	}
	return apivalidate.NewIdentityAuthorizer(runtime.Identity)(ctx, op, "guard.manage", "traffic-guard:"+action)
}

func readGuardReceipt(t *guardTransaction, kind string, input any, result any) (bool, error) {
	account := ""
	if t.op.Actor.Kind == "admin" {
		account = t.op.Actor.SubjectID
	}
	canonical, err := platform.CanonicalJSON(map[string]any{"account": account, "input": input})
	if err != nil {
		return false, platform.NewError(platform.CodeValidation, false, "风控操作输入无法编码")
	}
	var digest string
	var raw []byte
	err = t.tx.QueryRowContext(t.ctx, `SELECT input_hash,result_json FROM guard_operation_receipts WHERE site_id=? AND data_epoch=? AND caller=? AND operation_kind=? AND operation_id=?`, t.site, t.op.DataEpoch, t.op.CallerService, kind, t.op.OperationID).Scan(&digest, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, loginDBError()
	}
	if digest != platform.HashBytes(canonical) {
		return false, platform.NewError(platform.CodeConflict, false, "同操作已绑定其他风控输入")
	}
	if json.Unmarshal(raw, result) != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "风控原回执损坏")
	}
	return true, nil
}
func saveGuardReceipt(t *guardTransaction, kind string, input any, result any) error {
	account := ""
	if t.op.Actor.Kind == "admin" {
		account = t.op.Actor.SubjectID
	}
	canonical, err := platform.CanonicalJSON(map[string]any{"account": account, "input": input})
	if err != nil {
		return platform.NewError(platform.CodeValidation, false, "风控操作输入无法编码")
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "风控回执无法编码")
	}
	_, err = t.tx.ExecContext(t.ctx, `INSERT INTO guard_operation_receipts(site_id,data_epoch,caller,operation_kind,operation_id,input_hash,result_json,created_at_ns) VALUES(?,?,?,?,?,?,?,?)`, t.site, t.op.DataEpoch, t.op.CallerService, kind, t.op.OperationID, platform.HashBytes(canonical), raw, t.now.UnixNano())
	if err != nil {
		return loginDBError()
	}
	return nil
}
func enqueueGuardEvent(t *guardTransaction, eventType, subject string, revision int64, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "风控事件无法编码")
	}
	return outbox.EnqueueEvent(t.tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: eventType, EventVersion: 1, Producer: "traffic-guard", SubjectID: subject, Revision: revision, OccurredAt: t.now.Format(time.RFC3339Nano), DataEpoch: t.op.DataEpoch, Payload: raw})
}
func readGuardPolicy(t *guardTransaction) (GuardPolicy, int64, error) {
	var raw []byte
	var revision int64
	var epoch string
	err := t.tx.QueryRowContext(t.ctx, `SELECT policy_json,revision,data_epoch FROM guard_policy WHERE site_id=?`, t.site).Scan(&raw, &revision, &epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return GuardPolicy{}, 0, platform.NewError(platform.CodeNotFound, false, "当前站点尚未保存风控策略")
	}
	if err != nil {
		return GuardPolicy{}, 0, loginDBError()
	}
	if epoch != t.op.DataEpoch {
		return GuardPolicy{}, 0, platform.NewError(platform.CodeVersionMismatch, false, "风控策略代次不一致")
	}
	var policy GuardPolicy
	if json.Unmarshal(raw, &policy) != nil {
		return GuardPolicy{}, 0, platform.NewError(platform.CodeIntegrityError, false, "风控策略损坏")
	}
	if policy.QaPerNetworkPerMinute == nil {
		rate := 600
		policy.QaPerNetworkPerMinute = &rate
	}
	return policy, revision, nil
}

func registerGuardWorkTables(ctx context.Context, db *sql.DB) error {
	for _, table := range guardWorkTables {
		if err := outbox.RegisterWorkTable(ctx, db, table); err != nil {
			return err
		}
	}
	return nil
}

// ObserveWork 在同一SQLite读事务中复用outbox revision，逐表核准入、挑战、登录
// 和排队的真实待决事实；不维护第二套水位，也不把未知数量写成零。
func (s *PolicyStore) ObserveWork(ctx context.Context) (lifecycle.SnapshotWorkState, error) {
	if s == nil || s.db == nil || !s.JournalRecovered {
		return lifecycle.SnapshotWorkState{}, platform.NewError(platform.CodeDependencyUnavailable, true, "风控工作水位依赖未恢复")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return lifecycle.SnapshotWorkState{}, platform.NewError(platform.CodeDependencyUnavailable, true, "建立风控工作观察事务失败")
	}
	defer tx.Rollback()
	state, err := outbox.NewStore(s.db).ObserveTx(ctx, tx)
	if err != nil {
		return lifecycle.SnapshotWorkState{}, err
	}
	pendingWork, err := readGuardPendingWork(ctx, tx)
	if err != nil {
		return lifecycle.SnapshotWorkState{}, err
	}
	if state.Revision < 1 || state.Pending < 0 || pendingWork < 0 {
		return lifecycle.SnapshotWorkState{}, platform.NewError(platform.CodeIntegrityError, false, "风控工作水位不完整")
	}
	return lifecycle.SnapshotWorkState{
		OutboxRevision: state.Revision,
		WorkRevision:   state.Revision,
		PendingOutbox:  state.Pending,
		PendingWork:    pendingWork,
	}, nil
}

// SealWork 在同一SQLite事务内核业务待决、CAS共享outbox revision并封口。
func (s *PolicyStore) SealWork(ctx context.Context, expected lifecycle.SnapshotWorkState, proofHash string) error {
	if s == nil || s.db == nil || expected.OutboxRevision < 1 || expected.WorkRevision != expected.OutboxRevision || expected.PendingOutbox != 0 || expected.PendingWork != 0 || !platform.ValidHash(proofHash) {
		return platform.NewError(platform.CodeValidation, false, "风控封口输入或真实待决水位不完整")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "建立风控封口事务失败")
	}
	defer tx.Rollback()
	state, err := outbox.NewStore(s.db).ObserveTx(ctx, tx)
	if err != nil {
		return err
	}
	pendingWork, err := readGuardPendingWork(ctx, tx)
	if err != nil {
		return err
	}
	if state.Revision != expected.OutboxRevision || state.Pending != expected.PendingOutbox || pendingWork != expected.PendingWork || state.Pending != 0 || pendingWork != 0 {
		return platform.NewError(platform.CodeConflict, true, "风控工作水位已改变或仍有待决")
	}
	if err := outbox.NewStore(s.db).SealTx(ctx, tx, state.Revision, proofHash); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeInternal, true, "提交风控封口事务失败")
	}
	return nil
}

// ResumeWork 在同一SQLite事务内按原摘要重开平台业务写门；不启动新业务或付费操作。
func (s *PolicyStore) ResumeWork(ctx context.Context, proofHash string) error {
	if s == nil || s.db == nil || !platform.ValidHash(proofHash) {
		return platform.NewError(platform.CodeValidation, false, "风控重开缺少原封口摘要")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "建立风控重开事务失败")
	}
	defer tx.Rollback()
	if err := outbox.NewStore(s.db).ResumeTx(ctx, tx, proofHash); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeInternal, true, "提交风控重开事务失败")
	}
	return nil
}

func readGuardPendingWork(ctx context.Context, tx platform.Tx) (int64, error) {
	var pending int64
	for _, query := range guardPendingWorkQueries {
		var count int64
		if err := tx.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return 0, platform.NewError(platform.CodeIntegrityError, false, "读取风控待决表失败")
		}
		if count < 0 || pending > int64(^uint64(0)>>1)-count {
			return 0, platform.NewError(platform.CodeIntegrityError, false, "风控待决数量溢出")
		}
		pending += count
	}
	return pending, nil
}

// NewTrafficSnapshotManager把本域真实SQLite、运行代次、文件捕获和恢复迁移
// 接入公共SnapshotManager。全域封口证明仍由jobs协调器提供，缺失时失败关闭。
func NewTrafficSnapshotManager(store *PolicyStore, dataRoot string, openedEpoch platform.Identifier, runtime platform.SiteConfigClient) (*lifecycle.SnapshotManager, error) {
	if store == nil || store.db == nil || runtime == nil || dataRoot == "" || !platform.ValidIdentifier(openedEpoch) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "风控快照装配依赖不完整")
	}
	return lifecycle.NewSnapshotManager(lifecycle.SnapshotConfig{
		Owner:            "traffic-guard",
		DataRoot:         dataRoot,
		OpenedDataEpoch:  string(openedEpoch),
		Runtime:          func(ctx context.Context) (platform.RuntimeView, error) { return lifecycle.ReadRuntime(ctx, runtime) },
		ValidateManifest: validateGuardSnapshotManifest,
		BeginDrain: func(ctx context.Context, _ *lifecycle.SnapshotCleanup) error {
			return beginGuardDrain(ctx, store)
		},
		Drain: func(ctx context.Context, _ *lifecycle.SnapshotCleanup) error {
			return drainGuardWork(ctx, store)
		},
		Watermark:  func(ctx context.Context) (int64, error) { return guardWorkWatermark(ctx, store) },
		ObserveWork: store.ObserveWork,
		SealWork:    store.SealWork,
		ResumeWork:  store.ResumeWork,
		VerifyGlobalSeal: func(context.Context, lifecycle.SnapshotSealProof, string) error {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "jobs全域封口协调证明尚未装配")
		},
		AuthorizeCleanupOrigin: func(context.Context, platform.OperationContext) error {
			return platform.NewError(platform.CodeForbidden, false, "traffic-guard没有可清理的旧付费执行")
		},
		Capture: func(ctx context.Context, cut lifecycle.SnapshotSafeCut, workspace string, input lifecycle.SnapshotExportInput) (lifecycle.DomainSnapshotManifest, error) {
			return captureGuardSnapshot(ctx, store, dataRoot, cut, workspace, input)
		},
		MigrateStage:   migrateGuardStage,
		ValidateStaged: validateGuardStaged,
		StageBudget:    guardStageBudget,
	})
}

func beginGuardDrain(ctx context.Context, store *PolicyStore) error {
	if store == nil || store.db == nil || !store.JournalRecovered {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "风控排空依赖未恢复")
	}
	if err := store.db.PingContext(ctx); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "风控业务库排空前不可用")
	}
	_, err := store.ObserveWork(ctx)
	return err
}

func drainGuardWork(ctx context.Context, store *PolicyStore) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		work, err := store.ObserveWork(ctx)
		if err != nil {
			return err
		}
		if work.PendingOutbox == 0 && work.PendingWork == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return platform.NewError(platform.CodeDeadlineExceeded, true, "风控旧工作未在屏障期限内排空")
		case <-ticker.C:
		}
	}
}

func guardWorkWatermark(ctx context.Context, store *PolicyStore) (int64, error) {
	work, err := store.ObserveWork(ctx)
	if err != nil {
		return 0, err
	}
	return work.WorkRevision, nil
}

func captureGuardSnapshot(ctx context.Context, store *PolicyStore, dataRoot string, cut lifecycle.SnapshotSafeCut, workspace string, input lifecycle.SnapshotExportInput) (lifecycle.DomainSnapshotManifest, error) {
	if input.DataEpoch != cut.Epoch || input.BarrierID != cut.BarrierID {
		return lifecycle.DomainSnapshotManifest{}, platform.NewError(platform.CodeVersionMismatch, false, "风控快照请求与安全截点不一致")
	}
	before, err := store.ObserveWork(ctx)
	if err != nil {
		return lifecycle.DomainSnapshotManifest{}, err
	}
	if before.WorkRevision != cut.Watermark || before.PendingOutbox != 0 || before.PendingWork != 0 {
		return lifecycle.DomainSnapshotManifest{}, platform.NewError(platform.CodeStaleResult, false, "风控安全截点水位已变化")
	}
	files, err := lifecycle.CaptureSnapshotFiles(ctx, filepath.Join(dataRoot, "active"), workspace, map[string]*sql.DB{policyDatabaseName: store.db}, lifecycle.SnapshotCaptureExclude{})
	if err != nil {
		return lifecycle.DomainSnapshotManifest{}, err
	}
	after, err := store.ObserveWork(ctx)
	if err != nil {
		return lifecycle.DomainSnapshotManifest{}, err
	}
	if after.WorkRevision != before.WorkRevision || after.PendingOutbox != 0 || after.PendingWork != 0 {
		return lifecycle.DomainSnapshotManifest{}, platform.NewError(platform.CodeStaleResult, false, "风控快照捕获期间业务水位已变化")
	}
	all, err := guardmigrations.All()
	if err != nil {
		return lifecycle.DomainSnapshotManifest{}, platform.NewError(platform.CodeIntegrityError, false, "风控迁移版本不可读")
	}
	return lifecycle.DomainSnapshotManifest{
		Format:            "nice-wiki.domain-snapshot",
		SchemaVersion:     1,
		Owner:             "traffic-guard",
		BackupID:          input.BackupID,
		BarrierID:         cut.BarrierID,
		SnapshotID:        input.SnapshotID,
		SourceEpoch:       cut.Epoch,
		DataSchemaVersion: len(all),
		RevisionWatermark: cut.Watermark,
		ContainsSecrets:   false,
		Files:             files,
		Objects:           []lifecycle.DomainSnapshotObject{},
		Publications:      []lifecycle.SnapshotPublishedRevisionRef{},
		References:        []lifecycle.DomainSnapshotReference{},
	}, nil
}

func validateGuardSnapshotManifest(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest lifecycle.DomainSnapshotManifest
	if err := decoder.Decode(&manifest); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "风控快照清单不可解析")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return platform.NewError(platform.CodeIntegrityError, false, "风控快照清单包含多余内容")
	}
	canonical, err := platform.CanonicalJSON(manifest)
	if err != nil || !bytes.Equal(raw, canonical) {
		return platform.NewError(platform.CodeIntegrityError, false, "风控快照清单不是规范编码")
	}
	if manifest.Format != "nice-wiki.domain-snapshot" || manifest.Owner != "traffic-guard" || manifest.SchemaVersion != 1 || manifest.Files == nil || manifest.Objects == nil || manifest.Publications == nil || manifest.References == nil || manifest.Assets != nil || manifest.Invocations != nil {
		return platform.NewError(platform.CodeUnsupportedSchema, false, "风控快照清单版本或库存不受支持")
	}
	return nil
}

func guardStageBudget(_ context.Context, manifest lifecycle.DomainSnapshotManifest) (int64, error) {
	if manifest.Owner != "traffic-guard" || manifest.SchemaVersion != 1 || len(manifest.Files) == 0 {
		return 0, platform.NewError(platform.CodeUnsupportedSchema, false, "风控只接收V1真实数据库快照")
	}
	budget := lifecycle.MaxManifestBytes
	for _, file := range manifest.Files {
		if file.ByteSize < 1 || budget > lifecycle.MaxArchiveBytes-file.ByteSize {
			return 0, platform.NewError(platform.CodeBudgetExceeded, false, "风控恢复峰值超过归档上限")
		}
		budget += file.ByteSize
	}
	if budget > lifecycle.MaxArchiveBytes || budget > lifecycle.MaxArchiveBytes/3 {
		return 0, platform.NewError(platform.CodeBudgetExceeded, false, "风控恢复峰值超过归档上限")
	}
	return budget*3, nil
}

func migrateGuardStage(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error {
	if manifest.Owner != "traffic-guard" || manifest.DataSchemaVersion < 1 || !platform.ValidIdentifier(targetEpoch) || targetEpoch == manifest.SourceEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "风控恢复目标代次或清单不合法")
	}
	stageRoot, boundEpoch, _, err := lifecycle.SnapshotStageBinding(ctx)
	if err != nil {
		return err
	}
	if stageRoot != root || boundEpoch != targetEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "风控恢复暂存代次或目录绑定已变化")
	}
	db, err := openGuardStageDB(ctx, root, false)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := outbox.MigrateRestoredDatabase(ctx, db, func(tx *sql.Tx) error {
		if err := runGuardMigrationsTx(ctx, tx); err != nil {
			return err
		}
		return validateGuardEpochs(ctx, tx, manifest.SourceEpoch)
	}); err != nil {
		return err
	}
	return registerGuardWorkTables(ctx, db)
}

func validateGuardStaged(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error {
	if manifest.Owner != "traffic-guard" || !platform.ValidIdentifier(targetEpoch) || targetEpoch == manifest.SourceEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "风控暂存代次或清单不合法")
	}
	if _, err := guardStageBudget(ctx, manifest); err != nil {
		return err
	}
	db, err := openGuardStageDB(ctx, root, true)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := validateGuardSchema(ctx, db); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "读取风控暂存代次事务失败")
	}
	if err := validateGuardEpochs(ctx, tx, manifest.SourceEpoch); err != nil {
		_ = tx.Rollback()
		return err
	}
	_ = tx.Rollback()
	observed, err := outbox.NewStore(db).Observe(ctx)
	if err != nil {
		return err
	}
	if !platform.ValidHash(observed.SealHash) || observed.Pending != 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "风控暂存outbox封口证据缺失")
	}
	return nil
}

func openGuardStageDB(ctx context.Context, root string, readOnly bool) (*sql.DB, error) {
	path := filepath.Join(root, policyDatabaseName)
	if err := lifecycle.ValidatePrivatePath(path, "regular-file", false); err != nil {
		return nil, err
	}
	mode := "rw"
	if readOnly {
		mode = "ro"
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode="+mode+"&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)")
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "风控暂存数据库不可打开")
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "风控暂存数据库不可用")
	}
	return db, nil
}

func runGuardMigrationsTx(ctx context.Context, tx *sql.Tx) error {
	all, err := guardmigrations.All()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "风控迁移集不可读")
	}
	rows, err := tx.QueryContext(ctx, `SELECT version,name FROM schema_migrations`)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "风控暂存迁移记录不可读")
	}
	applied := map[int64]string{}
	for rows.Next() {
		var version int64
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			_ = rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "风控暂存迁移记录损坏")
		}
		applied[version] = name
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return platform.NewError(platform.CodeIntegrityError, false, "风控暂存迁移记录读取失败")
	}
	_ = rows.Close()
	for version, name := range applied {
		if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
			return platform.NewError(platform.CodeUnsupportedSchema, false, "风控暂存迁移版本未知")
		}
	}
	for _, item := range all {
		if _, ok := applied[item.Version]; ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, item.SQL); err != nil {
			return platform.NewError(platform.CodeUnsupportedSchema, false, "风控暂存迁移执行失败")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,name,applied_at) VALUES(?,?,?)`, item.Version, item.Name, platform.Now()); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "风控暂存迁移登记失败")
		}
	}
	return nil
}

func validateGuardSchema(ctx context.Context, db *sql.DB) error {
	all, err := guardmigrations.All()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "风控迁移集不可读")
	}
	var applied int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil || applied != len(all) {
		return platform.NewError(platform.CodeIntegrityError, false, "风控暂存schema未完整迁移")
	}
	return nil
}

func validateGuardEpochs(ctx context.Context, tx platform.Tx, sourceEpoch string) error {
	if !platform.ValidIdentifier(sourceEpoch) {
		return platform.NewError(platform.CodeUnsupportedSchema, false, "风控源数据代次不合法")
	}
	for _, table := range guardWorkTables {
		var invalid int64
		query := `SELECT COUNT(*) FROM "` + table + `" WHERE data_epoch <> ?`
		if err := tx.QueryRowContext(ctx, query, sourceEpoch).Scan(&invalid); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "风控源数据代次不可读")
		}
		if invalid != 0 {
			return platform.NewError(platform.CodeIntegrityError, false, "风控源数据代次不一致")
		}
	}
	return nil
}
