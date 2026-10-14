-- site-config active 区业务索引的初始迁移。
-- 正文仍以 active/settings 与 active/announcements 下的不可变文件为权威；
-- 本库只保存提交索引、历史水位和恢复后可重建的状态。
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);

CREATE TABLE site_config_state (
    singleton            INTEGER PRIMARY KEY CHECK (singleton = 1),
    data_epoch           TEXT NOT NULL,
    initialized          INTEGER NOT NULL CHECK (initialized IN (0, 1)),
    site_id              TEXT,
    config_revision      INTEGER NOT NULL CHECK (config_revision >= 0),
    config_revision_id   TEXT,
    updated_at           TEXT NOT NULL
);

CREATE TABLE config_revision_index (
    site_id       TEXT NOT NULL,
    revision_id   TEXT NOT NULL,
    object_id     TEXT NOT NULL,
    sha256        TEXT NOT NULL,
    byte_size     INTEGER NOT NULL CHECK (byte_size > 0),
    revision      INTEGER NOT NULL CHECK (revision >= 1),
    committed_at  TEXT NOT NULL,
    PRIMARY KEY (site_id, revision_id)
);

CREATE TABLE announcement_heads (
    announcement_id     TEXT PRIMARY KEY,
    site_id             TEXT NOT NULL,
    revision_id         TEXT NOT NULL,
    object_id           TEXT NOT NULL,
    body_hash           TEXT NOT NULL,
    revision            INTEGER NOT NULL CHECK (revision >= 1),
    visibility_revision INTEGER NOT NULL CHECK (visibility_revision >= 1),
    enabled             INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    starts_at           TEXT,
    ends_at             TEXT,
    updated_at          TEXT NOT NULL
);

CREATE INDEX idx_config_revision_index_site_revision
    ON config_revision_index (site_id, revision DESC);
