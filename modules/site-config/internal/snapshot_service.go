package siteconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/site-config/internal/gen"
)

type SnapshotExportRequest = gen.SnapshotExportRequest
type SnapshotExportResult = gen.SnapshotExportResult
type SnapshotStageRequest = gen.SnapshotStageRequest
type SnapshotStageResult = gen.SnapshotStageResult
type SnapshotDiscardRequest = gen.SnapshotDiscardRequest
type SnapshotDiscardResult = gen.SnapshotDiscardResult

func lifecycleExportInput(op platform.OperationContext, input gen.SnapshotExportRequest) (lifecycle.SnapshotExportInput, error) {
	var converted lifecycle.SnapshotExportInput
	if err := snapshotJSONConvert(input, &converted); err != nil {
		return lifecycle.SnapshotExportInput{}, err
	}
	if converted.DataEpoch != string(op.DataEpoch) {
		return lifecycle.SnapshotExportInput{}, platform.NewError(platform.CodeVersionMismatch, false, "导出请求代次不一致")
	}
	return converted, nil
}

// ExportSnapshot 导出本域已由公共SnapshotManager证明的完整快照。
func ExportSnapshot(ctx context.Context, op platform.OperationContext, input gen.SnapshotExportRequest) (gen.SnapshotExportResult, error) {
	if err := requireSnapshotController(op); err != nil {
		return gen.SnapshotExportResult{}, err
	}
	runtime, err := snapshotRuntimeFrom(ctx)
	if err != nil {
		return gen.SnapshotExportResult{}, err
	}
	converted, err := lifecycleExportInput(op, input)
	if err != nil {
		return gen.SnapshotExportResult{}, err
	}
	result, err := runtime.API.Export(ctx, converted)
	if err != nil {
		return gen.SnapshotExportResult{}, err
	}
	var output gen.SnapshotExportResult
	if err := snapshotJSONConvert(result, &output); err != nil {
		return gen.SnapshotExportResult{}, err
	}
	return output, nil
}

// StageSnapshot 从data-protection受控归档端口读取完整流，再交给公共Manager校验/迁移/暂存。
func StageSnapshot(ctx context.Context, op platform.OperationContext, input gen.SnapshotStageRequest) (gen.SnapshotStageResult, error) {
	if err := requireSnapshotController(op); err != nil {
		return gen.SnapshotStageResult{}, err
	}
	runtime, err := snapshotRuntimeFrom(ctx)
	if err != nil {
		return gen.SnapshotStageResult{}, err
	}
	if runtime.RestoreArchive == nil {
		return gen.SnapshotStageResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复归档端口未装配")
	}
	var converted lifecycle.SnapshotStageInput
	if err := snapshotJSONConvert(input, &converted); err != nil {
		return gen.SnapshotStageResult{}, err
	}
	if converted.TargetDataEpoch == string(op.DataEpoch) {
		return gen.SnapshotStageResult{}, platform.NewError(platform.CodeVersionMismatch, false, "目标恢复代次不能等于当前代次")
	}
	source, err := runtime.RestoreArchive(ctx, converted.RestoreID, "site-config")
	if err != nil {
		return gen.SnapshotStageResult{}, err
	}
	defer source.Close()
	result, err := runtime.API.Stage(ctx, converted, source)
	if err != nil {
		return gen.SnapshotStageResult{}, err
	}
	var output gen.SnapshotStageResult
	if err := snapshotJSONConvert(result, &output); err != nil {
		return gen.SnapshotStageResult{}, err
	}
	return output, nil
}

// DiscardSnapshot 只转发公共Manager已绑定restoreId/scope的清理动作。
func DiscardSnapshot(ctx context.Context, op platform.OperationContext, input gen.SnapshotDiscardRequest) (gen.SnapshotDiscardResult, error) {
	if err := requireSnapshotController(op); err != nil {
		return gen.SnapshotDiscardResult{}, err
	}
	runtime, err := snapshotRuntimeFrom(ctx)
	if err != nil {
		return gen.SnapshotDiscardResult{}, err
	}
	var converted lifecycle.SnapshotDiscardInput
	if err := snapshotJSONConvert(input, &converted); err != nil {
		return gen.SnapshotDiscardResult{}, err
	}
	result, err := runtime.API.Discard(ctx, converted)
	if err != nil {
		return gen.SnapshotDiscardResult{}, err
	}
	var output gen.SnapshotDiscardResult
	if err := snapshotJSONConvert(result, &output); err != nil {
		return gen.SnapshotDiscardResult{}, err
	}
	return output, nil
}

var globalSealVerifier struct {
	sync.RWMutex
	verify func(context.Context, lifecycle.SnapshotSealProof, string) error
}

// ConfigureGlobalSealVerifier 只供集成装配真实全域封口核验；未装配时失败关闭。
func ConfigureGlobalSealVerifier(verify func(context.Context, lifecycle.SnapshotSealProof, string) error) {
	globalSealVerifier.Lock()
	globalSealVerifier.verify = verify
	globalSealVerifier.Unlock()
}

func verifyGlobalSeal(ctx context.Context, proof lifecycle.SnapshotSealProof, hash string) error {
	globalSealVerifier.RLock()
	verify := globalSealVerifier.verify
	globalSealVerifier.RUnlock()
	if verify == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "全域封口核验依赖未装配")
	}
	return verify(ctx, proof, hash)
}

// NewSnapshotManager 装配本域真实control/outbox/active文件回调，跨域封口由集成注入。
func NewSnapshotManager(dataRoot string, openedEpoch platform.Identifier, runtime func(context.Context) (platform.RuntimeView, error), store *ActiveStore, relayDrain func(context.Context) error) (*lifecycle.SnapshotManager, error) {
	if store == nil || runtime == nil || relayDrain == nil || dataRoot == "" || !platform.ValidIdentifier(openedEpoch) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "本域快照回调未完整装配")
	}
	manager, err := lifecycle.NewSnapshotManager(lifecycle.SnapshotConfig{
		Owner:           "site-config",
		DataRoot:        dataRoot,
		OpenedDataEpoch: string(openedEpoch),
		Runtime:         runtime,
		ValidateManifest: validateSiteSnapshotManifest,
		BeginDrain:      func(context.Context, *lifecycle.SnapshotCleanup) error { return nil },
		Drain: func(ctx context.Context, cleanup *lifecycle.SnapshotCleanup) error {
			if err := relayDrain(ctx); err != nil {
				return err
			}
			return drainSiteConfigReferences(ctx, cleanup, runtime, store)
		},
		Watermark:       func(ctx context.Context) (int64, error) { return siteConfigWatermark(ctx, store) },
		ObserveWork:     func(ctx context.Context) (lifecycle.SnapshotWorkState, error) { return observeSiteWork(ctx, store) },
		SealWork:        func(ctx context.Context, work lifecycle.SnapshotWorkState, proofHash string) error { return sealSiteWork(ctx, store, work, proofHash) },
		ResumeWork:      func(ctx context.Context, proofHash string) error { return resumeSiteWork(ctx, store, proofHash) },
		VerifyGlobalSeal: verifyGlobalSeal,
		AuthorizeCleanupOrigin: func(_ context.Context, op platform.OperationContext) error {
			if op.CallerService != "site-config" || op.Actor.Kind != "service" || op.Actor.SubjectID != "site-config" {
				return platform.NewError(platform.CodeForbidden, false, "首页旧引用清理只允许本域受信后台")
			}
			return nil
		},
		Capture:         func(ctx context.Context, cut lifecycle.SnapshotSafeCut, workspace string, input lifecycle.SnapshotExportInput) (lifecycle.DomainSnapshotManifest, error) { return captureSiteSnapshot(ctx, store, cut, workspace, input) },
		MigrateStage:    func(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error { return migrateSiteStage(ctx, manifest, targetEpoch, root) },
		ValidateStaged:  func(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error { return validateSiteStage(ctx, manifest, targetEpoch, root) },
		StageBudget:     func(ctx context.Context, manifest lifecycle.DomainSnapshotManifest) (int64, error) { return siteStageBudget(ctx, manifest) },
	})
	if err != nil {
		return nil, err
	}
	barrierReader, ok := any(manager).(snapshotBarrierStatusReader)
	if !ok {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "公共SnapshotManager缺少屏障状态读取器")
	}
	store.configureSnapshotBarrier(barrierReader)
	return manager, nil
}

func drainSiteConfigReferences(ctx context.Context, cleanup *lifecycle.SnapshotCleanup, runtime func(context.Context) (platform.RuntimeView, error), store *ActiveStore) error {
	if currentActiveStore() != store {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "首页旧引用清理未绑定当前active业务库")
	}
	view, err := runtime(ctx)
	if err != nil {
		return err
	}
	deadline := time.Now().UTC().Add(30 * time.Second)
	if current, ok := ctx.Deadline(); ok && current.Before(deadline) {
		deadline = current
	}
	id := platform.NewIdentifier()
	op := platform.OperationContext{
		RequestID: id, OperationID: id, RootOperationID: id, CallerService: "site-config",
		DataEpoch: view.DataEpoch, Deadline: platform.Timestamp(deadline.UTC().Format(time.RFC3339Nano)),
		Actor: platform.AuthContext{Kind: "service", SubjectID: "site-config", SiteID: "system"},
	}
	cleanupContext, done, err := cleanup.Context(platform.WithOperationContext(ctx, op))
	if err != nil {
		return err
	}
	defer done()
	result, err := RecoverHomeReferences(cleanupContext, op, RecoverHomeReferencesInput{})
	if err != nil {
		return err
	}
	if result.PendingCount != 0 {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "首页旧引用仍待收敛")
	}
	return nil
}

func validateSiteSnapshotManifest(raw []byte) error {
	var manifest lifecycle.DomainSnapshotManifest
	decoder := json.NewDecoder(bytesReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "站点快照清单不可解析")
	}
	if manifest.Owner != "site-config" || manifest.Format != "nice-wiki.domain-snapshot" || manifest.DataSchemaVersion != 1 || manifest.Files == nil || manifest.Objects == nil || manifest.Publications == nil || manifest.References == nil || manifest.Assets == nil || manifest.Invocations == nil {
		return platform.NewError(platform.CodeIntegrityError, false, "站点快照清单库存不完整")
	}
	return nil
}

type byteReader struct{ data []byte; offset int }

func bytesReader(data []byte) *byteReader { return &byteReader{data: data} }
func (r *byteReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

func siteConfigWatermark(ctx context.Context, store *ActiveStore) (int64, error) {
	var revision int64
	if err := store.db.QueryRowContext(ctx, `SELECT config_revision FROM site_config_state WHERE singleton=1`).Scan(&revision); err != nil {
		return 0, platform.NewError(platform.CodeDependencyUnavailable, true, "配置修订水位不可读")
	}
	if revision < 0 {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "配置修订水位不合法")
	}
	return revision, nil
}

func observeSiteWork(ctx context.Context, store *ActiveStore) (lifecycle.SnapshotWorkState, error) {
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return lifecycle.SnapshotWorkState{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开site-config工作观察事务失败")
	}
	defer tx.Rollback()
	observation, pendingWork, err := observeSiteWorkFacts(ctx, store, tx)
	if err != nil {
		return lifecycle.SnapshotWorkState{}, err
	}
	return lifecycle.SnapshotWorkState{OutboxRevision: observation.Revision, WorkRevision: observation.Revision, PendingOutbox: observation.Pending, PendingWork: pendingWork}, nil
}

func sealSiteWork(ctx context.Context, store *ActiveStore, work lifecycle.SnapshotWorkState, proofHash string) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "打开site-config工作封口事务失败")
	}
	defer tx.Rollback()
	observation, pendingWork, err := observeSiteWorkFacts(ctx, store, tx)
	if err != nil {
		return err
	}
	if observation.Revision != work.OutboxRevision || observation.Revision != work.WorkRevision || observation.Pending != work.PendingOutbox || pendingWork != work.PendingWork {
		return platform.NewError(platform.CodeConflict, true, "site-config工作水位已改变")
	}
	if observation.Pending != 0 || pendingWork != 0 || observation.Leased != 0 || observation.Quarantined != 0 {
		return platform.NewError(platform.CodeConflict, true, "site-config仍有未收敛工作")
	}
	if err := store.OutboxStore().SealTx(ctx, tx, work.OutboxRevision, proofHash); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "提交site-config工作封口失败")
	}
	return nil
}

func resumeSiteWork(ctx context.Context, store *ActiveStore, proofHash string) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "打开site-config工作重开事务失败")
	}
	defer tx.Rollback()
	if err := store.OutboxStore().ResumeTx(ctx, tx, proofHash); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "提交site-config工作重开失败")
	}
	return nil
}

func observeSiteWorkFacts(ctx context.Context, store *ActiveStore, tx *sql.Tx) (outbox.Observation, int64, error) {
	observation, err := store.OutboxStore().ObserveTx(ctx, tx)
	if err != nil {
		return outbox.Observation{}, 0, err
	}
	initialization, _, err := store.ReadInitializationRecord(ctx, tx)
	if err != nil {
		return outbox.Observation{}, 0, err
	}
	initializationPending := false
	switch initialization.Phase {
	case gen.InitializationStatePhaseUninitialized, gen.InitializationStatePhaseCompleted, gen.InitializationStatePhaseAborted:
	case gen.InitializationStatePhasePreparing, gen.InitializationStatePhaseConfigCommitted, gen.InitializationStatePhaseBlocked, gen.InitializationStatePhaseAborting:
		initializationPending = true
	default:
		return outbox.Observation{}, 0, platform.NewError(platform.CodeIntegrityError, false, "site-config初始化工作状态未知")
	}
	var pending int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT operation_id FROM config_operations WHERE state='in_progress'
			UNION
			SELECT operation_id FROM home_reference_decisions WHERE phase NOT IN ('confirmed','released')
		)`).Scan(&pending); err != nil {
		return outbox.Observation{}, 0, platform.NewError(platform.CodeDependencyUnavailable, true, "读取site-config待决工作失败")
	}
	if initializationPending {
		counted := false
		if initialization.OperationId != nil {
			var existing int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM config_operations WHERE operation_id=? AND state='in_progress'`, string(*initialization.OperationId)).Scan(&existing); err != nil {
				return outbox.Observation{}, 0, platform.NewError(platform.CodeDependencyUnavailable, true, "读取初始化待决操作失败")
			}
			counted = existing == 1
		}
		if !counted {
			pending++
		}
	}
	return observation, pending, nil
}

func captureSiteSnapshot(ctx context.Context, store *ActiveStore, cut lifecycle.SnapshotSafeCut, workspace string, input lifecycle.SnapshotExportInput) (lifecycle.DomainSnapshotManifest, error) {
	files, err := lifecycle.CaptureSnapshotFiles(ctx, store.activeRoot, workspace, map[string]*sql.DB{"site-config.db": store.db}, lifecycle.SnapshotCaptureExclude{})
	if err != nil {
		return lifecycle.DomainSnapshotManifest{}, err
	}
	manifest := lifecycle.DomainSnapshotManifest{
		Format: "nice-wiki.domain-snapshot", SchemaVersion: 2, Owner: "site-config", BackupID: input.BackupID,
		BarrierID: cut.BarrierID, SnapshotID: input.SnapshotID, SourceEpoch: cut.Epoch, DataSchemaVersion: 1,
		RevisionWatermark: cut.Watermark, ContainsSecrets: false, Files: files,
		Objects: []lifecycle.DomainSnapshotObject{}, Publications: []lifecycle.SnapshotPublishedRevisionRef{},
		References: []lifecycle.DomainSnapshotReference{}, Assets: []platform.AssetVersionRef{}, Invocations: []lifecycle.SnapshotInvocationRef{},
	}
	if err := appendSiteConfigInventory(ctx, store, &manifest); err != nil {
		return lifecycle.DomainSnapshotManifest{}, err
	}
	return manifest, nil
}

func appendSiteConfigInventory(ctx context.Context, store *ActiveStore, manifest *lifecycle.DomainSnapshotManifest) error {
	var siteID, revisionID sql.NullString
	var revision int64
	if err := store.db.QueryRowContext(ctx, `SELECT site_id,config_revision_id,config_revision FROM site_config_state WHERE singleton=1`).Scan(&siteID, &revisionID, &revision); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "配置快照水位不可读")
	}
	if !siteID.Valid || !revisionID.Valid || revision < 1 {
		return nil
	}
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "打开配置快照索引事务失败")
	}
	defer tx.Rollback()
	config, record, _, err := store.readIndexedConfig(ctx, tx, revisionID.String)
	if err != nil {
		return err
	}
	objectPath, err := filepath.Rel(store.activeRoot, configObjectPath(store.activeRoot, siteID.String, revisionID.String))
	if err != nil || objectPath == ".." || len(objectPath) > 4096 {
		return platform.NewError(platform.CodeIntegrityError, false, "配置快照对象路径不合法")
	}
	var object platform.ObjectRef
	if err := snapshotJSONConvert(record.Object, &object); err != nil {
		return err
	}
	manifest.Objects = append(manifest.Objects, lifecycle.DomainSnapshotObject{Reference: object, Path: filepath.ToSlash(objectPath)})
	for _, kind := range []string{"home", "branding"} {
		references, _, err := referenceMaterial(kind, config, record.RevisionID)
		if err != nil {
			return err
		}
		for _, reference := range references {
			var asset platform.AssetVersionRef
			if err := snapshotJSONConvert(reference.Asset, &asset); err != nil {
				return err
			}
			manifest.Assets = append(manifest.Assets, asset)
			manifest.References = append(manifest.References, lifecycle.DomainSnapshotReference{Kind: "asset", Asset: &asset})
		}
	}
	return nil
}

func openStagedActiveStore(ctx context.Context, root string) (*ActiveStore, error) {
	path := filepath.Join(root, "site-config.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)&_pragma=synchronous(FULL)")
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "打开暂存active数据库失败")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &ActiveStore{db: db, activeRoot: root}
	closeOnError := func(failure error) (*ActiveStore, error) { _ = db.Close(); return nil, failure }
	if err := db.PingContext(ctx); err != nil {
		return closeOnError(platform.NewError(platform.CodeDependencyUnavailable, true, "暂存active数据库不可用"))
	}
	return store, nil
}

func migrateSiteStage(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error {
	if manifest.Owner != "site-config" || manifest.DataSchemaVersion != 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "恢复清单不属于本域")
	}
	stageRoot, boundEpoch, _, err := lifecycle.SnapshotStageBinding(ctx)
	if err != nil {
		return err
	}
	if stageRoot != root || boundEpoch != targetEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "恢复暂存代次或目录绑定已变化")
	}
	store, err := openStagedActiveStore(ctx, root)
	if err != nil {
		return err
	}
	defer store.db.Close()
	if err := outbox.MigrateRestoredDatabase(ctx, store.db, func(tx *sql.Tx) error {
		if err := store.runActiveMigrationsTx(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE site_config_state SET data_epoch=? WHERE singleton=1`, targetEpoch); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "更新暂存配置代次失败")
		}
		if err := store.validateActiveSchemaTx(ctx, tx); err != nil {
			return err
		}
		if _, err := store.validateActiveStateWithResultTx(ctx, tx, platform.Identifier(targetEpoch)); err != nil {
			return err
		}
		return rebuildStagedAnnouncementIndex(ctx, store, tx)
	}); err != nil {
		return err
	}
	return registerSiteConfigWorkTables(ctx, store.db)
}

func rebuildStagedAnnouncementIndex(ctx context.Context, store *ActiveStore, tx *sql.Tx) error {
	entries, _, err := readAnnouncementJournal(store.activeRoot)
	if err != nil {
		return err
	}
	latest := make(map[string]announcementJournalEntry)
	for _, entry := range entries {
		if err := validateAnnouncementJournalEntry(entry); err != nil {
			return err
		}
		if entry.State != announcementStateCommitted {
			continue
		}
		key := string(entry.AnnouncementID)
		previous, exists := latest[key]
		if exists && entry.Revision < previous.Revision {
			return platform.NewError(platform.CodeIntegrityError, false, "暂存公告journal修订顺序不合法")
		}
		if exists && entry.Revision == previous.Revision && !sameAnnouncementEntry(previous, entry) {
			return platform.NewError(platform.CodeIntegrityError, false, "暂存公告journal同修订内容不一致")
		}
		latest[key] = entry
	}
	keys := make([]string, 0, len(latest))
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if _, err := tx.ExecContext(ctx, `DELETE FROM announcement_heads`); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "重建暂存公告索引失败")
	}
	for _, key := range keys {
		entry := latest[key]
		if err := validateCommittedAnnouncement(store.activeRoot, entry); err != nil {
			return err
		}
		if err := upsertAnnouncementHeadOn(ctx, tx, entry); err != nil {
			return err
		}
	}
	return nil
}

func validateSiteStage(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error {
	if manifest.Owner != "site-config" || manifest.DataSchemaVersion != 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "暂存清单不属于本域")
	}
	store, err := openStagedActiveStore(ctx, root)
	if err != nil {
		return err
	}
	defer store.db.Close()
	if err := store.validateActiveSchema(ctx); err != nil {
		return err
	}
	if _, err := store.validateActiveStateWithResult(ctx, platform.Identifier(targetEpoch)); err != nil {
		return err
	}
	return nil
}

func siteStageBudget(ctx context.Context, manifest lifecycle.DomainSnapshotManifest) (int64, error) {
	var total int64 = lifecycle.MaxManifestBytes
	for _, file := range manifest.Files {
		if file.ByteSize < 0 || total > lifecycle.MaxArchiveBytes-file.ByteSize {
			return 0, platform.NewError(platform.CodeBudgetExceeded, false, "暂存目标容量超过快照上限")
		}
		total += file.ByteSize
	}
	return total, nil
}
