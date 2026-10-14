package siteconfig

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

const (
	announcementJournalName   = "announcement.jsonl"
	announcementJournalSchema = 1
	announcementStatePrepared = "PREPARED"
	announcementStateCommitted = "COMMITTED"
)

// announcementJournalEntry 是 active journal 的内部记录格式。它只保存
// 不可变对象引用和公告元数据，不保存公告正文，正文仍由 object 引用定位。
type announcementJournalEntry struct {
	SchemaVersion       int             `json:"schemaVersion"`
	State               string          `json:"state"`
	AnnouncementID      gen.Identifier  `json:"announcementId"`
	SiteID              gen.Identifier  `json:"siteId"`
	RevisionID          gen.Identifier  `json:"revisionId"`
	BodyHash            gen.Hash        `json:"bodyHash"`
	Object              gen.ObjectRef   `json:"object"`
	Revision            gen.Revision    `json:"revision"`
	VisibilityRevision  gen.Revision    `json:"visibilityRevision"`
	Enabled             bool            `json:"enabled"`
	StartsAt            *gen.Timestamp  `json:"startsAt"`
	EndsAt              *gen.Timestamp  `json:"endsAt"`
	UpdatedAt           gen.Timestamp   `json:"updatedAt"`
}

// RecoverAnnouncementJournal 在打开业务入口前恢复公告决定日志。COMMITTED
// 记录必须同时通过 head、对象文件、大小、hash 和正文 schema 校验；孤儿
// PREPARED 记录只进入隔离区，绝不推定为已发布。
func RecoverAnnouncementJournal(ctx context.Context, op platform.OperationContext, input gen.RecoverAnnouncementJournalInput) (gen.RecoverAnnouncementJournalResult, error) {
	_ = input
	if err := checkOperationDeadline(ctx, op); err != nil {
		return gen.RecoverAnnouncementJournalResult{}, err
	}
	store := activeStore()
	if store == nil {
		return gen.RecoverAnnouncementJournalResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	entries, journalHash, err := readAnnouncementJournal(store.activeRoot)
	if err != nil {
		return gen.RecoverAnnouncementJournalResult{}, err
	}
	committed := make(map[string]announcementJournalEntry)
	prepared := make([]announcementJournalEntry, 0)
	for _, entry := range entries {
		if err := validateAnnouncementJournalEntry(entry); err != nil {
			return gen.RecoverAnnouncementJournalResult{}, err
		}
		key := string(entry.AnnouncementID)
		switch entry.State {
		case announcementStatePrepared:
			prepared = append(prepared, entry)
		case announcementStateCommitted:
			previous, exists := committed[key]
			if exists {
				if entry.Revision < previous.Revision {
					return gen.RecoverAnnouncementJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告journal修订顺序不合法")
				}
				if entry.Revision == previous.Revision && !sameAnnouncementEntry(previous, entry) {
					return gen.RecoverAnnouncementJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "公告journal同修订内容不一致")
				}
			}
			committed[key] = entry
		}
	}

	for key, entry := range committed {
		if err := validateCommittedAnnouncement(store.activeRoot, entry); err != nil {
			return gen.RecoverAnnouncementJournalResult{}, err
		}
		if err := store.upsertAnnouncementHead(ctx, entry); err != nil {
			return gen.RecoverAnnouncementJournalResult{}, err
		}
		for i := range prepared {
			if string(prepared[i].AnnouncementID) == key && sameAnnouncementEntry(prepared[i], entry) {
				prepared[i].State = "already_committed"
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
			return gen.RecoverAnnouncementJournalResult{}, err
		}
	}
	store.markJournalRecovered(journalHash)
	return gen.RecoverAnnouncementJournalResult{
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
	if value == nil || value.IsZero() {
		return nil
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
	}
	return nil
}

func sameAnnouncementEntry(left, right announcementJournalEntry) bool {
	left.State = right.State
	right.State = left.State
	left.SchemaVersion = right.SchemaVersion
	right.SchemaVersion = left.SchemaVersion
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

func (s *ActiveStore) upsertAnnouncementHead(ctx context.Context, entry announcementJournalEntry) error {
	var currentRevision int64
	var currentRevisionID, currentBodyHash string
	err := s.db.QueryRowContext(ctx, `
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
	_, err = s.db.ExecContext(ctx, `
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
