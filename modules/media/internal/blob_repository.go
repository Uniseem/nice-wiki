package media

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "modernc.org/sqlite" // 注册无CGO的 sqlite 驱动（ADR0005）。

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/inbox"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/media/internal/gen"
	mediamigrations "nice-wiki/modules/media/migrations"
)

// CommitAssetBlobInput 只由已校验完成链构造，不由HTTP直接解码。
type CommitAssetBlobInput struct {
	UploadID         Identifier   `json:"uploadId"`
	SHA256           Hash         `json:"sha256"`
	MediaType        string       `json:"mediaType"`
	ExpectedRevision gen.Revision `json:"expectedRevision"`
	RequestHash      Hash         `json:"requestHash"`
	Animated         bool         `json:"animated"`
	Width            *int         `json:"width"`
	Height           *int         `json:"height"`
}
type CommitAssetBlobResult struct {
	Asset gen.Asset `json:"asset"`
}
type blobStore interface {
	OpenUpload(context.Context, Identifier) (ioReadCloser, error)
}

// ioReadCloser 只保留 HashUpload 所需的受限读取能力，避免把文件句柄或路径
// 泄露到函数合同之外。
type ioReadCloser interface {
	Read([]byte) (int, error)
	Close() error
}

type uploadQuota struct {
	MaxUploadBytes       int
	SiteStorageBytes     int64
	MaxUploadConcurrency int
}

const (
	mediaDatabaseName    = "media.db"
	blobJournalPrepared  = "PREPARED"
	blobJournalCommitted = "COMMITTED"
	blobJournalDeleting  = "DELETING"
)

// localBlobStore 负责 media active 区的私有 SQLite 元数据库与不可变文件。
// 数据库单连接串行写入；mu 串行化同进程的提交，使“写文件→元数据事务”
// 不与另一提交交错。所有路径都由受约束的 ID/hash 生成。
type localBlobStore struct {
	root                      string
	db                        *sql.DB
	mu                        sync.Mutex
	openedEpoch               Identifier
	window                    platform.WriteWindowConfig
	validateAsset             func(json.RawMessage) error
	validateUpload            func(json.RawMessage) error
	validatePin               func(json.RawMessage) error
	validateReferenceInput    func(json.RawMessage) error
	validateDecision          func(json.RawMessage) error
	validateContentPage       func(json.RawMessage) error
	validateConfigPage        func(json.RawMessage) error
	validateSnapshot          func(json.RawMessage) error
	validateCleanupPlan       func(json.RawMessage) error
	validateCleanupApproval   func(json.RawMessage) error
	validateCleanupJobReceipt func(json.RawMessage) error
	cleanupJobs               *gen.JobsClient
	listingKey                *platform.SigningKey
	contentDecisions          *gen.ContentClient
	configDecisions           *gen.SiteConfigClient
	referenceLocksMu          sync.Mutex
	referenceLocks            map[string]*referenceLock
	chunkRecovered            bool
	blobRecovered             bool
}

// activeBlobStore 由本域存储装配在接受业务请求前设置；nil 时所有依赖该
// 存储的函数都返回 DEPENDENCY_UNAVAILABLE，不能伪造提交成功。
var (
	activeBlobStore   blobStore
	activeBlobStoreMu sync.RWMutex
)

// InstallLocalBlobStore 打开本域 active 私有目录与 SQLite 元数据库并装配
// 不可变 blob 边界。该入口只接受服务配置生成的数据根，不接受请求携带的路径。
func InstallLocalBlobStore(dataRoot string, openedEpoch Identifier) error {
	if !filepath.IsAbs(dataRoot) {
		return platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	store, err := openLocalBlobStore(filepath.Join(dataRoot, "active"), openedEpoch)
	if err != nil {
		return err
	}
	activeBlobStoreMu.Lock()
	previous := activeBlobStore
	activeBlobStore = store
	activeBlobStoreMu.Unlock()
	if old, ok := previous.(*localBlobStore); ok && old != store {
		_ = old.Close()
	}
	return nil
}

func currentBlobStore() blobStore {
	activeBlobStoreMu.RLock()
	defer activeBlobStoreMu.RUnlock()
	return activeBlobStore
}

func openLocalBlobStore(activeRoot string, openedEpoch Identifier) (*localBlobStore, error) {
	if !filepath.IsAbs(activeRoot) {
		return nil, platform.NewError(platform.CodeValidation, false, "媒体存储根必须为绝对路径")
	}
	if !platform.ValidIdentifier(openedEpoch) {
		return nil, platform.NewError(platform.CodeValidation, false, "媒体活动代次缺失")
	}
	if err := ensureMediaDirectory(activeRoot); err != nil {
		return nil, err
	}
	root := filepath.Join(activeRoot, "media")
	for _, directory := range []string{"blobs", "uploads", "staging", "isolated"} {
		if err := ensureMediaDirectory(filepath.Join(root, directory)); err != nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "建立媒体私有目录失败")
		}
	}
	databasePath := filepath.Join(activeRoot, mediaDatabaseName)
	if err := lifecycle.ValidatePrivatePath(databasePath, "regular-file", true); err != nil {
		return nil, err
	}
	databaseBefore, statErr := os.Lstat(databasePath)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体数据库路径不可读")
	}
	dsn := "file:" + databasePath +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(3000)" +
		"&_pragma=synchronous(FULL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开媒体元数据库失败")
	}
	db.SetMaxOpenConns(1)
	store := &localBlobStore{root: root, db: db, openedEpoch: openedEpoch}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体元数据库不可用")
	}
	if err := lifecycle.ValidatePrivatePath(databasePath, "regular-file", false); err != nil {
		_ = db.Close()
		return nil, err
	}
	databaseAfter, err := os.Lstat(databasePath)
	if err != nil || (databaseBefore != nil && !os.SameFile(databaseBefore, databaseAfter)) {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "媒体数据库打开后身份已变化")
	}
	document := []byte(gen.PayloadValidationSpec)
	store.validateAsset, err = apivalidate.NewPayloadValidator(document, "Asset")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.validateUpload, err = apivalidate.NewPayloadValidator(document, "UploadSession")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.validatePin, err = apivalidate.NewPayloadValidator(document, "AcquireResourceReferencesResult")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.validateReferenceInput, err = apivalidate.NewPayloadValidator(document, "AcquireResourceReferencesInput")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.validateDecision, err = apivalidate.NewPayloadValidator(document, "ReferenceSourceDecision")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.validateContentPage, err = apivalidate.NewPayloadValidator(document, "ContentReferencePage")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.validateConfigPage, err = apivalidate.NewPayloadValidator(document, "SiteConfigReferencePage")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.validateSnapshot, err = apivalidate.NewPayloadValidator(document, "ReferenceSnapshot")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.validateCleanupPlan, err = apivalidate.NewPayloadValidator(document, "CleanupPlan")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.validateCleanupApproval, err = apivalidate.NewPayloadValidator(document, "CleanupApproval")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.validateCleanupJobReceipt, err = apivalidate.NewPayloadValidator(document, "CleanupJobReceipt")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.runMigrations(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := outbox.EnsureSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := inbox.EnsureSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close 关闭元数据库；重复调用安全。
func (s *localBlobStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "关闭媒体元数据库失败")
	}
	return nil
}

// runMigrations 按编号执行 migrations/ 内嵌的只追加迁移；首个迁移自建
// schema_migrations，已应用版本超出迁移集或名称不符时拒绝启动。
func (s *localBlobStore) runMigrations() error {
	all, err := mediamigrations.All()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "媒体迁移集不合法")
	}
	applied := map[int64]string{}
	rows, err := s.db.Query(`SELECT version, name FROM schema_migrations`)
	if err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			return platform.NewError(platform.CodeIntegrityError, false, "读取媒体迁移记录失败")
		}
	} else {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				_ = rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "媒体迁移记录损坏")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "读取媒体迁移记录失败")
		}
		_ = rows.Close()
	}
	for version, name := range applied {
		if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
			return platform.NewError(platform.CodeIntegrityError, false, "媒体迁移版本未知")
		}
	}
	for _, migration := range all {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		tx, err := s.db.Begin()
		if err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "开始媒体迁移事务失败")
		}
		if _, err := tx.Exec(migration.SQL); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeIntegrityError, false, "执行媒体迁移失败")
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			migration.Version, migration.Name, platform.Now()); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeInternal, false, "登记媒体迁移失败")
		}
		if err := tx.Commit(); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "提交媒体迁移失败")
		}
	}
	return nil
}

// sqlExecer 是事务与数据库共用的写入面。
type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func appendJournal(ctx context.Context, exec sqlExecer, event, siteID, uploadID Identifier, sha256Hash Hash, assetID Identifier) error {
	var asset any
	if assetID != "" {
		asset = string(assetID)
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO blob_journal (event, site_id, upload_id, sha256, asset_id, occurred_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		event, string(siteID), string(uploadID), string(sha256Hash), asset, string(platform.Now())); err != nil {
		return mapMediaDBError(err, "写入媒体journal失败")
	}
	return nil
}

func (s *localBlobStore) uploadPath(uploadID Identifier) string {
	return filepath.Join(s.root, "uploads", uploadID+".part")
}

func (s *localBlobStore) stagingPath(uploadID Identifier, sha256Hash Hash) string {
	return filepath.Join(s.root, "staging", uploadID+"-"+sha256Hash+".part")
}

func (s *localBlobStore) blobPath(siteID Identifier, sha256Hash Hash) string {
	return filepath.Join(s.root, "blobs", siteID, sha256Hash[:2], sha256Hash)
}

func (s *localBlobStore) isolatedPath(prefix, uploadID Identifier, sha256Hash Hash) string {
	return filepath.Join(s.root, "isolated", prefix+"-"+uploadID+"-"+sha256Hash)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func hashFile(path string) (Hash, int64, error) {
	if err := lifecycle.ValidatePrivatePath(path, "regular-file", false); err != nil {
		return "", 0, err
	}
	lstat, err := os.Lstat(path)
	if err != nil || !lstat.Mode().IsRegular() {
		return "", 0, errors.New("非普通文件")
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(lstat, info) {
		return "", 0, errors.New("非普通文件或身份变化")
	}
	hasher := sha256.New()
	buffer := make([]byte, hashBufferSize)
	var size int64
	for {
		readSize, readErr := file.Read(buffer)
		if readSize < 0 || readSize > len(buffer) {
			return "", 0, errors.New("文件读取结果不合法")
		}
		if readSize > 0 {
			if _, err := hasher.Write(buffer[:readSize]); err != nil {
				return "", 0, err
			}
			size += int64(readSize)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return "", 0, readErr
		}
		if readSize == 0 {
			return "", 0, errors.New("文件读取无进展")
		}
	}
	if err := lifecycle.ValidatePrivatePath(path, "regular-file", false); err != nil {
		return "", 0, err
	}
	named, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, named) {
		return "", 0, errors.New("资源路径身份变化")
	}
	return Hash(hex.EncodeToString(hasher.Sum(nil))), size, nil
}

type storedUpload struct {
	session   gen.UploadSession
	createdBy Identifier
	dataEpoch Identifier
}

func readUploadRow(ctx context.Context, exec sqlExecer, uploadID Identifier, validate func(json.RawMessage) error) (storedUpload, bool, error) {
	var raw sql.NullString
	var createdBy, site, state string
	var expected int
	var epoch sql.NullString
	err := exec.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(session_json AS BLOB))<=16777216 THEN session_json ELSE NULL END,created_by,data_epoch,site_id,state,expected_bytes FROM upload_sessions WHERE upload_id = ?`, string(uploadID)).Scan(&raw, &createdBy, &epoch, &site, &state, &expected)
	if errors.Is(err, sql.ErrNoRows) {
		return storedUpload{}, false, nil
	}
	if err != nil {
		return storedUpload{}, false, mapMediaDBError(err, "读取上传会话失败")
	}
	var session gen.UploadSession
	if !raw.Valid || validate == nil || validate(json.RawMessage(raw.String)) != nil || json.Unmarshal([]byte(raw.String), &session) != nil || session.UploadId != uploadID || session.SiteId != site || string(session.State) != state || session.ExpectedBytes != expected || session.ReceivedBytes > session.ExpectedBytes || !platform.ValidIdentifier(createdBy) {
		return storedUpload{}, false, platform.NewError(platform.CodeIntegrityError, false, "上传会话元数据损坏")
	}
	return storedUpload{session: session, createdBy: Identifier(createdBy), dataEpoch: epoch.String}, true, nil
}

func writeUploadRow(ctx context.Context, exec sqlExecer, record storedUpload, insert bool) error {
	raw, err := json.Marshal(record.session)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "序列化上传会话失败")
	}
	now := string(platform.Now())
	if insert {
		_, err = exec.ExecContext(ctx, `
			INSERT INTO upload_sessions (upload_id, site_id, created_by, state, expected_bytes, session_json, updated_at,data_epoch)
			VALUES (?, ?, ?, ?, ?, ?, ?,?)`,
			string(record.session.UploadId), string(record.session.SiteId), string(record.createdBy),
			string(record.session.State), record.session.ExpectedBytes, string(raw), now, record.dataEpoch)
	} else {
		_, err = exec.ExecContext(ctx, `
			UPDATE upload_sessions SET state = ?, session_json = ?, updated_at = ? WHERE upload_id = ?`,
			string(record.session.State), string(raw), now, string(record.session.UploadId))
	}
	if err != nil {
		return mapMediaDBError(err, "保存上传会话失败")
	}
	return nil
}

func (s *localBlobStore) OpenUpload(ctx context.Context, uploadID Identifier) (ioReadCloser, error) {
	root, err := s.openMediaRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return openMediaRegular(root, s.root, "uploads/"+uploadID+".part", os.O_RDONLY)
}

// CreateUpload 为后续上传 handler 提供私有 receiving 会话落点；会话、回执与
// 临时文件必须同时成功，元数据事务失败时删除临时文件，不留下可被恢复误认的半成品。
func (s *localBlobStore) replayBeginUpload(ctx context.Context, op OperationContext, q sqlExecer, requestHash Hash) (gen.UploadSession, bool, error) {
	var storedHash, id string
	var raw, rawHash sql.NullString
	err := q.QueryRowContext(ctx, `SELECT request_hash,upload_id,CASE WHEN result_json IS NULL OR length(CAST(result_json AS BLOB))<=16777216 THEN result_json ELSE NULL END,result_hash FROM upload_receipts WHERE operation_id=?`, op.OperationID).Scan(&storedHash, &id, &raw, &rawHash)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.UploadSession{}, false, nil
	}
	if err != nil {
		return gen.UploadSession{}, false, mapMediaDBError(err, "读取创建上传原回执失败")
	}
	if storedHash != requestHash || !raw.Valid || !rawHash.Valid {
		return gen.UploadSession{}, true, platform.NewError(platform.CodeConflict, false, "创建上传操作已绑定其他材料或旧回执缺原结果")
	}
	record, found, err := readUploadRow(ctx, q, id, s.validateUpload)
	if err != nil {
		return gen.UploadSession{}, true, err
	}
	if !found {
		return gen.UploadSession{}, true, platform.NewError(platform.CodeIntegrityError, false, "创建上传回执缺来源会话")
	}
	if record.dataEpoch == "" || record.dataEpoch != op.DataEpoch {
		return gen.UploadSession{}, true, platform.NewError(platform.CodeVersionMismatch, false, "创建上传原代次未知或已变化")
	}
	if record.createdBy != op.Actor.SubjectID || record.session.SiteId != op.Actor.SiteID {
		return gen.UploadSession{}, true, platform.NewError(platform.CodeConflict, false, "创建上传操作已绑定其他主体或站点")
	}
	if platform.HashBytes([]byte(raw.String)) != rawHash.String || s.validateUpload(json.RawMessage(raw.String)) != nil {
		return gen.UploadSession{}, true, platform.NewError(platform.CodeIntegrityError, false, "创建上传原结果损坏")
	}
	var result gen.UploadSession
	if json.Unmarshal([]byte(raw.String), &result) != nil || result.UploadId != id || result.SiteId != op.Actor.SiteID || result.State != gen.Receiving || result.Revision != 0 || result.ReceivedBytes != 0 {
		return result, true, platform.NewError(platform.CodeIntegrityError, false, "创建上传原结果与来源不符")
	}
	return result, true, nil
}

func (s *localBlobStore) CreateUpload(ctx context.Context, op OperationContext, session gen.UploadSession, quota uploadQuota, requestHash Hash) (gen.UploadSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	original, found, err := s.replayBeginUpload(ctx, op, s.db, requestHash)
	if err != nil || found {
		return original, err
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return gen.UploadSession{}, err
	}
	defer cancel()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return gen.UploadSession{}, mapMediaDBError(err, "开始上传会话事务失败")
	}
	defer tx.Rollback()
	var chunkID string
	if err := tx.QueryRowContext(work, `SELECT operation_id FROM upload_chunks WHERE operation_id=?`, op.OperationID).Scan(&chunkID); err == nil {
		return gen.UploadSession{}, platform.NewError(platform.CodeConflict, false, "operationId已绑定上传分块")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return gen.UploadSession{}, mapMediaDBError(err, "读取分块操作绑定失败")
	}
	var receivingCount int
	var receivingBytes, readyBytes int64
	if err := tx.QueryRowContext(work, `SELECT COUNT(CASE WHEN state='receiving' THEN 1 END),COALESCE(SUM(expected_bytes),0) FROM upload_sessions WHERE site_id=? AND state IN ('receiving','blocked')`, op.Actor.SiteID).Scan(&receivingCount, &receivingBytes); err != nil {
		return gen.UploadSession{}, mapMediaDBError(err, "读取上传预留失败")
	}
	if err := tx.QueryRowContext(work, `SELECT COALESCE(SUM(byte_size),0) FROM assets WHERE site_id=? AND state='ready'`, op.Actor.SiteID).Scan(&readyBytes); err != nil {
		return gen.UploadSession{}, mapMediaDBError(err, "读取站点存储用量失败")
	}
	if session.ExpectedBytes > quota.MaxUploadBytes || receivingCount >= quota.MaxUploadConcurrency || receivingBytes+readyBytes > quota.SiteStorageBytes-int64(session.ExpectedBytes) {
		return gen.UploadSession{}, platform.NewError(platform.CodeConflict, false, "站点上传资源配额已满")
	}
	root, err := s.openMediaRoot()
	if err != nil {
		return gen.UploadSession{}, err
	}
	defer root.Close()
	name := "uploads/" + session.UploadId + ".part"
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return gen.UploadSession{}, platform.NewError(platform.CodeDependencyUnavailable, true, "创建上传临时对象失败")
	}
	preserveTemporary := false
	defer func() {
		file.Close()
		if !preserveTemporary {
			_ = root.Remove(name)
		}
	}()
	if err := file.Sync(); err != nil {
		return gen.UploadSession{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化上传临时对象失败")
	}
	if err := file.Close(); err != nil {
		return gen.UploadSession{}, platform.NewError(platform.CodeDependencyUnavailable, true, "关闭上传临时对象失败")
	}
	directory, err := root.Open("uploads")
	if err != nil {
		return gen.UploadSession{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开上传目录失败")
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return gen.UploadSession{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化上传目录失败")
	}
	if err := writeUploadRow(work, tx, storedUpload{session: session, createdBy: op.Actor.SubjectID, dataEpoch: op.DataEpoch}, true); err != nil {
		return gen.UploadSession{}, err
	}
	raw, err := platform.CanonicalJSON(session)
	if err != nil {
		return gen.UploadSession{}, platform.NewError(platform.CodeIntegrityError, false, "创建上传原结果无法编码")
	}
	if _, err := tx.ExecContext(work, `INSERT INTO upload_receipts(operation_id,request_hash,upload_id,created_at,result_json,result_hash) VALUES(?,?,?,?,?,?)`, op.OperationID, requestHash, session.UploadId, platform.Now(), string(raw), platform.HashBytes(raw)); err != nil {
		return gen.UploadSession{}, mapMediaDBError(err, "持久化创建上传原回执失败")
	}
	// Commit返回错误可能无法确定是否已落库，保留文件但仍返回错误；同op查原回执确认。
	preserveTemporary = true
	if err := tx.Commit(); err != nil {
		return gen.UploadSession{}, mapMediaDBError(err, "提交上传会话失败")
	}
	return session, nil
}

func (s *localBlobStore) ReadUpload(ctx context.Context, uploadID Identifier) (gen.UploadSession, Identifier, error) {
	if err := ctx.Err(); err != nil {
		return gen.UploadSession{}, "", contextError(err)
	}
	if !validIdentifier(uploadID) {
		return gen.UploadSession{}, "", platform.NewError(platform.CodeValidation, false, "上传标识不合法")
	}
	record, exists, err := readUploadRow(ctx, s.db, uploadID, s.validateUpload)
	if err != nil {
		return gen.UploadSession{}, "", err
	}
	if !exists {
		return gen.UploadSession{}, "", platform.NewError(platform.CodeNotFound, false, "上传不存在")
	}
	return record.session, record.createdBy, nil
}

func (s *localBlobStore) AbortUpload(ctx context.Context, siteID, uploadID Identifier) error {
	if err := ctx.Err(); err != nil {
		return contextError(err)
	}
	if !validIdentifier(siteID) || !validIdentifier(uploadID) {
		return platform.NewError(platform.CodeValidation, false, "上传标识不合法")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapMediaDBError(err, "开始取消上传事务失败")
	}
	defer func() { _ = tx.Rollback() }()
	record, exists, err := readUploadRow(ctx, tx, uploadID, s.validateUpload)
	if err != nil {
		return err
	}
	if !exists {
		return platform.NewError(platform.CodeNotFound, false, "上传不存在")
	}
	if record.session.SiteId != siteID {
		return platform.NewError(platform.CodeForbidden, false, "上传不属于当前站点")
	}
	if record.session.State == gen.Aborted || record.session.State == gen.Expired {
		return nil
	}
	if record.session.State != gen.Receiving {
		return platform.NewError(platform.CodeConflict, false, "上传状态不允许取消")
	}
	record.session.State = gen.Aborted
	record.session.Revision++
	if err := writeUploadRow(ctx, tx, record, false); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return mapMediaDBError(err, "提交取消上传失败")
	}
	_ = os.Remove(s.uploadPath(uploadID))
	return nil
}

// CommitVerifiedBlob 在调用方持有上传互斥锁时提交文件与完成事务。
func (s *localBlobStore) CommitVerifiedBlob(ctx context.Context, op OperationContext, input CommitAssetBlobInput) (gen.Asset, error) {
	// Complete持store互斥；未完成DELETING决定存在时不得复用或重建相同bytes。
	var deleting int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM manual_cleanup_deletions d JOIN manual_cleanup_executions e ON e.plan_id=d.plan_id JOIN assets a ON a.asset_id=d.asset_id WHERE a.site_id=? AND a.sha256=? AND e.result_json IS NULL`, op.Actor.SiteID, input.SHA256).Scan(&deleting); err != nil {
		return gen.Asset{}, mapMediaDBError(err, "核对共享blob删除决定失败")
	}
	if deleting != 0 {
		return gen.Asset{}, platform.NewError(platform.CodeLocked, false, "共享blob的人工删除决定尚未完成")
	}
	record, found, err := readUploadRow(ctx, s.db, input.UploadID, s.validateUpload)
	if err != nil {
		return gen.Asset{}, err
	}
	if !found {
		return gen.Asset{}, platform.NewError(platform.CodeNotFound, false, "上传不存在")
	}
	root, err := s.openMediaRoot()
	if err != nil {
		return gen.Asset{}, err
	}
	defer root.Close()
	prepare, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return gen.Asset{}, err
	}
	defer cancel()
	if err := appendJournal(prepare, s.db, blobJournalPrepared, op.Actor.SiteID, input.UploadID, input.SHA256, ""); err != nil {
		return gen.Asset{}, err
	}
	finalPath := s.blobPath(op.Actor.SiteID, input.SHA256)
	if err := ensureMediaDirectory(filepath.Dir(finalPath)); err != nil {
		return gen.Asset{}, err
	}
	finalName := "blobs/" + op.Actor.SiteID + "/" + input.SHA256[:2] + "/" + input.SHA256
	finalInfo, err := root.Lstat(finalName)
	if err == nil {
		if !finalInfo.Mode().IsRegular() {
			return gen.Asset{}, platform.NewError(platform.CodeIntegrityError, false, "不可变资源不是普通文件")
		}
		existing, err := openMediaRegular(root, s.root, finalName, os.O_RDONLY)
		if err != nil {
			return gen.Asset{}, err
		}
		digest := sha256.New()
		size, err := io.CopyBuffer(digest, chunkContextReader{prepare, existing}, make([]byte, hashBufferSize))
		existing.Close()
		if err != nil {
			return gen.Asset{}, mapMediaDBError(err, "核对同站点资源失败")
		}
		if size != int64(record.session.ExpectedBytes) || hex.EncodeToString(digest.Sum(nil)) != input.SHA256 {
			return gen.Asset{}, platform.NewError(platform.CodeIntegrityError, false, "同站点不可变资源SHA或长度不一致")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		source, err := openMediaRegular(root, s.root, "uploads/"+input.UploadID+".part", os.O_RDONLY)
		if err != nil {
			return gen.Asset{}, err
		}
		defer source.Close()
		stageName := "staging/" + input.UploadID + "-" + input.SHA256 + ".part"
		stage, err := root.OpenFile(stageName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "创建资源暂存失败")
		}
		preserve := false
		defer func() {
			stage.Close()
			if !preserve {
				_ = root.Remove(stageName)
			}
		}()
		digest := sha256.New()
		size, err := io.CopyBuffer(io.MultiWriter(stage, digest), io.LimitReader(chunkContextReader{prepare, source}, int64(record.session.ExpectedBytes)+1), make([]byte, hashBufferSize))
		if err != nil {
			return gen.Asset{}, mapMediaDBError(err, "写入不可变资源失败")
		}
		if size != int64(record.session.ExpectedBytes) || hex.EncodeToString(digest.Sum(nil)) != input.SHA256 {
			return gen.Asset{}, platform.NewError(platform.CodeIntegrityError, false, "提交时上传SHA或长度已变化")
		}
		if err := stage.Sync(); err != nil {
			return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化资源暂存失败")
		}
		if err := stage.Close(); err != nil {
			return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "关闭资源暂存失败")
		}
		if err := root.Rename(stageName, finalName); err != nil {
			return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "提交不可变资源失败")
		}
		preserve = true
	} else {
		return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取不可变资源路径失败")
	}
	// 目录确认失败时不能提交ready；已有文件也重新确认持久目录边界。
	if err := syncDirectory(filepath.Dir(finalPath)); err != nil {
		return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化资源目录失败")
	}
	if err := syncDirectory(filepath.Join(s.root, "staging")); err != nil {
		return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化资源暂存目录失败")
	}
	work, commitCancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return gen.Asset{}, err
	}
	defer commitCancel()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return gen.Asset{}, mapMediaDBError(err, "开始完成上传事务失败")
	}
	defer tx.Rollback()
	asset, found, err := s.findReadyAsset(work, tx, op.Actor.SiteID, input.SHA256, record.session.ExpectedBytes, input.MediaType, gen.AssetKind(record.session.Kind))
	if err != nil {
		return gen.Asset{}, err
	}
	if !found {
		asset = gen.Asset{Animated: input.Animated, CreatedAt: time.Now().UTC(), CreatedBy: record.createdBy, Height: input.Height, Width: input.Width, ImageVariants: []gen.ImageVariant{}, Kind: gen.AssetKind(record.session.Kind), OriginalName: record.session.OriginalName, Ref: gen.AssetVersionRef{AssetId: platform.NewIdentifier(), Version: 0, Sha256: input.SHA256, MediaType: input.MediaType, ByteSize: record.session.ExpectedBytes}, Revision: 0, SiteId: op.Actor.SiteID, State: gen.AssetStateReady}
		raw, err := platform.CanonicalJSON(asset)
		if err != nil {
			return gen.Asset{}, platform.NewError(platform.CodeIntegrityError, false, "资源元数据无法编码")
		}
		if _, err := tx.ExecContext(work, `INSERT INTO assets(asset_id,site_id,sha256,byte_size,media_type,state,asset_json,updated_at) VALUES(?,?,?,?,?,'ready',?,?)`, asset.Ref.AssetId, asset.SiteId, asset.Ref.Sha256, asset.Ref.ByteSize, asset.Ref.MediaType, string(raw), platform.Now()); err != nil {
			return gen.Asset{}, mapMediaDBError(err, "写入资源元数据失败")
		}
		if _, err := tx.ExecContext(work, `INSERT INTO reference_objects(asset_id,deletion_fence,reference_generation) VALUES(?,0,0)`, asset.Ref.AssetId); err != nil {
			return gen.Asset{}, mapMediaDBError(err, "建立新资源引用fence失败")
		}
	}
	record.session.State = gen.Complete
	record.session.Revision++
	completeInput := CompleteUploadInput{UploadId: input.UploadID, ExpectedRevision: input.ExpectedRevision}
	if err := s.casCompleteSession(work, tx, op, completeInput, record); err != nil {
		return gen.Asset{}, err
	}
	if err := appendJournal(work, tx, blobJournalCommitted, op.Actor.SiteID, input.UploadID, input.SHA256, asset.Ref.AssetId); err != nil {
		return gen.Asset{}, err
	}
	if err := writeCompletionReceipt(work, tx, op, input.UploadID, input.RequestHash, completionReceipt{Version: 1, Kind: "complete_upload", Asset: &asset}); err != nil {
		return gen.Asset{}, err
	}
	if !found {
		if err := PublishAssetEvent(work, op, PublishAssetEventInput{Tx: tx, Asset: asset, EventID: platform.NewIdentifier(), OccurredAt: platform.Now()}); err != nil {
			return gen.Asset{}, err
		}
	}
	// 未知Commit不能删除临时或不可变文件；同operationId读取SQL原回执确认。
	if err := tx.Commit(); err != nil {
		return gen.Asset{}, mapMediaDBError(err, "提交完成上传失败")
	}
	_ = root.Remove("uploads/" + input.UploadID + ".part")
	return asset, nil
}

func (s *localBlobStore) findReadyAsset(ctx context.Context, exec sqlExecer, siteID Identifier, sha256Hash Hash, byteSize int, mediaType string, kind gen.AssetKind) (gen.Asset, bool, error) {
	var raw sql.NullString
	var assetID string
	err := exec.QueryRowContext(ctx, `SELECT asset_id,CASE WHEN length(CAST(asset_json AS BLOB))<=16777216 THEN asset_json ELSE NULL END FROM assets WHERE site_id=? AND sha256=? AND byte_size=? AND media_type=? AND state='ready' AND json_extract(asset_json,'$.kind')=? ORDER BY asset_id LIMIT 1`, siteID, sha256Hash, byteSize, mediaType, kind).Scan(&assetID, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.Asset{}, false, nil
	}
	if err != nil {
		return gen.Asset{}, false, mapMediaDBError(err, "读取已有资源失败")
	}
	var asset gen.Asset
	if !raw.Valid || s.validateAsset(json.RawMessage(raw.String)) != nil || json.Unmarshal([]byte(raw.String), &asset) != nil || asset.Ref.AssetId != assetID || asset.Kind != kind || asset.SiteId != siteID || asset.Ref.Sha256 != sha256Hash || asset.Ref.ByteSize != byteSize || asset.Ref.MediaType != mediaType || asset.State != gen.AssetStateReady {
		return gen.Asset{}, false, platform.NewError(platform.CodeIntegrityError, false, "已有ready资源元数据与索引不符")
	}
	return asset, true, nil
}

func mapMediaDBError(err error, message string) error {
	switch {
	case errors.Is(err, context.Canceled):
		return platform.NewError(platform.CodeCancelled, true, "%s", message)
	case errors.Is(err, context.DeadlineExceeded):
		return platform.NewError(platform.CodeDeadlineExceeded, true, "%s", message)
	default:
		return platform.NewError(platform.CodeDependencyUnavailable, true, "%s", message)
	}
}

// CommitAssetBlob 仅供CompleteUpload持锁、校验完毕的链调用。
func CommitAssetBlob(ctx context.Context, op OperationContext, input CommitAssetBlobInput) (CommitAssetBlobResult, error) {
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储未装配")
	}
	asset, err := s.CommitVerifiedBlob(ctx, op, input)
	return CommitAssetBlobResult{Asset: asset}, err
}

func mapBlobStoreError(err error, fallback string) error {
	if err == nil {
		return nil
	}
	var platformErr *platform.Error
	if errors.As(err, &platformErr) {
		return platformErr
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "%s", fallback)
}

// ensureMediaDirectory 逐段拒绝符号链接/特殊文件，目录创建后同步父目录。
func ensureMediaDirectory(path string) error {
	current := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Clean(path), current), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err = os.Mkdir(current, 0o700); err != nil {
				return platform.NewError(platform.CodeDependencyUnavailable, true, "创建媒体私有目录失败")
			}
			if err = syncDirectory(filepath.Dir(current)); err != nil {
				return platform.NewError(platform.CodeDependencyUnavailable, true, "持久化媒体私有目录失败")
			}
			continue
		}
		if err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有目录不可读")
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return platform.NewError(platform.CodeIntegrityError, false, "媒体私有目录包含链接或非目录")
		}
	}
	return nil
}

func (s *localBlobStore) openMediaRoot() (*os.Root, error) {
	if err := ensureMediaDirectory(s.root); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "打开媒体私有目录失败")
	}
	for _, dir := range []string{"uploads", "staging"} {
		info, err := root.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			root.Close()
			return nil, platform.NewError(platform.CodeIntegrityError, false, "媒体上传目录不安全")
		}
	}
	return root, nil
}

// openMediaRegular 使用根目录能力和NOFOLLOW打开已存在的普通文件。
func openMediaRegular(root *os.Root, absoluteRoot, name string, flags int) (*os.File, error) {
	absolute := filepath.Join(absoluteRoot, name)
	if err := lifecycle.ValidatePrivatePath(absolute, "regular-file", false); err != nil {
		return nil, err
	}
	before, err := root.Lstat(name)
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "媒体上传文件缺失")
	}
	namedBefore, err := os.Lstat(absolute)
	if err != nil || !os.SameFile(before, namedBefore) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "媒体目录能力与路径身份不一致")
	}
	file, err := root.OpenFile(name, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "打开媒体上传文件失败")
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		file.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "媒体上传文件身份已变化")
	}
	if err := lifecycle.ValidatePrivatePath(absolute, "regular-file", false); err != nil {
		file.Close()
		return nil, err
	}
	namedAfter, err := os.Lstat(absolute)
	if err != nil || !os.SameFile(after, namedAfter) {
		file.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "媒体文件打开后路径身份已变化")
	}
	return file, nil
}

// ConfigureWriteWindow 仅由受信装配注入运行权威；本轮维护直接关闭，不伪造屏障窗口。
func ConfigureWriteWindow(config platform.WriteWindowConfig) error {
	store, ok := currentBlobStore().(*localBlobStore)
	if !ok || config.Runtime == nil || config.WriteWindow == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "媒体写窗口未完整装配")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.window = config
	return nil
}

func CloseLocalBlobStore() error {
	activeBlobStoreMu.Lock()
	store := activeBlobStore
	activeBlobStore = nil
	activeBlobStoreMu.Unlock()
	if local, ok := store.(*localBlobStore); ok {
		return local.Close()
	}
	return nil
}

func LocalStorageRecovered(epoch Identifier) bool {
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.openedEpoch == epoch && s.chunkRecovered && s.blobRecovered
}
