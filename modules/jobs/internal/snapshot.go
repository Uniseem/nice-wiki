package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/jobs/internal/gen"
	jobsmigrations "nice-wiki/modules/jobs/migrations"
)

type SnapshotExportRequest = gen.SnapshotExportRequest
type SnapshotExportResult = gen.SnapshotExportResult
type SnapshotStageRequest = gen.SnapshotStageRequest
type SnapshotStageResult = gen.SnapshotStageResult
type SnapshotDiscardRequest = gen.SnapshotDiscardRequest
type SnapshotDiscardResult = gen.SnapshotDiscardResult

// ValidateJobsSnapshotManifest核验jobs域快照清单的真实库存闭包；不把未知库存当空数组。
func ValidateJobsSnapshotManifest(raw []byte) error {
	var manifest lifecycle.DomainSnapshotManifest
	if json.Unmarshal(raw, &manifest) != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs快照清单不可解码")
	}
	return ValidateJobsSnapshotManifestValue(manifest)
}

func ValidateJobsSnapshotManifestValue(manifest lifecycle.DomainSnapshotManifest) error {
	if manifest.Format != "nice-wiki.domain-snapshot" || (manifest.SchemaVersion != 1 && manifest.SchemaVersion != 2) || manifest.Owner != "jobs" || !platform.ValidIdentifier(manifest.BackupID) || !platform.ValidIdentifier(manifest.SnapshotID) || !platform.ValidIdentifier(manifest.SourceEpoch) || manifest.DataSchemaVersion < 1 || manifest.RevisionWatermark < 0 {
		return platform.NewError(platform.CodeUnsupportedSchema, false, "jobs快照清单版本或归属不符")
	}
	if manifest.Files == nil || manifest.Objects == nil || manifest.Publications == nil || manifest.References == nil || (manifest.SchemaVersion == 2 && (manifest.Assets == nil || manifest.Invocations == nil)) {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs快照清单真实库存缺失")
	}
	for _, file := range manifest.Files {
		clean := filepath.ToSlash(filepath.Clean(file.Path))
		if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || clean != file.Path || !platform.ValidHash(file.SHA256) || file.ByteSize < 0 {
			return platform.NewError(platform.CodeIntegrityError, false, "jobs快照文件清单不合法")
		}
	}
	return nil
}

// DrainJobs只等待仍持有执行权或outbox行租约的真实活动；queued/retry-wait事实保留，不伪造已安全。
func DrainJobs(ctx context.Context, store *Store) error {
	if store == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	db, err := store.SnapshotDatabase()
	if err != nil {
		return err
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "开始任务活动观察事务失败")
		}
		var leases int64
		observation, err := outbox.NewStore(db).ObserveTx(ctx, tx)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM leases`).Scan(&leases); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeDependencyUnavailable, true, "读取任务活动租约失败")
		}
		if err := tx.Commit(); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "提交任务活动观察事务失败")
		}
		if leases == 0 && observation.Leased == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return platform.NewError(platform.CodeDeadlineExceeded, true, "任务活动尚未到达安全点")
		case <-ticker.C:
		}
	}
}

// MigrateJobsStage只恢复可重建的outbox活动资格；保留任务、事件、费用、根操作和原dataEpoch事实。
func MigrateJobsStage(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error {
	if err := ValidateJobsSnapshotManifestValue(manifest); err != nil {
		return err
	}
	if !platform.ValidIdentifier(targetEpoch) || targetEpoch == manifest.SourceEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "jobs恢复目标代次必须区别于源代次")
	}
	path := filepath.Join(root, "jobs.db")
	if filepath.Clean(path) != path || !filepath.IsAbs(path) {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存数据库路径不合法")
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=rw&_pragma=foreign_keys(ON)")
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "打开jobs暂存数据库失败")
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存数据库不可用")
	}
	db.SetMaxOpenConns(1)
	if err := outbox.MigrateRestoredDatabase(ctx, db, func(tx *sql.Tx) error {
		if err := runRestoredJobsMigrations(ctx, tx); err != nil {
			return err
		}
		return revokeRestoredEligibility(ctx, tx)
	}); err != nil {
		return err
	}
	return validateJobsPreservedFacts(ctx, db, targetEpoch)
}

func ValidateJobsStaged(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error {
	if err := ValidateJobsSnapshotManifestValue(manifest); err != nil {
		return err
	}
	path := filepath.Join(root, "jobs.db")
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=query_only(ON)")
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存数据库不可读")
	}
	defer db.Close()
	var check string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&check); err != nil || check != "ok" {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存数据库完整性失败")
	}
	return validateJobsPreservedFacts(ctx, db, targetEpoch)
}

func revokeRestoredEligibility(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM leases`); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "撤销jobs旧租约失败")
	}
	return nil
}

// runRestoredJobsMigrations 在公共恢复事务内执行本域已登记的追加迁移，
// 只补schema与可重建索引，不改写已有业务行的epoch、payload、根或费用引用。
func runRestoredJobsMigrations(ctx context.Context, tx *sql.Tx) error {
	all, err := jobsmigrations.All()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "任务恢复迁移集不合法")
	}
	applied := map[int64]string{}
	rows, err := tx.QueryContext(ctx, `SELECT version, name FROM schema_migrations`)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "读取任务恢复迁移记录失败")
	}
	for rows.Next() {
		var version int64
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			_ = rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "任务恢复迁移记录损坏")
		}
		applied[version] = name
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return platform.NewError(platform.CodeIntegrityError, false, "读取任务恢复迁移记录失败")
	}
	_ = rows.Close()
	for version, name := range applied {
		if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
			return platform.NewError(platform.CodeIntegrityError, false, "任务恢复迁移版本未知")
		}
	}
	for _, migration := range all {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "执行任务恢复迁移失败")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`, migration.Version, migration.Name, platform.Now()); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "登记任务恢复迁移失败")
		}
	}
	return nil
}

func validateJobsPreservedFacts(ctx context.Context, db *sql.DB, targetEpoch string) error {
	if !platform.ValidIdentifier(targetEpoch) {
		return platform.NewError(platform.CodeVersionMismatch, false, "jobs恢复代次绑定不合法")
	}
	for _, table := range []string{"jobs", "events", "schedules", "consumer_offsets", "event_operations", "lease_operations", "job_management_operations", "schedule_operations", "barriers"} {
		var targetRows int64
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE data_epoch=?`, targetEpoch).Scan(&targetRows); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存代次字段缺失")
		}
		if targetRows != 0 {
			return platform.NewError(platform.CodeVersionMismatch, false, "jobs暂存错误引入目标代次业务事实")
		}
	}
	var unknown int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nw_outbox WHERE state NOT IN ('relayed','discarded')`).Scan(&unknown); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存outbox活动状态不可核验")
	}
	if unknown != 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存outbox仍含可重放活动")
	}
	var targetOutbox int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nw_outbox WHERE data_epoch=?`, targetEpoch).Scan(&targetOutbox); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存outbox目标代次不可核验")
	}
	if targetOutbox != 0 {
		return platform.NewError(platform.CodeVersionMismatch, false, "jobs暂存错误引入目标代次outbox事实")
	}
	var leases int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM leases`).Scan(&leases); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存旧租约不可核验")
	}
	if leases != 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存仍含旧执行资格")
	}
	if err := validatePreservedEvents(ctx, db); err != nil {
		return err
	}
	return nil
}

func validatePreservedEvents(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT event_id,envelope_json,envelope_hash,data_epoch,producer,event_type,event_version,subject_id,source_revision FROM events`)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "读取jobs暂存事件事实失败")
	}
	defer rows.Close()
	for rows.Next() {
		var eventID, epoch, producer, eventType, subjectID, storedHash string
		var raw []byte
		var version int
		var revision int64
		if err := rows.Scan(&eventID, &raw, &storedHash, &epoch, &producer, &eventType, &version, &subjectID, &revision); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "读取jobs暂存事件事实失败")
		}
		var envelope platform.EventEnvelope
		if json.Unmarshal(raw, &envelope) != nil || envelope.EventID != eventID || envelope.DataEpoch != epoch || envelope.Producer != producer || envelope.EventType != eventType || envelope.EventVersion != version || envelope.SubjectID != subjectID || envelope.Revision != platform.Revision(revision) {
			return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存事件信封与索引不一致")
		}
		if storedHash != "" && platform.HashBytes(raw) != platform.Hash(storedHash) {
			return platform.NewError(platform.CodeIntegrityError, false, "jobs暂存事件信封哈希改变")
		}
	}
	if err := rows.Err(); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "遍历jobs暂存事件事实失败")
	}
	return nil
}

func JobsStageBudget(manifest lifecycle.DomainSnapshotManifest) (int64, error) {
	if err := ValidateJobsSnapshotManifestValue(manifest); err != nil {
		return 0, err
	}
	var total int64
	for _, file := range manifest.Files {
		if file.ByteSize > math.MaxInt64-total {
			return 0, platform.NewError(platform.CodeBudgetExceeded, false, "jobs暂存空间预算溢出")
		}
		total += file.ByteSize
	}
	if total < 1 || total > lifecycle.MaxArchiveBytes/2 {
		return 0, platform.NewError(platform.CodeBudgetExceeded, false, "jobs暂存空间预算未知或超过上限")
	}
	return total * 2, nil
}

func snapshotRefToLifecycle(ref gen.SnapshotRef) lifecycle.SnapshotRef {
	return lifecycle.SnapshotRef{Owner: ref.Owner, SnapshotID: ref.SnapshotId, DataEpoch: ref.DataEpoch, DataSchemaVersion: ref.DataSchemaVersion, Object: platform.ObjectRef{Owner: ref.Object.Owner, ObjectID: ref.Object.ObjectId, Version: int64(ref.Object.Version), SHA256: ref.Object.Sha256, MediaType: ref.Object.MediaType, ByteSize: int64(ref.Object.ByteSize)}, RevisionWatermark: int64(ref.RevisionWatermark), ContainsSecrets: ref.ContainsSecrets}
}

func snapshotRefFromLifecycle(ref lifecycle.SnapshotRef) gen.SnapshotRef {
	return gen.SnapshotRef{Owner: ref.Owner, SnapshotId: ref.SnapshotID, DataEpoch: ref.DataEpoch, DataSchemaVersion: ref.DataSchemaVersion, Object: gen.ObjectRef{Owner: ref.Object.Owner, ObjectId: ref.Object.ObjectID, Version: gen.Revision(ref.Object.Version), Sha256: ref.Object.SHA256, MediaType: ref.Object.MediaType, ByteSize: int(ref.Object.ByteSize)}, RevisionWatermark: gen.Revision(ref.RevisionWatermark), ContainsSecrets: ref.ContainsSecrets}
}

func CreateSnapshot(ctx context.Context, op OperationContext, request SnapshotExportRequest) (SnapshotExportResult, error) {
	if op.CallerService != "data-protection" {
		return SnapshotExportResult{}, platform.NewError(platform.CodeForbidden, false, "仅data-protection可以导出jobs快照")
	}
	if request.DataEpoch != op.DataEpoch {
		return SnapshotExportResult{}, platform.NewError(platform.CodeVersionMismatch, false, "快照导出代次不符")
	}
	store := currentStore()
	if store == nil {
		return SnapshotExportResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	manager, _, err := store.snapshotClients()
	if err != nil {
		return SnapshotExportResult{}, err
	}
	result, err := manager.Export(ctx, lifecycle.SnapshotExportInput{BackupID: request.BackupId, BarrierID: request.BarrierId, DataEpoch: request.DataEpoch, IncludeSecrets: request.IncludeSecrets})
	if err != nil {
		return SnapshotExportResult{}, err
	}
	return SnapshotExportResult{Snapshot: snapshotRefFromLifecycle(result.Snapshot), Checksum: result.Checksum}, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func NewSnapshotHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"exportSnapshot": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.SnapshotExportRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "快照导出请求不可读"), 0)
				return
			}
			result, err := CreateSnapshot(r.Context(), op, body)
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"stageSnapshot": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.SnapshotStageRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "快照暂存请求不可读"), 0)
				return
			}
			result, err := StageSnapshot(r.Context(), op, body)
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"discardSnapshot": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.SnapshotDiscardRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "快照丢弃请求不可读"), 0)
				return
			}
			result, err := DiscardSnapshot(r.Context(), op, body)
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"readSnapshotArchive": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			byteSize, parseErr := strconv.ParseInt(r.URL.Query().Get("byteSize"), 10, 64)
			result, err := ReadSnapshotArchive(r.Context(), op, SnapshotArchiveReadInput{SnapshotID: r.PathValue("snapshotId"), SHA256: r.URL.Query().Get("sha256"), ByteSize: byteSize})
			if parseErr != nil && err == nil {
				err = platform.NewError(platform.CodeValidation, false, "归档大小不合法")
			}
			if err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
			defer result.Reader.Close()
			w.Header().Set("Content-Type", "application/x-tar")
			w.Header().Set("Content-Length", strconv.FormatInt(result.Object.ByteSize, 10))
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, result.Reader)
		}),
	}
}
