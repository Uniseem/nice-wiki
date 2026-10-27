package content

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nice-wiki/libs/platform/pgdb"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/content/migrations"
)

// Store只拥有本域active库；正文准备与业务提交是不同持久边界。
// 草稿提交另由受信创建/保存意图装配，不能从文件准备隐式创建文档。
type Store struct {
	db              *sql.DB
	root            string
	epoch           Identifier
	initializeEpoch bool
	mu              sync.Mutex
}

// RevisionStorageConfig由已授权上游提供本次真实站点限额和公共安全窗口。
// 空配置可用于已提交读取；文件写入会在既有入口拒绝缺少配置。
type RevisionStorageConfig struct {
	MaxDocumentBytes int64
	WriteWindow      platform.WriteWindowConfig
}

type revisionStorage struct {
	store  *Store
	config RevisionStorageConfig
}

// OpenDatabase安全打开active/content.db；已存在的未知库不视为空库。
func OpenDatabase(ctx context.Context, dataRoot string, epoch Identifier) (*Store, error) {
	if !platform.ValidIdentifier(epoch) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "正文打开代次未知")
	}
	root := filepath.Join(dataRoot, "active")
	if !filepath.IsAbs(dataRoot) {
		return nil, platform.NewError(platform.CodeValidation, false, "正文私有根必须为绝对路径")
	}
	if err := lifecycle.ValidatePrivatePath(root, "directory", false); err != nil {
		return nil, err
	}
	path := filepath.Join(root, "content.db")
	_, statErr := os.Lstat(path)
	newDatabase := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !newDatabase {
		return nil, contentStorageError()
	}
	for _, name := range []string{"content.db", "content.db-wal", "content.db-shm"} {
		if err := lifecycle.ValidatePrivatePath(filepath.Join(root, name), "regular-file", true); err != nil {
			return nil, err
		}
	}
	if newDatabase {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return nil, contentStorageError()
		}
		if err := file.Close(); err != nil {
			return nil, contentStorageError()
		}
	}
	dsn := (&url.URL{Scheme: "file", Path: path}).String() + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)&_pragma=synchronous(FULL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, contentStorageError()
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, root: root, epoch: epoch, initializeEpoch: newDatabase}
	success := false
	defer func() {
		if !success {
			_ = db.Close()
		}
	}()
	if err := db.PingContext(ctx); err != nil {
		return nil, contentStorageError()
	}
	if !newDatabase {
		var exists int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&exists); err != nil || exists != 1 {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "既有正文库缺少受支持迁移记录")
		}
	}
	if err := s.RunContentMigrations(ctx); err != nil {
		return nil, err
	}
	var storedEpoch string
	if err := db.QueryRowContext(ctx, `SELECT data_epoch FROM store_identity WHERE singleton=1`).Scan(&storedEpoch); err != nil || storedEpoch != epoch {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "正文库活动代次缺失或不一致")
	}
	for _, name := range []string{"content.db", "content.db-wal", "content.db-shm"} {
		if err := lifecycle.ValidatePrivatePath(filepath.Join(root, name), "regular-file", true); err != nil {
			return nil, err
		}
	}
	if err := syncDirectory(root); err != nil {
		return nil, contentStorageError()
	}
	success = true
	return s, nil
}

// RunContentMigrations拒绝集外版本、缺号和修改已应用SQL；每个新增版本独立事务。
func (s *Store) RunContentMigrations(ctx context.Context) error {
	all, err := migrations.All()
	if err != nil || len(all) == 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "正文迁移集不合法")
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&exists); err != nil {
		return contentStorageError()
	}
	applied := 0
	if exists != 0 {
		rows, err := s.db.QueryContext(ctx, `SELECT version,name,sha256 FROM schema_migrations ORDER BY version`)
		if err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "正文迁移记录不可读")
		}
		for rows.Next() {
			var version int64
			var name, hash string
			if err := rows.Scan(&version, &name, &hash); err != nil {
				_ = rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "正文迁移记录损坏")
			}
			if applied >= len(all) || version != int64(applied+1) || name != all[applied].Name || hash != all[applied].SHA256 {
				_ = rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "正文迁移版本或内容不匹配")
			}
			applied++
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return contentStorageError()
		}
		if applied == 0 {
			return platform.NewError(platform.CodeIntegrityError, false, "正文迁移记录为空")
		}
	}
	for _, item := range all[applied:] {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return contentStorageError()
		}
		if _, err := tx.ExecContext(ctx, item.SQL); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeIntegrityError, false, "正文迁移执行失败")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,name,sha256,applied_at) VALUES(?,?,?,?)`, item.Version, item.Name, item.SHA256, platform.Now()); err != nil {
			_ = tx.Rollback()
			return contentStorageError()
		}
		if s.initializeEpoch && item.Version == 1 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO store_identity(singleton,data_epoch,site_id) VALUES(1,?,NULL)`, s.epoch); err != nil {
				_ = tx.Rollback()
				return contentStorageError()
			}
		}
		if err := tx.Commit(); err != nil {
			return contentStorageError()
		}
	}
	return nil
}

func (s *Store) ConfigureRevisionStorage(config RevisionStorageConfig) RevisionFileStore {
	return &revisionStorage{store: s, config: config}
}
func (s *revisionStorage) RevisionRoot() string            { return s.store.root }
func (s *revisionStorage) RevisionDataEpoch() Identifier   { return s.store.epoch }
func (s *revisionStorage) RevisionMaxDocumentBytes() int64 { return s.config.MaxDocumentBytes }
func (s *revisionStorage) RevisionWriteWindow() platform.WriteWindowConfig {
	return s.config.WriteWindow
}

const preparationScopeSQL = `caller_service=? AND actor_id=? AND site_id=? AND data_epoch=? AND operation_id=?`

type preparationRow struct {
	commitID, documentID, revisionID Identifier
	state                            string
	digest                           Hash
	result                           WriteRevisionFileResult
}

// FindRevisionPreparation只回放首次材料；JSONL隔离/终止不能被旧SQLite preparing复活。
func (s *revisionStorage) FindRevisionPreparation(ctx context.Context, op OperationContext, digest Hash) (PreparedRevisionFile, bool, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	return s.store.findRevisionPreparation(ctx, op, digest)
}

func (s *Store) findRevisionPreparation(ctx context.Context, op OperationContext, digest Hash) (PreparedRevisionFile, bool, error) {
	if err := s.checkRevisionScope(ctx, op); err != nil {
		return PreparedRevisionFile{}, false, err
	}
	var row preparationRow
	var raw []byte
	err := s.db.QueryRowContext(ctx, `SELECT commit_id,document_id,revision_id,state,request_hash,result_json FROM revision_preparations WHERE `+preparationScopeSQL,
		op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.OperationID).Scan(&row.commitID, &row.documentID, &row.revisionID, &row.state, &row.digest, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return PreparedRevisionFile{}, false, nil
	}
	if err != nil {
		return PreparedRevisionFile{}, false, contentStorageError()
	}
	if row.digest != digest {
		return PreparedRevisionFile{}, false, platform.NewError(platform.CodeConflict, false, "正文准备操作摘要不一致")
	}
	if err := decodePreparation(raw, &row, op); err != nil {
		return PreparedRevisionFile{}, false, err
	}
	state, found, err := s.preparationJournalState(ctx, row.result.Journal)
	if err != nil {
		return PreparedRevisionFile{}, false, err
	}
	committed, err := s.hasCommittedReceipt(ctx, row.commitID)
	if err != nil {
		return PreparedRevisionFile{}, false, err
	}
	if !found && (row.state == "prepared" || row.state == "committed" || committed) {
		return PreparedRevisionFile{}, false, platform.NewError(platform.CodeIntegrityError, false, "正文持久准备或提交决定缺少匹配日志")
	}
	if row.state == "quarantined" || (found && state == "quarantined") {
		row.state = "quarantined"
	} else if committed {
		row.state = "committed"
	} else if row.state == "committed" || (found && state == "committed") {
		return PreparedRevisionFile{}, false, platform.NewError(platform.CodeIntegrityError, false, "正文COMMITTED缺少私有提交资格")
	} else if found && state == "aborted" {
		row.state = "aborted"
	}
	return PreparedRevisionFile{Result: row.result, State: row.state}, true, nil
}

// PrepareRevision在当前公共写窗口内原子绑定首次站点与不可变原材料。
func (s *revisionStorage) PrepareRevision(ctx context.Context, op OperationContext, digest Hash, result WriteRevisionFileResult) (PreparedRevisionFile, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if existing, found, err := s.store.findRevisionPreparation(ctx, op, digest); err != nil {
		return PreparedRevisionFile{}, err
	} else if found {
		return existing, nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return PreparedRevisionFile{}, contentStorageError()
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return PreparedRevisionFile{}, contentStorageError()
	}
	defer tx.Rollback()
	binding, err := tx.ExecContext(ctx, `UPDATE store_identity SET site_id=? WHERE singleton=1 AND data_epoch=? AND (site_id IS NULL OR site_id=?)`, op.Actor.SiteID, op.DataEpoch, op.Actor.SiteID)
	if err != nil {
		return PreparedRevisionFile{}, contentStorageError()
	}
	count, err := binding.RowsAffected()
	if err != nil {
		return PreparedRevisionFile{}, contentStorageError()
	}
	if count != 1 {
		return PreparedRevisionFile{}, platform.NewError(platform.CodeForbidden, false, "正文库站点或代次不匹配")
	}
	var owned int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM revision_preparations WHERE site_id=? AND revision_id=?`, op.Actor.SiteID, result.Draft.RevisionId).Scan(&owned); err != nil {
		return PreparedRevisionFile{}, contentStorageError()
	}
	if owned != 0 {
		return PreparedRevisionFile{}, platform.NewError(platform.CodeConflict, false, "不可变正文修订已归属其他操作")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO revision_preparations(commit_id,caller_service,actor_id,site_id,data_epoch,operation_id,request_hash,document_id,revision_id,state,result_json) VALUES(?,?,?,?,?,?,?,?,?,'preparing',?)`,
		result.Journal.CommitID, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.OperationID, digest, result.Draft.DocumentId, result.Draft.RevisionId, string(raw)); err != nil {
		return PreparedRevisionFile{}, contentStorageError()
	}
	if err := tx.Commit(); err != nil {
		return PreparedRevisionFile{}, contentStorageError()
	}
	return PreparedRevisionFile{Result: result, State: "preparing"}, nil
}

func (s *revisionStorage) MarkRevisionPrepared(ctx context.Context, op OperationContext, commitID Identifier) error {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if err := s.store.checkRevisionScope(ctx, op); err != nil {
		return err
	}
	var digest Hash
	if err := s.store.db.QueryRowContext(ctx, `SELECT request_hash FROM revision_preparations WHERE commit_id=? AND `+preparationScopeSQL, commitID, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.OperationID).Scan(&digest); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "正文准备材料不存在或scope不符")
	}
	record, found, err := s.store.findRevisionPreparation(ctx, op, digest)
	if err != nil {
		return err
	}
	if !found || record.Result.Journal.CommitID != commitID {
		return platform.NewError(platform.CodeIntegrityError, false, "正文准备身份不一致")
	}
	switch record.State {
	case "prepared", "committed":
		return nil
	case "aborted", "quarantined":
		return platform.NewError(platform.CodeConflict, false, "正文准备已终止或隔离")
	}
	state, found, err := s.store.preparationJournalState(ctx, record.Result.Journal)
	if err != nil {
		return err
	}
	if !found || state != "prepared" {
		return platform.NewError(platform.CodeIntegrityError, false, "正文准备日志未持久")
	}
	if _, err := s.store.db.ExecContext(ctx, `UPDATE revision_preparations SET state='prepared' WHERE commit_id=? AND state='preparing'`, commitID); err != nil {
		return contentStorageError()
	}
	return nil
}

// ReadCommittedRevision不把PREPARED文件、JSONL或当前head猜成提交资格。
func (s *revisionStorage) ReadCommittedRevision(ctx context.Context, op OperationContext, documentID, revisionID Identifier) (CommittedRevisionFile, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if err := s.store.checkRevisionScope(ctx, op); err != nil {
		return CommittedRevisionFile{}, err
	}
	var resultRaw, receiptRaw []byte
	var state, prepState string
	var commitID Identifier
	err := s.store.db.QueryRowContext(ctx, `SELECT p.commit_id,p.result_json,p.state,c.state,r.result_json FROM revision_preparations p JOIN committed_revisions c ON c.commit_id=p.commit_id AND c.site_id=p.site_id AND c.document_id=p.document_id AND c.revision_id=p.revision_id AND c.data_epoch=p.data_epoch JOIN draft_commit_receipts r ON r.commit_id=c.commit_id AND r.caller_service=p.caller_service AND r.actor_id=p.actor_id AND r.site_id=p.site_id AND r.data_epoch=p.data_epoch AND r.operation_id=p.operation_id WHERE p.site_id=? AND p.data_epoch=? AND p.document_id=? AND p.revision_id=? AND NOT EXISTS(SELECT 1 FROM document_quarantines q WHERE q.site_id=p.site_id AND q.document_id=p.document_id)`, op.Actor.SiteID, op.DataEpoch, documentID, revisionID).Scan(&commitID, &resultRaw, &prepState, &state, &receiptRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return CommittedRevisionFile{}, platform.NewError(platform.CodeNotFound, false, "正文修订没有可读的已提交资格")
	}
	if err != nil {
		return CommittedRevisionFile{}, contentStorageError()
	}
	if state != "committed" || prepState != "committed" {
		return CommittedRevisionFile{}, platform.NewError(platform.CodeIntegrityError, false, "正文修订已隔离或提交记录矛盾")
	}
	var result WriteRevisionFileResult
	var receipt CommitDraftJournalResult
	if decodeStorageJSON(resultRaw, &result) != nil || decodeStorageJSON(receiptRaw, &receipt) != nil || result.Journal.CommitID != commitID || !sameStoredDraft(result.Draft, receipt.Draft) {
		return CommittedRevisionFile{}, platform.NewError(platform.CodeIntegrityError, false, "正文提交材料或回执不一致")
	}
	journalState, found, err := s.store.preparationJournalState(ctx, result.Journal)
	if err != nil {
		return CommittedRevisionFile{}, err
	}
	if !found || (journalState == "quarantined" || journalState == "aborted") {
		return CommittedRevisionFile{}, platform.NewError(platform.CodeIntegrityError, false, "正文日志已终止或隔离")
	}
	result.Journal.State = "committed"
	return CommittedRevisionFile{Draft: result.Draft, Journal: result.Journal}, nil
}

func (s *revisionStorage) QuarantineRevision(ctx context.Context, op OperationContext, documentID, revisionID Identifier, reason string) error {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if err := s.store.checkRevisionScope(ctx, op); err != nil {
		return err
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return contentStorageError()
	}
	defer tx.Rollback()
	var committed int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM committed_revisions WHERE site_id=? AND data_epoch=? AND document_id=? AND revision_id=?`, op.Actor.SiteID, op.DataEpoch, documentID, revisionID).Scan(&committed); err != nil {
		return contentStorageError()
	}
	updated, err := tx.ExecContext(ctx, `UPDATE revision_preparations SET state='quarantined' WHERE site_id=? AND data_epoch=? AND document_id=? AND revision_id=?`, op.Actor.SiteID, op.DataEpoch, documentID, revisionID)
	if err != nil {
		return contentStorageError()
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return contentStorageError()
	}
	if affected != 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "正文隔离目标缺失或归属不一致")
	}
	if committed != 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE committed_revisions SET state='quarantined' WHERE site_id=? AND data_epoch=? AND document_id=? AND revision_id=?`, op.Actor.SiteID, op.DataEpoch, documentID, revisionID); err != nil {
			return contentStorageError()
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO document_quarantines(site_id,document_id,reason,quarantined_at) VALUES(?,?,'body_integrity',?) ON CONFLICT(site_id,document_id) DO NOTHING`, op.Actor.SiteID, documentID, platform.Now()); err != nil {
			return contentStorageError()
		}
	}
	if err := tx.Commit(); err != nil {
		return contentStorageError()
	}
	return nil
}

func (s *Store) checkRevisionScope(ctx context.Context, op OperationContext) error {
	if op.DataEpoch != s.epoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "正文操作与打开代次不一致")
	}
	var epoch string
	var site sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT data_epoch,site_id FROM store_identity WHERE singleton=1`).Scan(&epoch, &site); err != nil {
		return contentStorageError()
	}
	if epoch != s.epoch {
		return platform.NewError(platform.CodeIntegrityError, false, "正文持久代次与打开代次不一致")
	}
	if site.Valid && site.String != op.Actor.SiteID {
		return platform.NewError(platform.CodeForbidden, false, "正文操作站点与私有库不一致")
	}
	return nil
}

func (s *Store) CurrentHead(ctx context.Context, documentID Identifier) (Identifier, error) {
	var head Identifier
	err := s.db.QueryRowContext(ctx, `SELECT revision_id FROM draft_heads WHERE site_id=(SELECT site_id FROM store_identity WHERE singleton=1) AND document_id=?`, documentID).Scan(&head)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", contentStorageError()
	}
	return head, nil
}

func (s *Store) HasCommittedReceipt(ctx context.Context, commitID Identifier) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasCommittedReceipt(ctx, commitID)
}

func (s *Store) hasCommittedReceipt(ctx context.Context, commitID Identifier) (bool, error) {
	var prepared, receipt []byte
	err := s.db.QueryRowContext(ctx, `SELECT p.result_json,r.result_json FROM revision_preparations p JOIN committed_revisions c ON c.commit_id=p.commit_id AND c.site_id=p.site_id AND c.document_id=p.document_id AND c.revision_id=p.revision_id AND c.data_epoch=p.data_epoch JOIN draft_commit_receipts r ON r.commit_id=c.commit_id AND r.caller_service=p.caller_service AND r.actor_id=p.actor_id AND r.site_id=p.site_id AND r.data_epoch=p.data_epoch AND r.operation_id=p.operation_id WHERE p.commit_id=? AND p.data_epoch=? AND p.site_id=(SELECT site_id FROM store_identity WHERE singleton=1)`, commitID, s.epoch).Scan(&prepared, &receipt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, contentStorageError()
	}
	var material WriteRevisionFileResult
	var result CommitDraftJournalResult
	if decodeStorageJSON(prepared, &material) != nil || decodeStorageJSON(receipt, &result) != nil || material.Journal.CommitID != commitID || !sameStoredDraft(material.Draft, result.Draft) {
		return false, platform.NewError(platform.CodeIntegrityError, false, "正文持久回执与首次材料不一致")
	}
	return true, nil
}

func (s *Store) QuarantineDocument(ctx context.Context, documentID Identifier, reason string) error {
	var site sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT site_id FROM store_identity WHERE singleton=1 AND data_epoch=?`, s.epoch).Scan(&site); err != nil {
		return contentStorageError()
	}
	if !site.Valid {
		return platform.NewError(platform.CodeIntegrityError, false, "正文隔离缺少持久站点绑定")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO document_quarantines(site_id,document_id,reason,quarantined_at) VALUES(?,?,'recovery_integrity',?) ON CONFLICT(site_id,document_id) DO NOTHING`, site.String, documentID, platform.Now()); err != nil {
		return contentStorageError()
	}
	return nil
}

// RecoverDatabaseJournal只能在入口尚未开放的独占启动段调用。库里有多个站点时逐站点恢复（日志文件各站点共用，按提交归属过滤）。
func (s *Store) RecoverDatabaseJournal(ctx context.Context) error {
	sites, err := pgdb.Sites(ctx, s.db)
	if err != nil {
		return contentStorageError()
	}
	if len(sites) == 0 {
		return s.recoverSiteJournal(pgdb.WithTemplateView(ctx), false)
	}
	for _, site := range sites {
		if err := s.recoverSiteJournal(pgdb.WithSite(ctx, site), true); err != nil {
			return err
		}
	}
	return nil
}

// recoverSiteJournal 恢复当前上下文站点的提交日志；shared 表示日志文件被多个站点共用。
func (s *Store) recoverSiteJournal(ctx context.Context, shared bool) error {
	var site sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT site_id FROM store_identity WHERE singleton=1`).Scan(&site); err != nil {
		return contentStorageError()
	}
	path := filepath.Join(s.root, journalRelativePath)
	if err := lifecycle.ValidatePrivatePath(path, "regular-file", true); err != nil {
		return err
	}
	if !site.Valid {
		if shared {
			return nil
		}
		if info, err := os.Lstat(path); err == nil && info.Size() != 0 {
			return platform.NewError(platform.CodeIntegrityError, false, "未绑定站点的正文库存在提交日志")
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return contentStorageError()
		}
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM revision_preparations)+(SELECT COUNT(*) FROM committed_revisions)+(SELECT COUNT(*) FROM draft_heads)+(SELECT COUNT(*) FROM documents)+(SELECT COUNT(*) FROM draft_operation_intents)+(SELECT COUNT(*) FROM content_tree_state)`).Scan(&count); err != nil {
			return contentStorageError()
		}
		if count != 0 {
			return platform.NewError(platform.CodeIntegrityError, false, "未绑定站点的正文库存在权威记录")
		}
		return nil
	}
	if !platform.ValidIdentifier(site.String) || site.String == "." || site.String == ".." {
		return platform.NewError(platform.CodeIntegrityError, false, "正文持久站点绑定不合法")
	}
	work, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	op := OperationContext{RequestID: platform.NewIdentifier(), OperationID: platform.NewIdentifier(), CallerService: "content", Actor: platform.AuthContext{Kind: "service", SubjectID: "content", SiteID: site.String}, DataEpoch: s.epoch, Deadline: time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339Nano)}
	recovery := RevisionRecoveryStore{ActiveRoot: s.root, SiteID: site.String, DataEpoch: s.epoch, State: s, ExclusiveStartup: true}
	if shared {
		recovery.Owns = func(ctx context.Context, commitID Identifier) (bool, error) {
			var n int
			if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM revision_preparations WHERE commit_id=?`, commitID).Scan(&n); err != nil {
				return false, contentStorageError()
			}
			return n > 0, nil
		}
	}
	if err := s.validatePreparedJournalCoverage(work, recovery); err != nil {
		return err
	}
	_, err := RecoverPreparedCommits(WithRevisionRecoveryStore(work, recovery), op, RecoverPreparedCommitsInput{})
	if err != nil {
		return err
	}
	entries, invalid, err := scanCommitJournal(ctx, recovery, path)
	if err != nil {
		return err
	}
	if invalid {
		return platform.NewError(platform.CodeIntegrityError, false, "正文恢复日志不合法")
	}
	for _, entry := range entries {
		if entry.journal.State != "quarantined" && entry.journal.State != "aborted" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE revision_preparations SET state=? WHERE commit_id=? AND site_id=? AND data_epoch=?`, entry.journal.State, entry.journal.CommitID, site.String, s.epoch); err != nil {
			return contentStorageError()
		}
	}
	return nil
}

// validatePreparedJournalCoverage反向核对私有持久决定，不能因日志丢失或截空漏扫。
// 只允许尚在preparing且无提交资格的首次材料没有日志；不猜补日志。
func (s *Store) validatePreparedJournalCoverage(ctx context.Context, recovery RevisionRecoveryStore) error {
	entries, invalid, err := scanCommitJournal(ctx, recovery, filepath.Join(s.root, journalRelativePath))
	if err != nil {
		return err
	}
	if invalid {
		return platform.NewError(platform.CodeIntegrityError, false, "正文提交日志不合法")
	}
	journals := make(map[Identifier]CommitJournal, len(entries))
	for _, entry := range entries {
		journals[entry.journal.CommitID] = entry.journal
	}
	// journal读锁已释放，之后才打开SQL快照；不持SQL事务跨文件IO。
	rows, err := s.db.QueryContext(ctx, `SELECT p.commit_id,p.document_id,p.revision_id,p.state,p.request_hash,p.result_json,p.caller_service,p.actor_id,p.site_id,p.data_epoch,p.operation_id FROM revision_preparations p WHERE p.state IN ('prepared','committed') OR EXISTS(SELECT 1 FROM committed_revisions c WHERE c.commit_id=p.commit_id AND c.state='committed')`)
	if err != nil {
		return contentStorageError()
	}
	defer rows.Close()
	for rows.Next() {
		var row preparationRow
		var raw []byte
		var op OperationContext
		if err := rows.Scan(&row.commitID, &row.documentID, &row.revisionID, &row.state, &row.digest, &raw, &op.CallerService, &op.Actor.SubjectID, &op.Actor.SiteID, &op.DataEpoch, &op.OperationID); err != nil {
			return contentStorageError()
		}
		if op.Actor.SiteID != recovery.SiteID || op.DataEpoch != s.epoch {
			return platform.NewError(platform.CodeIntegrityError, false, "正文持久准备站点或代次不一致")
		}
		if err := decodePreparation(raw, &row, op); err != nil {
			return err
		}
		journal, found := journals[row.commitID]
		if !found || !sameJournalIdentity(row.result.Journal, journal) {
			return platform.NewError(platform.CodeIntegrityError, false, "正文持久准备或提交决定缺少完整匹配日志")
		}
	}
	if err := rows.Err(); err != nil {
		return contentStorageError()
	}
	return nil
}

func (s *Store) preparationJournalState(ctx context.Context, expected CommitJournal) (string, bool, error) {
	path := filepath.Join(s.root, journalRelativePath)
	revisionJournalMu.RLock()
	defer revisionJournalMu.RUnlock()
	file, err := openRevisionJournal(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, normalizeDraftCommitError(err, "正文提交日志不可读")
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), maxJournalLineBytes)
	states := map[Identifier]CommitJournal{}
	for scanner.Scan() {
		if err := checkContextOnly(ctx); err != nil {
			return "", false, err
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		journal, err := decodeCommitJournal(line)
		if err != nil {
			return "", false, platform.NewError(platform.CodeIntegrityError, false, "正文提交日志损坏")
		}
		if previous, found := states[journal.CommitID]; found && (!sameJournalIdentity(previous, journal) || !validJournalTransition(previous.State, journal.State)) {
			return "", false, platform.NewError(platform.CodeIntegrityError, false, "正文提交日志状态矛盾")
		}
		states[journal.CommitID] = journal
	}
	if err := scanner.Err(); err != nil {
		return "", false, contentStorageError()
	}
	if err := validateRevisionJournalFile(file); err != nil {
		return "", false, err
	}
	journal, found := states[expected.CommitID]
	if found && !sameJournalIdentity(expected, journal) {
		return "", false, platform.NewError(platform.CodeIntegrityError, false, "正文日志与私有准备材料矛盾")
	}
	return journal.State, found, nil
}

func decodePreparation(raw []byte, row *preparationRow, op OperationContext) error {
	if err := decodeStorageJSON(raw, &row.result); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "正文准备原材料不可解码")
	}
	result := row.result
	if result.Journal.CommitID != row.commitID || result.Journal.DocumentID != row.documentID || result.Journal.RevisionID != row.revisionID || result.Draft.DocumentId != row.documentID || result.Draft.RevisionId != row.revisionID || result.Draft.AuthorAccountId != op.Actor.SubjectID || result.Draft.Object.ObjectId != row.revisionID || result.Draft.Object.Version != 1 || Timestamp(result.Draft.CreatedAt.UTC().Format(time.RFC3339Nano)) != result.Journal.CreatedAt {
		return platform.NewError(platform.CodeIntegrityError, false, "正文准备原材料归属不一致")
	}
	input := CommitDraftJournalInput{Draft: result.Draft, Journal: result.Journal, ExpectedHead: result.Journal.ExpectedHead, ExpectedDraftVersion: result.Draft.DraftVersion - 1}
	return validateDraftCommitInput(op, input)
}

func decodeStorageJSON(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("正文私有记录包含额外JSON")
	}
	return nil
}

func sameStoredDraft(left, right DraftRevision) bool {
	l, _ := json.Marshal(left)
	r, _ := json.Marshal(right)
	return bytes.Equal(l, r)
}
func contentStorageError() error {
	return platform.NewError(platform.CodeDependencyUnavailable, true, "正文私有存储不可用")
}
func (s *Store) CloseDatabase() error { return s.db.Close() }

// SiteID 读取私有存储绑定的站点；尚无任何写入（未绑定）时返回 false。
func (s *Store) SiteID(ctx context.Context) (Identifier, bool) {
	var site sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT site_id FROM store_identity WHERE singleton=1`).Scan(&site); err != nil || !site.Valid {
		return "", false
	}
	return site.String, true
}

// Sites 列出内容库里出现过的站点；后台循环按它逐站点处理。
func (s *Store) Sites(ctx context.Context) []Identifier {
	sites, err := pgdb.Sites(ctx, s.db)
	if err != nil {
		return nil
	}
	return sites
}

// ServiceOperation 构造后台循环使用的本域服务操作上下文。
func ServiceOperation(site, epoch Identifier) OperationContext {
	id := platform.NewIdentifier()
	deadline := platform.Timestamp(time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano))
	return OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: "content", Deadline: deadline, DataEpoch: epoch,
		Actor: platform.AuthContext{Kind: "service", SubjectID: "content", SiteID: site, Grants: []string{}, ExpiresAt: deadline}}
}
