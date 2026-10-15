-- media 域初始权威表（ADR0005：每服务私有 SQLite）。
-- 不可变原文件仍以 active/media/blobs/<siteId>/<sha 前两位>/<sha> 内容寻址存储，
-- 本库保存上传会话、资源元数据、操作回执与 blob 持久日志；nw_outbox/nw_inbox
-- 由 libs/platform 的 EnsureSchema 在同一业务库建立，不在本迁移重复建表。
CREATE TABLE upload_sessions (
    upload_id      TEXT PRIMARY KEY,
    site_id        TEXT NOT NULL,
    created_by     TEXT NOT NULL,
    state          TEXT NOT NULL,
    expected_bytes INTEGER NOT NULL CHECK (expected_bytes > 0),
    session_json   TEXT NOT NULL CHECK (json_valid(session_json)),
    updated_at     TEXT NOT NULL
);

CREATE INDEX idx_upload_sessions_site_state
    ON upload_sessions (site_id, state);

CREATE TABLE upload_receipts (
    operation_id TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    upload_id    TEXT NOT NULL REFERENCES upload_sessions (upload_id),
    created_at   TEXT NOT NULL
);

CREATE TABLE assets (
    asset_id   TEXT PRIMARY KEY,
    site_id    TEXT NOT NULL,
    sha256     TEXT NOT NULL,
    byte_size  INTEGER NOT NULL CHECK (byte_size > 0),
    media_type TEXT NOT NULL,
    state      TEXT NOT NULL,
    asset_json TEXT NOT NULL CHECK (json_valid(asset_json)),
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_assets_site_sha
    ON assets (site_id, sha256, state);

-- blob 持久日志：PREPARED 在写不可变文件前单独提交；COMMITTED 与资源元数据、
-- 上传会话完成在同一事务提交；DELETING 只记录人工确认的清理决定。
CREATE TABLE blob_journal (
    seq         INTEGER PRIMARY KEY AUTOINCREMENT,
    event       TEXT NOT NULL CHECK (event IN ('PREPARED', 'COMMITTED', 'DELETING')),
    site_id     TEXT NOT NULL,
    upload_id   TEXT NOT NULL,
    sha256      TEXT NOT NULL,
    asset_id    TEXT,
    occurred_at TEXT NOT NULL
);

CREATE INDEX idx_blob_journal_key
    ON blob_journal (site_id, upload_id, sha256, seq);

CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
