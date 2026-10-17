package siteconfig

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
	"strings"
	"time"
	"unicode"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

const (
	announcementJournalName    = "announcement.jsonl"
	announcementJournalSchema  = 1
	announcementStatePrepared  = "PREPARED"
	announcementStateCommitted = "COMMITTED"
)

// announcementJournalEntry 是 active journal 的内部记录格式。它只保存
// 不可变对象引用和公告元数据，不保存公告正文，正文仍由 object 引用定位。
type announcementJournalEntry struct {
	SchemaVersion      int            `json:"schemaVersion"`
	State              string         `json:"state"`
	AnnouncementID     gen.Identifier `json:"announcementId"`
	SiteID             gen.Identifier `json:"siteId"`
	RevisionID         gen.Identifier `json:"revisionId"`
	BodyHash           gen.Hash       `json:"bodyHash"`
	Object             gen.ObjectRef  `json:"object"`
	Revision           gen.Revision   `json:"revision"`
	VisibilityRevision gen.Revision   `json:"visibilityRevision"`
	Enabled            bool           `json:"enabled"`
	StartsAt           *gen.Timestamp `json:"startsAt"`
	EndsAt             *gen.Timestamp `json:"endsAt"`
	UpdatedAt          gen.Timestamp  `json:"updatedAt"`
}

// RecoverAnnouncementJournalInput 是恢复公告正文文件与提交日志的输入；恢复参数由
// 受信启动装配注入，不接受请求字段。
type RecoverAnnouncementJournalInput struct{}

// RecoverAnnouncementJournalResult 是一次公告journal恢复的统计结果。
type RecoverAnnouncementJournalResult struct {
	CommittedCount   int `json:"committedCount"`
	QuarantinedCount int `json:"quarantinedCount"`
}

// RecoverAnnouncementJournal 在打开业务入口前恢复公告决定日志。COMMITTED
// 记录必须同时通过 head、对象文件、大小、hash 和正文 schema 校验；孤儿
// PREPARED 记录只进入隔离区，绝不推定为已发布。
func RecoverAnnouncementJournal(ctx context.Context, op platform.OperationContext, input RecoverAnnouncementJournalInput) (RecoverAnnouncementJournalResult, error) {
	_ = input
	if err := checkOperationDeadline(ctx, op); err != nil {
		return RecoverAnnouncementJournalResult{}, err
	}
	store := currentActiveStore()
	if store == nil {
		return RecoverAnnouncementJournalResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	entries, journalHash, err := readAnnouncementJournal(store.activeRoot)
	if err != nil {
		return RecoverAnnouncementJournalResult{}, err
	}
	committed := make(map[string]announcementJournalEntry)
	// committedHistory 保存每条公告曾提交过的全部修订；被后续修订取代的
	// PREPARED 只要内容与某次 COMMITTED 一致，就不是孤儿。
	committedHistory := make(map[string][]announcementJournalEntry)
	prepared := make([]announcementJournalEntry, 0)
	for _, entry := range entries {
		if err := validateAnnouncementJournalEntry(entry); err != nil {
			return RecoverAnnouncementJournalResult{}, err
		}
		key := string(entry.AnnouncementID)
		switch entry.State {
		case announcementStatePrepared:
			prepared = append(prepared, entry)
		case announcementStateCommitted:
			previous, exists := committed[key]
			if exists {
				if entry.Revision < previous.Revision {
					return RecoverAnnouncementJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告journal修订顺序不合法")
				}
				if entry.Revision == previous.Revision && !sameAnnouncementEntry(previous, entry) {
					return RecoverAnnouncementJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告journal同修订内容不一致")
				}
			}
			committed[key] = entry
			committedHistory[key] = append(committedHistory[key], entry)
		}
	}

	for _, entry := range committed {
		if err := validateCommittedAnnouncement(store.activeRoot, entry); err != nil {
			return RecoverAnnouncementJournalResult{}, err
		}
		if err := store.upsertAnnouncementHead(ctx, entry); err != nil {
			return RecoverAnnouncementJournalResult{}, err
		}
	}
	for i := range prepared {
		for _, entry := range committedHistory[string(prepared[i].AnnouncementID)] {
			if sameAnnouncementEntry(prepared[i], entry) {
				prepared[i].State = "already_committed"
				break
			}
		}
	}

	quarantined := make([]announcementJournalEntry, 0)
	for _, entry := range prepared {
		if entry.State == "already_committed" {
			continue
		}
		quarantined = append(quarantined, entry)
	}
	if len(quarantined) > 0 {
		if err := quarantinePreparedAnnouncements(store.activeRoot, quarantined); err != nil {
			return RecoverAnnouncementJournalResult{}, err
		}
	}
	store.markJournalRecovered(journalHash)
	return RecoverAnnouncementJournalResult{
		CommittedCount:   len(committed),
		QuarantinedCount: len(quarantined),
	}, nil
}

func readAnnouncementJournal(activeRoot string) ([]announcementJournalEntry, platform.Hash, error) {
	path := filepath.Join(activeRoot, "journal", announcementJournalName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []announcementJournalEntry{}, platform.HashBytes(nil), nil
	}
	if err != nil {
		return nil, "", platform.NewError(platform.CodeDependencyUnavailable, true, "公告journal不可读")
	}
	entries := make([]announcementJournalEntry, 0)
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			return nil, "", platform.NewError(platform.CodeIntegrityError, false, "公告journal含空记录")
		}
		var entry announcementJournalEntry
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&entry); err != nil {
			return nil, "", platform.NewError(platform.CodeIntegrityError, false, "公告journal记录格式不合法")
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			return nil, "", platform.NewError(platform.CodeIntegrityError, false, "公告journal记录包含多余内容")
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "公告journal记录过大或读取失败")
	}
	return entries, platform.HashBytes(raw), nil
}

func readAnnouncementJournalEntry(path string) (announcementJournalEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return announcementJournalEntry{}, platform.NewError(platform.CodeIntegrityError, false, "公告head不可读")
	}
	var entry announcementJournalEntry
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&entry); err != nil {
		return announcementJournalEntry{}, platform.NewError(platform.CodeIntegrityError, false, "公告head格式不合法")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return announcementJournalEntry{}, platform.NewError(platform.CodeIntegrityError, false, "公告head包含多余内容")
	}
	return entry, nil
}

func validateAnnouncementJournalEntry(entry announcementJournalEntry) error {
	if entry.SchemaVersion != announcementJournalSchema ||
		(entry.State != announcementStatePrepared && entry.State != announcementStateCommitted) ||
		!platform.ValidIdentifier(string(entry.AnnouncementID)) ||
		!platform.ValidIdentifier(string(entry.SiteID)) ||
		!platform.ValidIdentifier(string(entry.RevisionID)) ||
		!platform.ValidHash(platform.Hash(entry.BodyHash)) ||
		!platform.ValidIdentifier(string(entry.Object.ObjectId)) ||
		!platform.ValidHash(platform.Hash(entry.Object.Sha256)) ||
		entry.Object.Owner != "site-config" ||
		entry.Object.ObjectId != entry.RevisionID ||
		entry.Object.Sha256 != entry.BodyHash ||
		entry.Object.ByteSize <= 0 || entry.Object.Version < 1 ||
		entry.Object.MediaType != "application/json" ||
		entry.Revision < 1 || entry.VisibilityRevision < 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "公告journal字段不合法")
	}
	if entry.UpdatedAt.IsZero() {
		return platform.NewError(platform.CodeIntegrityError, false, "公告journal时间不合法")
	}
	if err := validateOptionalTimestamp(entry.StartsAt); err != nil {
		return err
	}
	if err := validateOptionalTimestamp(entry.EndsAt); err != nil {
		return err
	}
	if entry.StartsAt != nil && entry.EndsAt != nil {
		if entry.EndsAt.Before(*entry.StartsAt) {
			return platform.NewError(platform.CodeIntegrityError, false, "公告时间窗不合法")
		}
	}
	return nil
}
func validateOptionalTimestamp(value *gen.Timestamp) error {
	if value != nil && value.IsZero() {
		return platform.NewError(platform.CodeIntegrityError, false, "公告时间窗不合法")
	}
	return nil
}

func validateCommittedAnnouncement(activeRoot string, entry announcementJournalEntry) error {
	headPath := filepath.Join(activeRoot, "announcements", string(entry.AnnouncementID), "head.json")
	head, err := readAnnouncementJournalEntry(headPath)
	if err != nil {
		return err
	}
	if head.State != announcementStateCommitted || !sameAnnouncementEntry(head, entry) {
		return platform.NewError(platform.CodeIntegrityError, false, "公告head与COMMITTED记录不一致")
	}
	objectPath := filepath.Join(activeRoot, "announcements", string(entry.AnnouncementID), string(entry.RevisionID)+".json")
	body, err := os.ReadFile(objectPath)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "公告正文对象缺失")
	}
	if len(body) != entry.Object.ByteSize || platform.HashBytes(body) != entry.Object.Sha256 {
		return platform.NewError(platform.CodeIntegrityError, false, "公告正文对象hash或大小不一致")
	}
	if err := validateAnnouncementBody(body); err != nil {
		return err
	}
	return nil
}

func validateAnnouncementBody(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "公告正文不是合法JSON")
	}
	for key := range fields {
		if key != "schemaVersion" && key != "text" && key != "target" {
			return platform.NewError(platform.CodeIntegrityError, false, "公告正文包含未知字段")
		}
	}
	var schemaVersion int
	var text string
	if err := json.Unmarshal(fields["schemaVersion"], &schemaVersion); err != nil || schemaVersion != announcementJournalSchema {
		return platform.NewError(platform.CodeIntegrityError, false, "公告正文schema版本不支持")
	}
	if err := json.Unmarshal(fields["text"], &text); err != nil || len([]rune(text)) < 1 || len([]rune(text)) > 2000 {
		return platform.NewError(platform.CodeIntegrityError, false, "公告正文长度不合法")
	}
	target, exists := fields["target"]
	if !exists {
		return platform.NewError(platform.CodeIntegrityError, false, "公告正文缺少target")
	}
	if string(bytes.TrimSpace(target)) != "null" {
		var targetFields map[string]json.RawMessage
		if err := json.Unmarshal(target, &targetFields); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "公告链接格式不合法")
		}
		for key := range targetFields {
			if key != "kind" && key != "blockId" && key != "documentId" && key != "url" {
				return platform.NewError(platform.CodeIntegrityError, false, "公告链接包含未知字段")
			}
		}
		if _, ok := targetFields["kind"]; !ok {
			return platform.NewError(platform.CodeIntegrityError, false, "公告链接缺少kind")
		}
		var kind string
		if err := json.Unmarshal(targetFields["kind"], &kind); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "公告链接kind不合法")
		}
		switch kind {
		case "document":
			if len(targetFields) != 3 {
				return platform.NewError(platform.CodeIntegrityError, false, "站内公告链接字段不完整")
			}
			var documentID string
			if err := json.Unmarshal(targetFields["documentId"], &documentID); err != nil || !platform.ValidIdentifier(documentID) {
				return platform.NewError(platform.CodeIntegrityError, false, "站内公告链接documentId不合法")
			}
			if blockID, ok := targetFields["blockId"]; !ok {
				return platform.NewError(platform.CodeIntegrityError, false, "站内公告链接缺少blockId")
			} else if string(bytes.TrimSpace(blockID)) != "null" {
				var value string
				if err := json.Unmarshal(blockID, &value); err != nil || !platform.ValidIdentifier(value) {
					return platform.NewError(platform.CodeIntegrityError, false, "站内公告链接blockId不合法")
				}
			}
		case "qa":
			if len(targetFields) != 1 {
				return platform.NewError(platform.CodeIntegrityError, false, "问答公告链接字段不完整")
			}
		case "external":
			if len(targetFields) != 2 {
				return platform.NewError(platform.CodeIntegrityError, false, "外链公告链接字段不完整")
			}
			var value string
			if err := json.Unmarshal(targetFields["url"], &value); err != nil || len(value) < 1 || len(value) > 2048 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
				return platform.NewError(platform.CodeIntegrityError, false, "外链公告链接url不合法")
			}
			parsed, err := url.Parse(value)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "mailto") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || ((parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host == "") {
				return platform.NewError(platform.CodeIntegrityError, false, "外链公告链接安全校验失败")
			}
		default:
			return platform.NewError(platform.CodeIntegrityError, false, "公告链接kind不支持")
		}
	}
	return nil
}

func sameAnnouncementEntry(left, right announcementJournalEntry) bool {
	left.State = right.State
	return string(mustMarshalJournalEntry(left)) == string(mustMarshalJournalEntry(right))
}

func mustMarshalJournalEntry(entry announcementJournalEntry) []byte {
	raw, _ := json.Marshal(entry)
	return raw
}

func quarantinePreparedAnnouncements(activeRoot string, entries []announcementJournalEntry) error {
	if len(entries) == 0 {
		return nil
	}
	raw := bytes.NewBuffer(nil)
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return platform.NewError(platform.CodeInternal, false, "编码公告隔离记录失败")
		}
		raw.Write(line)
		raw.WriteByte('\n')
	}
	hash := platform.HashBytes(raw.Bytes())
	path := filepath.Join(activeRoot, "quarantine", "announcement", "prepared-"+string(hash)+".jsonl")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "公告孤儿隔离文件不可写")
	}
	if _, err := file.Write(raw.Bytes()); err != nil {
		_ = file.Close()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "公告孤儿隔离失败")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "公告孤儿隔离未持久化")
	}
	if err := file.Close(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "公告孤儿隔离文件关闭失败")
	}
	return nil
}

func readAnnouncementJournalHash(activeRoot string) (platform.Hash, error) {
	path := filepath.Join(activeRoot, "journal", announcementJournalName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return platform.HashBytes(nil), nil
	}
	if err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "公告journal不可读")
	}
	return platform.HashBytes(raw), nil
}

func nullableTimeString(value *gen.Timestamp) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

type announcementExec interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *ActiveStore) upsertAnnouncementHead(ctx context.Context, entry announcementJournalEntry) error {
	return upsertAnnouncementHeadOn(ctx, s.db, entry)
}

func upsertAnnouncementHeadOn(ctx context.Context, q announcementExec, entry announcementJournalEntry) error {
	var currentRevision int64
	var currentRevisionID, currentBodyHash string
	err := q.QueryRowContext(ctx, `
        SELECT revision, revision_id, body_hash
          FROM announcement_heads WHERE announcement_id = ?`,
		string(entry.AnnouncementID)).Scan(&currentRevision, &currentRevisionID, &currentBodyHash)
	if err == nil {
		if currentRevision > int64(entry.Revision) ||
			(currentRevision == int64(entry.Revision) &&
				(currentRevisionID != string(entry.RevisionID) || currentBodyHash != string(entry.BodyHash))) {
			return platform.NewError(platform.CodeIntegrityError, false, "公告head水位与journal冲突")
		}
		if currentRevision == int64(entry.Revision) {
			return nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "读取公告head失败")
	}
	_, err = q.ExecContext(ctx, `
        INSERT INTO announcement_heads
            (announcement_id, site_id, revision_id, object_id, body_hash,
             revision, visibility_revision, enabled, starts_at, ends_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT (announcement_id) DO UPDATE SET
            site_id = excluded.site_id,
            revision_id = excluded.revision_id,
            object_id = excluded.object_id,
            body_hash = excluded.body_hash,
            revision = excluded.revision,
            visibility_revision = excluded.visibility_revision,
            enabled = excluded.enabled,
            starts_at = excluded.starts_at,
            ends_at = excluded.ends_at,
            updated_at = excluded.updated_at`,
		string(entry.AnnouncementID), string(entry.SiteID), string(entry.RevisionID),
		string(entry.Object.ObjectId), string(entry.BodyHash), int64(entry.Revision),
		int64(entry.VisibilityRevision), boolInt(entry.Enabled), nullableTimeString(entry.StartsAt),
		nullableTimeString(entry.EndsAt), entry.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "保存公告head失败")
	}
	return nil
}

type WriteAnnouncementRevisionInput struct {
	AnnouncementId gen.Identifier       `json:"announcementId"`
	RevisionId     gen.Identifier       `json:"revisionId"`
	Body           gen.AnnouncementBody `json:"body"`
	BodyHash       gen.Hash             `json:"bodyHash"`
}

type WriteAnnouncementRevisionResult struct {
	Object gen.ObjectRef `json:"object"`
}

type CommitAnnouncementHeadInput struct {
	AnnouncementId          gen.Identifier  `json:"announcementId"`
	Object                  gen.ObjectRef   `json:"object"`
	ExpectedDraftRevisionId *gen.Identifier `json:"expectedDraftRevisionId"`
}

type CommitAnnouncementHeadResult struct {
	Committed bool         `json:"committed"`
	Revision  gen.Revision `json:"revision"`
}

type announcementPreparedFile struct {
	SchemaVersion  int    `json:"schemaVersion"`
	AnnouncementID string `json:"announcementId"`
	RevisionID     string `json:"revisionId"`
	BodyHash       string `json:"bodyHash"`
	ByteSize       int    `json:"byteSize"`
}

type announcementCommitScope struct {
	tx     *sql.Tx
	record configJournalRecord
	config gen.SiteConfig
}

type announcementCommitKey struct{}

func announcementBodyPath(root, announcementID, revisionID string) string {
	return filepath.Join(root, "announcements", announcementID, revisionID+".json")
}

func announcementHeadPath(root, announcementID string) string {
	return filepath.Join(root, "announcements", announcementID, "head.json")
}

func announcementPreparedPath(root, revisionID string) string {
	return filepath.Join(root, "journal", "announcement", revisionID+".prepared.json")
}

func readRegularConfigFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, platform.NewError(platform.CodeDependencyUnavailable, true, "公告文件状态不可读")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false, platform.NewError(platform.CodeIntegrityError, false, "公告文件不是正规文件")
	}
	raw, err := readConfigFile(path)
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

// WriteAnnouncementRevision 只写不可变正文和PREPARED旁路，不公开head。
func WriteAnnouncementRevision(ctx context.Context, op platform.OperationContext, input WriteAnnouncementRevisionInput) (WriteAnnouncementRevisionResult, error) {
	_ = ctx
	_ = op
	store := currentActiveStore()
	if store == nil {
		return WriteAnnouncementRevisionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	if !configPathID(string(input.AnnouncementId)) || !configPathID(string(input.RevisionId)) || !platform.ValidHash(input.BodyHash) {
		return WriteAnnouncementRevisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告正文标识不合法")
	}
	raw, err := canonicalAnnouncementBody(input.Body)
	if err != nil {
		return WriteAnnouncementRevisionResult{}, err
	}
	if platform.HashBytes(raw) != input.BodyHash {
		return WriteAnnouncementRevisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告正文hash与规范字节不一致")
	}
	object := gen.ObjectRef{Owner: "site-config", ObjectId: input.RevisionId, Version: 1, Sha256: input.BodyHash, ByteSize: len(raw), MediaType: "application/json"}
	store.configMu.Lock()
	defer store.configMu.Unlock()
	path := announcementBodyPath(store.activeRoot, string(input.AnnouncementId), string(input.RevisionId))
	existing, exists, err := readRegularConfigFile(path)
	if err != nil {
		return WriteAnnouncementRevisionResult{}, err
	}
	if exists && !bytes.Equal(existing, raw) {
		return WriteAnnouncementRevisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告正文被异内容复用")
	}
	if !exists {
		if err := writeConfigFile(path, raw, true); err != nil {
			return WriteAnnouncementRevisionResult{}, err
		}
	}
	preparedRaw, err := json.Marshal(announcementPreparedFile{SchemaVersion: announcementJournalSchema, AnnouncementID: string(input.AnnouncementId), RevisionID: string(input.RevisionId), BodyHash: string(input.BodyHash), ByteSize: len(raw)})
	if err != nil {
		return WriteAnnouncementRevisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告PREPARED记录无法编码")
	}
	if err := writeConfigFile(announcementPreparedPath(store.activeRoot, string(input.RevisionId)), preparedRaw, true); err != nil {
		return WriteAnnouncementRevisionResult{}, err
	}
	return WriteAnnouncementRevisionResult{Object: object}, nil
}

// CommitAnnouncementHead 在配置事务内提交公告；事务外只核正文与预期head。
func CommitAnnouncementHead(ctx context.Context, op platform.OperationContext, input CommitAnnouncementHeadInput) (CommitAnnouncementHeadResult, error) {
	_ = op
	store := currentActiveStore()
	if store == nil {
		return CommitAnnouncementHeadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	if !configPathID(string(input.AnnouncementId)) || input.Object.Owner != "site-config" || input.Object.ObjectId == "" || !platform.ValidHash(input.Object.Sha256) || input.Object.ByteSize < 1 || input.Object.MediaType != "application/json" {
		return CommitAnnouncementHeadResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告提交对象不合法")
	}
	body, exists, err := readRegularConfigFile(announcementBodyPath(store.activeRoot, string(input.AnnouncementId), string(input.Object.ObjectId)))
	if err != nil {
		return CommitAnnouncementHeadResult{}, err
	}
	if !exists || len(body) != input.Object.ByteSize || platform.HashBytes(body) != input.Object.Sha256 || validateAnnouncementBody(body) != nil {
		return CommitAnnouncementHeadResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告正文与对象引用不一致")
	}
	head, headExists, err := readAnnouncementHead(store.activeRoot, string(input.AnnouncementId))
	if err != nil {
		return CommitAnnouncementHeadResult{}, err
	}
	scope, scoped := ctx.Value(announcementCommitKey{}).(announcementCommitScope)
	if !scoped {
		if headExists && head.RevisionID == input.Object.ObjectId && head.BodyHash == input.Object.Sha256 && head.Object.ByteSize == input.Object.ByteSize {
			return CommitAnnouncementHeadResult{Committed: true, Revision: head.Revision}, nil
		}
		if err := checkExpectedAnnouncementHead(head, headExists, input.ExpectedDraftRevisionId); err != nil {
			return CommitAnnouncementHeadResult{}, err
		}
		revision := input.Object.Version
		if revision < 1 {
			revision = 1
		}
		return CommitAnnouncementHeadResult{Committed: false, Revision: revision}, nil
	}
	if scope.tx == nil || scope.config.Banner == nil || scope.config.Banner.AnnouncementId != input.AnnouncementId || scope.config.Banner.RevisionId != input.Object.ObjectId || scope.config.Banner.BodyHash != input.Object.Sha256 {
		return CommitAnnouncementHeadResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告提交与配置横幅不一致")
	}
	if scope.config.Banner.Revision != scope.config.Revision || scope.record.Object.Version != scope.config.Revision {
		return CommitAnnouncementHeadResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告修订与配置水位不一致")
	}
	entry := announcementJournalEntry{
		SchemaVersion: announcementJournalSchema, State: announcementStateCommitted, AnnouncementID: scope.config.Banner.AnnouncementId,
		SiteID: scope.config.Banner.SiteId, RevisionID: scope.config.Banner.RevisionId, BodyHash: scope.config.Banner.BodyHash, Object: input.Object,
		Revision: scope.config.Revision, VisibilityRevision: scope.config.Banner.VisibilityRevision, Enabled: scope.config.Banner.Enabled,
		StartsAt: scope.config.Banner.StartsAt, EndsAt: scope.config.Banner.EndsAt, UpdatedAt: scope.config.UpdatedAt.UTC(),
	}
	if err := validateAnnouncementJournalEntry(entry); err != nil {
		return CommitAnnouncementHeadResult{}, err
	}
	if headExists && sameAnnouncementEntry(head, entry) {
		if err := upsertAnnouncementHeadOn(ctx, scope.tx, entry); err != nil {
			return CommitAnnouncementHeadResult{}, err
		}
		return CommitAnnouncementHeadResult{Committed: true, Revision: entry.Revision}, nil
	}
	if err := checkExpectedAnnouncementHead(head, headExists, input.ExpectedDraftRevisionId); err != nil {
		return CommitAnnouncementHeadResult{}, err
	}
	if headExists && head.Revision >= entry.Revision {
		return CommitAnnouncementHeadResult{}, platform.NewError(platform.CodeVersionMismatch, false, "公告head水位不低于本次配置修订")
	}
	if err := announcementBodyPrepared(store.activeRoot, entry); err != nil {
		return CommitAnnouncementHeadResult{}, err
	}
	if err := appendAnnouncementCommitted(store.activeRoot, entry); err != nil {
		return CommitAnnouncementHeadResult{}, err
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return CommitAnnouncementHeadResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告head无法编码")
	}
	if err := writeConfigFile(announcementHeadPath(store.activeRoot, string(entry.AnnouncementID)), raw, false); err != nil {
		return CommitAnnouncementHeadResult{}, err
	}
	if err := upsertAnnouncementHeadOn(ctx, scope.tx, entry); err != nil {
		return CommitAnnouncementHeadResult{}, err
	}
	return CommitAnnouncementHeadResult{Committed: true, Revision: entry.Revision}, nil
}

func checkExpectedAnnouncementHead(head announcementJournalEntry, exists bool, expected *gen.Identifier) error {
	if expected == nil {
		if exists {
			return platform.NewError(platform.CodeVersionMismatch, false, "公告head已存在，不能当作新建")
		}
		return nil
	}
	if !exists {
		return platform.NewError(platform.CodeIntegrityError, false, "当前公告head缺失")
	}
	if head.RevisionID != *expected {
		return platform.NewError(platform.CodeVersionMismatch, false, "公告head与预期草稿修订不一致")
	}
	return nil
}

func readAnnouncementHead(root, announcementID string) (announcementJournalEntry, bool, error) {
	path := announcementHeadPath(root, announcementID)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return announcementJournalEntry{}, false, nil
	}
	if err != nil {
		return announcementJournalEntry{}, false, platform.NewError(platform.CodeDependencyUnavailable, true, "公告head状态不可读")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return announcementJournalEntry{}, false, platform.NewError(platform.CodeIntegrityError, false, "公告head不是正规文件")
	}
	entry, err := readAnnouncementJournalEntry(path)
	if err != nil {
		return announcementJournalEntry{}, false, err
	}
	if err := validateAnnouncementJournalEntry(entry); err != nil {
		return announcementJournalEntry{}, false, err
	}
	return entry, true, nil
}

func announcementBodyPrepared(root string, entry announcementJournalEntry) error {
	entries, _, err := readAnnouncementJournal(root)
	if err != nil {
		return err
	}
	for _, existing := range entries {
		if existing.State == announcementStateCommitted && existing.AnnouncementID == entry.AnnouncementID && existing.RevisionID == entry.RevisionID && existing.BodyHash == entry.BodyHash {
			return nil
		}
		if sameAnnouncementEntry(existing, entry) {
			return nil
		}
	}
	raw, exists, err := readRegularConfigFile(announcementPreparedPath(root, string(entry.RevisionID)))
	if err != nil {
		return err
	}
	if !exists {
		return platform.NewError(platform.CodeIntegrityError, false, "公告正文尚未准备，不能提交")
	}
	var prepared announcementPreparedFile
	if decodeConfigJSON(raw, &prepared) != nil || prepared.SchemaVersion != announcementJournalSchema || prepared.AnnouncementID != string(entry.AnnouncementID) || prepared.RevisionID != string(entry.RevisionID) || prepared.BodyHash != string(entry.BodyHash) || prepared.ByteSize != entry.Object.ByteSize {
		return platform.NewError(platform.CodeIntegrityError, false, "公告PREPARED与正文不一致")
	}
	return nil
}

func appendAnnouncementCommitted(root string, entry announcementJournalEntry) error {
	path := filepath.Join(root, "journal", announcementJournalName)
	existing, exists, err := readRegularConfigFile(path)
	if err != nil {
		return err
	}
	if exists {
		entries, _, err := readAnnouncementJournal(root)
		if err != nil {
			return err
		}
		for _, current := range entries {
			if current.State == announcementStateCommitted && sameAnnouncementEntry(current, entry) {
				return nil
			}
		}
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "公告COMMITTED记录无法编码")
	}
	next := append([]byte{}, existing...)
	if len(next) > 0 && next[len(next)-1] != '\n' {
		next = append(next, '\n')
	}
	next = append(next, line...)
	next = append(next, '\n')
	return writeConfigFile(path, next, false)
}

func (s *ActiveStore) previousBanner(ctx context.Context, tx *sql.Tx, record configJournalRecord) (*gen.Announcement, error) {
	if record.ExpectedRevision < 1 {
		return nil, nil
	}
	var revisionID string
	if err := tx.QueryRowContext(ctx, `SELECT revision_id FROM config_revision_index WHERE site_id=? AND revision=?`, record.SiteID, record.ExpectedRevision).Scan(&revisionID); err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "上一版配置索引缺失")
	}
	config, _, _, err := s.readIndexedConfig(ctx, tx, revisionID)
	if err != nil {
		return nil, err
	}
	return config.Banner, nil
}

func (s *ActiveStore) finishAnnouncementCommit(ctx context.Context, tx *sql.Tx, record configJournalRecord, config gen.SiteConfig) error {
	if config.Banner == nil {
		return nil
	}
	previous, err := s.previousBanner(ctx, tx, record)
	if err != nil {
		return err
	}
	if sameBannerProjection(previous, config.Banner) {
		return nil
	}
	body, exists, err := readRegularConfigFile(announcementBodyPath(s.activeRoot, string(config.Banner.AnnouncementId), string(config.Banner.RevisionId)))
	if err != nil {
		return err
	}
	if !exists || platform.HashBytes(body) != config.Banner.BodyHash {
		return platform.NewError(platform.CodeIntegrityError, false, "公告正文与配置引用不一致")
	}
	var expected *gen.Identifier
	if previous != nil && previous.AnnouncementId == config.Banner.AnnouncementId {
		id := previous.RevisionId
		expected = &id
	}
	object := gen.ObjectRef{Owner: "site-config", ObjectId: config.Banner.RevisionId, Version: 1, Sha256: config.Banner.BodyHash, ByteSize: len(body), MediaType: "application/json"}
	scoped := context.WithValue(ctx, announcementCommitKey{}, announcementCommitScope{tx: tx, record: record, config: config})
	_, err = CommitAnnouncementHead(scoped, platform.OperationContext{OperationID: record.OperationID, DataEpoch: record.DataEpoch}, CommitAnnouncementHeadInput{AnnouncementId: config.Banner.AnnouncementId, Object: object, ExpectedDraftRevisionId: expected})
	return err
}
