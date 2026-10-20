package identity

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/identity/internal/gen"
	"nice-wiki/modules/identity/migrations"
)

type SnapshotExportRequest = gen.SnapshotExportRequest
type SnapshotExportResult = gen.SnapshotExportResult
type SnapshotStageRequest = gen.SnapshotStageRequest
type SnapshotStageResult = gen.SnapshotStageResult
type SnapshotDiscardRequest = gen.SnapshotDiscardRequest
type SnapshotDiscardResult = gen.SnapshotDiscardResult

type snapshotManagerContextKey struct{}
type snapshotRuntimeContextKey struct{}
type restoreArchiveClientContextKey struct{}

// RestoreArchiveClient是data-protection受控归档读取端口的最小接口。
type RestoreArchiveClient interface {
	ServeRestoreArchive(context.Context, string, string) (*http.Response, error)
}

func WithSnapshotManager(ctx context.Context, manager *lifecycle.SnapshotManager) context.Context {
	return context.WithValue(ctx, snapshotManagerContextKey{}, manager)
}

func WithSnapshotRuntime(ctx context.Context, runtime platform.SiteConfigClient) context.Context {
	return context.WithValue(ctx, snapshotRuntimeContextKey{}, runtime)
}

func WithRestoreArchiveClient(ctx context.Context, client RestoreArchiveClient) context.Context {
	return context.WithValue(ctx, restoreArchiveClientContextKey{}, client)
}

func snapshotManagerFromContext(ctx context.Context) (*lifecycle.SnapshotManager, error) {
	manager, ok := ctx.Value(snapshotManagerContextKey{}).(*lifecycle.SnapshotManager)
	if !ok || manager == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "身份快照管理器尚未装配")
	}
	return manager, nil
}

func snapshotRuntimeFromContext(ctx context.Context) (platform.SiteConfigClient, error) {
	runtime, ok := ctx.Value(snapshotRuntimeContextKey{}).(platform.SiteConfigClient)
	if !ok || runtime == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "身份快照运行权威尚未装配")
	}
	return runtime, nil
}

func restoreArchiveClientFromContext(ctx context.Context) (RestoreArchiveClient, error) {
	client, ok := ctx.Value(restoreArchiveClientContextKey{}).(RestoreArchiveClient)
	if !ok || client == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复归档端口尚未装配")
	}
	return client, nil
}

// ExportSnapshot把公共Manager已经完成的safe截点导出结果映射回本域OpenAPI DTO。
func ExportSnapshot(ctx context.Context, op OperationContext, input SnapshotExportRequest) (SnapshotExportResult, error) {
	if err := requireProtectionService(op); err != nil {
		return SnapshotExportResult{}, err
	}
	if input.DataEpoch != gen.Identifier(op.DataEpoch) || input.BackupId == "" || input.BarrierId == "" {
		return SnapshotExportResult{}, platform.NewError(platform.CodeVersionMismatch, false, "身份导出请求代次或屏障不一致")
	}
	manager, err := snapshotManagerFromContext(ctx)
	if err != nil {
		return SnapshotExportResult{}, err
	}
	result, err := manager.Export(ctx, lifecycle.SnapshotExportInput{
		BackupID:       string(input.BackupId),
		BarrierID:      string(input.BarrierId),
		DataEpoch:      string(input.DataEpoch),
		IncludeSecrets: input.IncludeSecrets,
	})
	if err != nil {
		return SnapshotExportResult{}, err
	}
	return gen.SnapshotExportResult{
		Checksum: gen.Hash(result.Checksum),
		Snapshot: snapshotRefFromLifecycle(result.Snapshot),
	}, nil
}

// StageSnapshot只允许data-protection携带已验证超管委托读取受控归档；
// 公共Manager在边读边hash后才把结果写入隔离staged目录。
func StageSnapshot(ctx context.Context, op OperationContext, input SnapshotStageRequest) (SnapshotStageResult, error) {
	if op.CallerService != "data-protection" || !validSuperadminDelegation(op) {
		return SnapshotStageResult{}, platform.NewError(platform.CodeForbidden, false, "恢复暂存需要data-protection及超管委托")
	}
	if input.RestoreId == "" || input.BarrierId == "" || input.TargetDataEpoch == "" || input.Checksum == "" || input.Snapshot.Owner != "identity" || input.Snapshot.DataEpoch != gen.Identifier(op.DataEpoch) {
		return SnapshotStageResult{}, platform.NewError(platform.CodeValidation, false, "身份恢复暂存绑定不完整")
	}
	client, err := restoreArchiveClientFromContext(ctx)
	if err != nil {
		return SnapshotStageResult{}, err
	}
	response, err := client.ServeRestoreArchive(ctx, string(input.RestoreId), "identity")
	if err != nil {
		return SnapshotStageResult{}, err
	}
	if response == nil || response.Body == nil {
		return SnapshotStageResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复归档响应缺失")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return SnapshotStageResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复归档读取失败")
	}
	mediaType, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/x-tar" || (response.ContentLength >= 0 && response.ContentLength != int64(input.Snapshot.Object.ByteSize)) {
		return SnapshotStageResult{}, platform.NewError(platform.CodeIntegrityError, false, "恢复归档响应元数据不符")
	}
	manager, err := snapshotManagerFromContext(ctx)
	if err != nil {
		return SnapshotStageResult{}, err
	}
	result, err := manager.Stage(ctx, lifecycle.SnapshotStageInput{
		RestoreID:       string(input.RestoreId),
		Snapshot:        snapshotRefToLifecycle(input.Snapshot),
		BarrierID:       string(input.BarrierId),
		TargetDataEpoch: string(input.TargetDataEpoch),
		Checksum:        string(input.Checksum),
	}, response.Body)
	if err != nil {
		return SnapshotStageResult{}, err
	}
	stageID := result.StageID
	manifestHash := gen.Hash(result.ManifestHash)
	return gen.SnapshotStageResult{
		RestoreId:       gen.Identifier(result.RestoreID),
		Owner:           result.Owner,
		SnapshotId:      gen.Identifier(result.SnapshotID),
		TargetDataEpoch: gen.Identifier(result.TargetDataEpoch),
		Checksum:        gen.Hash(result.Checksum),
		Staged:          result.Staged,
		StageId:         &stageID,
		ManifestHash:    &manifestHash,
	}, nil
}

// DiscardSnapshot交由公共Manager按restore journal和换目录事实判断删除范围。
func DiscardSnapshot(ctx context.Context, op OperationContext, input SnapshotDiscardRequest) (SnapshotDiscardResult, error) {
	if op.CallerService != "data-protection" || !validSuperadminDelegation(op) {
		return SnapshotDiscardResult{}, platform.NewError(platform.CodeForbidden, false, "恢复目录清理需要data-protection及超管委托")
	}
	manager, err := snapshotManagerFromContext(ctx)
	if err != nil {
		return SnapshotDiscardResult{}, err
	}
	scope, ok := input.Scope.(string)
	if !ok || (scope != "staged" && scope != "before") {
		return SnapshotDiscardResult{}, platform.NewError(platform.CodeValidation, false, "恢复目录清理范围不合法")
	}
	result, err := manager.Discard(ctx, lifecycle.SnapshotDiscardInput{RestoreID: string(input.RestoreId), Scope: scope})
	if err != nil {
		return SnapshotDiscardResult{}, err
	}
	return gen.SnapshotDiscardResult{RestoreId: gen.Identifier(result.RestoreID), Scope: result.Scope, Removed: result.Removed}, nil
}

func snapshotRefToLifecycle(input gen.SnapshotRef) lifecycle.SnapshotRef {
	return lifecycle.SnapshotRef{
		Owner:             input.Owner,
		SnapshotID:        string(input.SnapshotId),
		DataEpoch:         string(input.DataEpoch),
		DataSchemaVersion: input.DataSchemaVersion,
		RevisionWatermark: int64(input.RevisionWatermark),
		ContainsSecrets:   input.ContainsSecrets,
		Object: platform.ObjectRef{
			Owner:     input.Object.Owner,
			ObjectID:  string(input.Object.ObjectId),
			Version:   int64(input.Object.Version),
			SHA256:    string(input.Object.Sha256),
			MediaType: input.Object.MediaType,
			ByteSize:  int64(input.Object.ByteSize),
		},
	}
}

func snapshotRefFromLifecycle(input lifecycle.SnapshotRef) gen.SnapshotRef {
	return gen.SnapshotRef{
		Owner:             input.Owner,
		SnapshotId:        gen.Identifier(input.SnapshotID),
		DataEpoch:         gen.Identifier(input.DataEpoch),
		DataSchemaVersion: input.DataSchemaVersion,
		RevisionWatermark: gen.Revision(input.RevisionWatermark),
		ContainsSecrets:   input.ContainsSecrets,
		Object: gen.ObjectRef{
			Owner:     input.Object.Owner,
			ObjectId:  gen.Identifier(input.Object.ObjectID),
			Version:   gen.Revision(input.Object.Version),
			Sha256:    gen.Hash(input.Object.SHA256),
			MediaType: input.Object.MediaType,
			ByteSize:  int(input.Object.ByteSize),
		},
	}
}

// NewSnapshotManager把identity真实数据库、outbox和恢复文件回调装入同一个
// 公共Manager。全域封口证明尚待jobs协调协议；该回调失败关闭，不返回空成功。
func NewSnapshotManager(store *Store, dataRoot string, openedEpoch platform.Identifier, runtime platform.SiteConfigClient) (*lifecycle.SnapshotManager, error) {
	if store == nil || store.db == nil || runtime == nil || !platform.ValidIdentifier(openedEpoch) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "身份快照装配依赖不完整")
	}
	return lifecycle.NewSnapshotManager(lifecycle.SnapshotConfig{
		Owner:            "identity",
		DataRoot:         dataRoot,
		OpenedDataEpoch:  string(openedEpoch),
		Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(ctx, runtime)
		},
		ValidateManifest: validateIdentitySnapshotManifest,
		BeginDrain: func(ctx context.Context, _ *lifecycle.SnapshotCleanup) error {
			return beginIdentityDrain(ctx, store)
		},
		Drain: func(ctx context.Context, _ *lifecycle.SnapshotCleanup) error {
			return drainIdentityWork(ctx, store)
		},
		Watermark: func(ctx context.Context) (int64, error) {
			return identityWatermark(ctx, store)
		},
		ObserveWork: func(ctx context.Context) (lifecycle.SnapshotWorkState, error) {
			return observeIdentityWork(ctx, store)
		},
		SealWork: func(ctx context.Context, expected lifecycle.SnapshotWorkState, proofHash string) error {
			return sealIdentityWork(ctx, store, expected, proofHash)
		},
		ResumeWork: func(ctx context.Context, proofHash string) error {
			return resumeIdentityWork(ctx, store, proofHash)
		},
		VerifyGlobalSeal: func(context.Context, lifecycle.SnapshotSealProof, string) error {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "jobs全域封口协调证明尚未装配")
		},
		AuthorizeCleanupOrigin: func(context.Context, platform.OperationContext) error {
			return platform.NewError(platform.CodeForbidden, false, "identity没有可清理的旧付费执行")
		},
		Capture: func(ctx context.Context, cut lifecycle.SnapshotSafeCut, workspace string, input lifecycle.SnapshotExportInput) (lifecycle.DomainSnapshotManifest, error) {
			return captureIdentitySnapshot(ctx, store, dataRoot, cut, workspace, input)
		},
		MigrateStage:    migrateIdentityStage,
		ValidateStaged:  validateIdentityStaged,
		StageBudget:     identityStageBudget,
	})
}

func beginIdentityDrain(ctx context.Context, store *Store) error {
	if err := outbox.EnsureSchema(ctx, store.db); err != nil {
		return err
	}
	if err := store.db.PingContext(ctx); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "身份业务库排空前不可用")
	}
	_, err := outbox.NewStore(store.db).Observe(ctx)
	return err
}

func drainIdentityWork(ctx context.Context, store *Store) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		work, err := observeIdentityWork(ctx, store)
		if err != nil {
			return err
		}
		if work.PendingOutbox == 0 && work.PendingWork == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return platform.NewError(platform.CodeDeadlineExceeded, true, "身份旧工作未在屏障期限内排空")
			}
			return platform.NewError(platform.CodeCancelled, true, "身份屏障排空已取消")
		case <-ticker.C:
		}
	}
}

func observeIdentityWork(ctx context.Context, store *Store) (lifecycle.SnapshotWorkState, error) {
	if err := outbox.EnsureSchema(ctx, store.db); err != nil {
		return lifecycle.SnapshotWorkState{}, err
	}
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return lifecycle.SnapshotWorkState{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份工作水位事务不可用")
	}
	defer tx.Rollback()
	observed, err := outbox.NewStore(store.db).ObserveTx(ctx, tx)
	if err != nil {
		return lifecycle.SnapshotWorkState{}, err
	}
	workRevision, err := readIdentityWorkRevision(ctx, store, tx)
	if err != nil {
		return lifecycle.SnapshotWorkState{}, err
	}
	pendingWork, err := readIdentityPendingWorkQuery(ctx, tx, store)
	if err != nil {
		return lifecycle.SnapshotWorkState{}, err
	}
	return lifecycle.SnapshotWorkState{
		OutboxRevision: observed.Revision,
		WorkRevision:   workRevision,
		PendingOutbox:  observed.Pending,
		PendingWork:    pendingWork,
	}, nil
}

func readIdentityWorkRevision(ctx context.Context, store *Store, tx platform.Tx) (int64, error) {
	if store == nil || store.db == nil || tx == nil {
		return 0, platform.NewError(platform.CodeDependencyUnavailable, true, "身份workRevision事务未装配")
	}
	observed, err := outbox.NewStore(store.db).ObserveTx(ctx, tx)
	if err != nil {
		return 0, err
	}
	return observed.Revision, nil
}

func readIdentityPendingWorkQuery(ctx context.Context, query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, store *Store) (int64, error) {
	var pending int64
	if err := query.QueryRowContext(ctx, `SELECT COUNT(*) FROM operation_receipts WHERE state <> 'committed'`).Scan(&pending); err != nil {
		return 0, platform.NewError(platform.CodeDependencyUnavailable, true, "身份未决回执不可读")
	}
	var reserved int64
	if err := store.control.QueryRowContext(ctx, `SELECT COUNT(*) FROM bootstrap_claim WHERE state = 'reserved'`).Scan(&reserved); err != nil {
		return 0, platform.NewError(platform.CodeDependencyUnavailable, true, "身份注册决定不可读")
	}
	entries, err := readJournal(store.journal)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if entry.State == "open" {
			pending++
		}
	}
	return pending + reserved, nil
}

func identityWatermark(ctx context.Context, store *Store) (int64, error) {
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, platform.NewError(platform.CodeDependencyUnavailable, true, "身份水位事务不可用")
	}
	defer tx.Rollback()
	return readIdentityWorkRevision(ctx, store, tx)
}

func sealIdentityWork(ctx context.Context, store *Store, expected lifecycle.SnapshotWorkState, proofHash string) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "身份封口事务不可用")
	}
	defer tx.Rollback()
	storeOutbox := outbox.NewStore(store.db)
	observed, err := storeOutbox.ObserveTx(ctx, tx)
	if err != nil {
		return err
	}
	workRevision, err := readIdentityWorkRevision(ctx, store, tx)
	if err != nil {
		return err
	}
	pendingWork, err := readIdentityPendingWorkQuery(ctx, tx, store)
	if err != nil {
		return err
	}
	if observed.Revision != expected.OutboxRevision || observed.Pending != 0 || observed.Leased != 0 || observed.Quarantined != 0 || workRevision != expected.WorkRevision || pendingWork != expected.PendingWork {
		return platform.NewError(platform.CodeConflict, true, "身份持久工作水位已变化")
	}
	if err := storeOutbox.SealTx(ctx, tx, observed.Revision, proofHash); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "身份封口事务提交失败")
	}
	return nil
}

func resumeIdentityWork(ctx context.Context, store *Store, proofHash string) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "身份重开事务不可用")
	}
	defer tx.Rollback()
	if err := outbox.NewStore(store.db).ResumeTx(ctx, tx, proofHash); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "身份重开事务提交失败")
	}
	return nil
}

func captureIdentitySnapshot(ctx context.Context, store *Store, dataRoot string, cut lifecycle.SnapshotSafeCut, workspace string, input lifecycle.SnapshotExportInput) (lifecycle.DomainSnapshotManifest, error) {
	if input.DataEpoch != cut.Epoch || input.BarrierID != cut.BarrierID {
		return lifecycle.DomainSnapshotManifest{}, platform.NewError(platform.CodeVersionMismatch, false, "身份快照请求与安全截点不一致")
	}
	before, err := identityWatermark(ctx, store)
	if err != nil {
		return lifecycle.DomainSnapshotManifest{}, err
	}
	if before != cut.Watermark {
		return lifecycle.DomainSnapshotManifest{}, platform.NewError(platform.CodeStaleResult, false, "身份安全截点水位已变化")
	}
	if err := outbox.EnsureSchema(ctx, store.db); err != nil {
		return lifecycle.DomainSnapshotManifest{}, err
	}
	files, err := lifecycle.CaptureSnapshotFiles(ctx, filepath.Join(dataRoot, "active"), workspace, map[string]*sql.DB{"identity.db": store.db}, lifecycle.SnapshotCaptureExclude{})
	if err != nil {
		return lifecycle.DomainSnapshotManifest{}, err
	}
	after, err := identityWatermark(ctx, store)
	if err != nil {
		return lifecycle.DomainSnapshotManifest{}, err
	}
	if after != before {
		return lifecycle.DomainSnapshotManifest{}, platform.NewError(platform.CodeStaleResult, false, "身份快照捕获期间业务水位已变化")
	}
	all, err := migrations.All()
	if err != nil {
		return lifecycle.DomainSnapshotManifest{}, platform.NewError(platform.CodeIntegrityError, false, "身份迁移版本不可读")
	}
	return lifecycle.DomainSnapshotManifest{
		Format:            "nice-wiki.domain-snapshot",
		SchemaVersion:     1,
		Owner:             "identity",
		BackupID:          input.BackupID,
		BarrierID:         cut.BarrierID,
		SnapshotID:        input.SnapshotID,
		SourceEpoch:       cut.Epoch,
		DataSchemaVersion: len(all),
		RevisionWatermark: cut.Watermark,
		ContainsSecrets:   input.IncludeSecrets,
		Files:             files,
		Objects:           []lifecycle.DomainSnapshotObject{},
		Publications:      []lifecycle.SnapshotPublishedRevisionRef{},
		References:        []lifecycle.DomainSnapshotReference{},
		Assets:            []platform.AssetVersionRef{},
		Invocations:       []lifecycle.SnapshotInvocationRef{},
	}, nil
}

func validateIdentitySnapshotManifest(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest lifecycle.DomainSnapshotManifest
	if err := decoder.Decode(&manifest); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "身份快照清单字段不合法")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return platform.NewError(platform.CodeIntegrityError, false, "身份快照清单包含多余内容")
	}
	canonical, err := platform.CanonicalJSON(manifest)
	if err != nil || !bytes.Equal(raw, canonical) {
		return platform.NewError(platform.CodeIntegrityError, false, "身份快照清单不是规范编码")
	}
	if manifest.Format != "nice-wiki.domain-snapshot" || manifest.Owner != "identity" || manifest.SchemaVersion != 1 || manifest.Files == nil || manifest.Objects == nil || manifest.Publications == nil || manifest.References == nil || manifest.Assets != nil || manifest.Invocations != nil {
		return platform.NewError(platform.CodeUnsupportedSchema, false, "身份快照清单版本或库存不受支持")
	}
	return nil
}

func identityStageBudget(ctx context.Context, manifest lifecycle.DomainSnapshotManifest) (int64, error) {
	if manifest.Owner != "identity" || manifest.SchemaVersion != 1 || len(manifest.Files) != 1 || manifest.Files[0].Path != "identity.db" {
		return 0, platform.NewError(platform.CodeUnsupportedSchema, false, "身份只接收V1独立数据库快照")
	}
	raw, err := platform.CanonicalJSON(manifest)
	if err != nil {
		return 0, err
	}
	size := manifest.Files[0].ByteSize
	if size < 1 || size > lifecycle.MaxArchiveBytes {
		return 0, platform.NewError(platform.CodeBudgetExceeded, false, "身份快照数据库超过归档预算")
	}
	budget := size*3 + int64(len(raw))*2 + 4<<20
	if budget > lifecycle.MaxArchiveBytes {
		return 0, platform.NewError(platform.CodeBudgetExceeded, false, "身份恢复迁移峰值超过20GiB")
	}
	return budget, nil
}

func migrateIdentityStage(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error {
	boundRoot, boundEpoch, _, err := lifecycle.SnapshotStageBinding(ctx)
	if err != nil {
		return err
	}
	if boundRoot != root || boundEpoch != targetEpoch {
		return platform.NewError(platform.CodeIntegrityError, false, "身份恢复stage scope不一致")
	}
	if _, err := identityStageBudget(ctx, manifest); err != nil {
		return err
	}
	db, err := openIdentitySnapshotDB(ctx, filepath.Join(root, "identity.db"), false)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrateIdentitySnapshotDB(ctx, db); err != nil {
		return err
	}
	return validateIdentitySnapshotDatabase(ctx, db, manifest.SourceEpoch, manifest.DataSchemaVersion)
}

func validateIdentityStaged(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error {
	if !platform.ValidIdentifier(targetEpoch) || targetEpoch == manifest.SourceEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "身份恢复目标代次不合法")
	}
	if _, err := identityStageBudget(ctx, manifest); err != nil {
		return err
	}
	db, err := openIdentitySnapshotDB(ctx, filepath.Join(root, "identity.db"), true)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := validateIdentitySnapshotDatabase(ctx, db, manifest.SourceEpoch, manifest.DataSchemaVersion); err != nil {
		return err
	}
	observed, err := outbox.NewStore(db).Observe(ctx)
	if err != nil {
		return err
	}
	if !platform.ValidHash(observed.SealHash) {
		return platform.NewError(platform.CodeIntegrityError, false, "身份恢复outbox封口证据缺失")
	}
	return nil
}

func openIdentitySnapshotDB(ctx context.Context, path string, readOnly bool) (*sql.DB, error) {
	if err := lifecycle.ValidatePrivatePath(path, "regular-file", false); err != nil {
		return nil, err
	}
	mode := "rw"
	if readOnly {
		mode = "ro"
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode="+mode+"&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)")
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "身份恢复数据库不可打开")
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "身份恢复数据库不可用")
	}
	if !readOnly {
		if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=DELETE"); err != nil {
			_ = db.Close()
			return nil, platform.NewError(platform.CodeIntegrityError, false, "身份恢复数据库日志模式不受支持")
		}
	}
	return db, nil
}

func migrateIdentitySnapshotDB(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "身份恢复数据库未装配")
	}
	if err := outbox.MigrateRestoredDatabase(ctx, db, func(tx *sql.Tx) error {
		all, err := migrations.All()
		if err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "身份迁移版本不可读")
		}
		applied := map[int64]string{}
		rows, err := tx.QueryContext(ctx, `SELECT version, name FROM schema_migrations`)
		if err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "身份恢复迁移记录不可读")
		}
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				_ = rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "身份恢复迁移记录损坏")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "身份恢复迁移记录读取失败")
		}
		_ = rows.Close()
		for version, name := range applied {
			if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
				return platform.NewError(platform.CodeUnsupportedSchema, false, "身份恢复迁移版本未知")
			}
		}
		for _, item := range all {
			if _, ok := applied[item.Version]; ok {
				continue
			}
			if _, err := tx.ExecContext(ctx, item.SQL); err != nil {
				return platform.NewError(platform.CodeUnsupportedSchema, false, "身份恢复迁移执行失败")
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,name,applied_at) VALUES(?,?,?)`, item.Version, item.Name, platform.Now()); err != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "身份恢复迁移记录失败")
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, table := range identityWorkTables {
		if err := outbox.RegisterWorkTable(ctx, db, table); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "恢复数据库业务表work gate登记失败")
		}
	}
	return nil
}

func validateIdentitySnapshotDatabase(ctx context.Context, db *sql.DB, sourceEpoch string, sourceSchema int) error {
	all, err := migrations.All()
	if err != nil || !platform.ValidIdentifier(sourceEpoch) || sourceSchema < 1 || sourceSchema > len(all) {
		return platform.NewError(platform.CodeUnsupportedSchema, false, "身份数据schema不受支持")
	}
	var applied int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil || applied != len(all) {
		return platform.NewError(platform.CodeIntegrityError, false, "身份恢复schema未完整迁移")
	}
	var accounts, superadmins int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(is_superadmin),0) FROM accounts`).Scan(&accounts, &superadmins); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "身份账户权威不可读")
	}
	if superadmins > 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "身份唯一超管权威不一致")
	}
	var sessionFamilies int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_families`).Scan(&sessionFamilies); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "身份会话族权威不可读")
	}
	var authEpoch string
	authErr := db.QueryRowContext(ctx, `SELECT auth_epoch FROM auth_state WHERE singleton=1`).Scan(&authEpoch)
	if errors.Is(authErr, sql.ErrNoRows) {
		if accounts > 0 || sessionFamilies > 0 {
			return platform.NewError(platform.CodeIntegrityError, false, "身份认证代次权威缺失")
		}
		return nil
	}
	if authErr != nil || !platform.ValidIdentifier(authEpoch) {
		return platform.NewError(platform.CodeIntegrityError, false, "身份认证代次不可读")
	}
	var invalidFamilies int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_families WHERE auth_epoch <> ? OR auth_epoch = ''`, authEpoch).Scan(&invalidFamilies); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "身份会话认证代次不可读")
	}
	if invalidFamilies != 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "身份会话认证代次不一致")
	}
	var invalidDataEpochs int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_families WHERE data_epoch = '' OR data_epoch <> ?`, sourceEpoch).Scan(&invalidDataEpochs); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "身份会话数据代次不可读")
	}
	if invalidDataEpochs != 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "身份会话数据代次不一致")
	}
	return nil
}
