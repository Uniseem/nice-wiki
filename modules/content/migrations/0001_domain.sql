-- content 私有正文权威库。正文文件仍为内容真相；本迁移不创建文档或已提交资格。
-- nw_outbox 只能由 libs/platform/outbox.EnsureSchema 建立，禁止复制公共表。
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    sha256 TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
CREATE TABLE store_identity (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    data_epoch TEXT NOT NULL,
    site_id TEXT
);
CREATE TABLE revision_preparations (
    commit_id TEXT PRIMARY KEY,
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    document_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('preparing', 'prepared', 'committed', 'aborted', 'quarantined')),
    result_json TEXT NOT NULL CHECK (json_valid(result_json)),
    UNIQUE (caller_service, actor_id, site_id, data_epoch, operation_id),
    UNIQUE (site_id, revision_id)
);
-- 仅草稿业务单事务可同时写入以下三表；文件准备和启动恢复不能制造提交。
CREATE TABLE committed_revisions (
    commit_id TEXT PRIMARY KEY REFERENCES revision_preparations(commit_id),
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('committed', 'quarantined')),
    UNIQUE (site_id, revision_id)
);
CREATE TABLE draft_commit_receipts (
    commit_id TEXT PRIMARY KEY REFERENCES committed_revisions(commit_id),
    caller_service TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL CHECK (json_valid(result_json)),
    UNIQUE (caller_service, actor_id, site_id, data_epoch, operation_id)
);
CREATE TABLE draft_heads (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    draft_version INTEGER NOT NULL CHECK (draft_version > 0),
    PRIMARY KEY (site_id, document_id)
);
CREATE TABLE document_quarantines (
    site_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    quarantined_at TEXT NOT NULL,
    PRIMARY KEY (site_id, document_id)
);
