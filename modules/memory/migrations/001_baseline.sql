-- 基线迁移：由历史迁移重放得到的最终结构（001_initial.sql 至 002_business.sql）。
CREATE TABLE memory_entries (
    site_id             TEXT NOT NULL,
    entry_id            TEXT NOT NULL,
    version             INTEGER NOT NULL CHECK (version >= 0),
    title               TEXT NOT NULL,
    file_ref            TEXT NOT NULL,
    language            TEXT NOT NULL,
    tags_json           TEXT NOT NULL,
    priority            INTEGER NOT NULL CHECK (priority BETWEEN 0 AND 1000),
    state               TEXT NOT NULL CHECK (state IN ('active', 'archived')),
    updated_at          TEXT NOT NULL,
    body_hash           TEXT NOT NULL,
    description         TEXT NOT NULL,
    maximum_bytes       INTEGER NOT NULL CHECK (maximum_bytes BETWEEN 0 AND 9007199254740991),
    readonly            INTEGER NOT NULL CHECK (readonly IN (0, 1)),
    source              TEXT NOT NULL CHECK (source IN ('manual', 'assistant', 'auto')),
    auto_update_enabled INTEGER NOT NULL CHECK (auto_update_enabled IN (0, 1)),
    PRIMARY KEY (site_id, entry_id, version)
);
CREATE TABLE entry_current (
    site_id  TEXT NOT NULL,
    entry_id TEXT NOT NULL,
    version  INTEGER NOT NULL CHECK (version >= 0),
    PRIMARY KEY (site_id, entry_id),
    FOREIGN KEY (site_id, entry_id, version)
        REFERENCES memory_entries (site_id, entry_id, version)
);
CREATE TABLE entry_revisions (
    site_id    TEXT NOT NULL,
    entry_id   TEXT NOT NULL,
    version    INTEGER NOT NULL CHECK (version >= 0),
    body_hash  TEXT NOT NULL,
    file_ref   TEXT NOT NULL,
    actor_id   TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (site_id, entry_id, version)
);
CREATE TABLE task_mappings (
    site_id     TEXT NOT NULL,
    mapping_id  TEXT NOT NULL,
    task_id     TEXT NOT NULL,
    entry_id    TEXT NOT NULL,
    order_value INTEGER NOT NULL CHECK (order_value BETWEEN 0 AND 10000),
    enabled     INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    language    TEXT NOT NULL,
    revision    INTEGER NOT NULL CHECK (revision >= 0), deleted INTEGER NOT NULL DEFAULT 0 CHECK(deleted IN (0,1)),
    PRIMARY KEY (site_id, mapping_id)
);
CREATE TABLE task_budgets (
    site_id        TEXT NOT NULL,
    task_id        TEXT NOT NULL,
    maximum_bytes  INTEGER NOT NULL CHECK (maximum_bytes BETWEEN 1024 AND 65536),
    maximum_entries INTEGER NOT NULL CHECK (maximum_entries BETWEEN 0 AND 64),
    revision       INTEGER NOT NULL CHECK (revision >= 0),
    PRIMARY KEY (site_id, task_id)
);
CREATE TABLE context_snapshots (
    site_id         TEXT NOT NULL,
    snapshot_id     TEXT NOT NULL,
    task_id         TEXT NOT NULL,
    language        TEXT NOT NULL,
    mapping_revision INTEGER NOT NULL CHECK (mapping_revision >= 0),
    manifest_hash   TEXT NOT NULL,
    body_hash       TEXT NOT NULL,
    byte_size       INTEGER NOT NULL CHECK (byte_size >= 0),
    created_at      TEXT NOT NULL, manifest_json BLOB,
    PRIMARY KEY (site_id, snapshot_id)
);
CREATE TABLE assistant_runs (
    site_id          TEXT NOT NULL,
    run_id           TEXT NOT NULL,
    source_generation INTEGER NOT NULL CHECK (source_generation >= 0),
    state            TEXT NOT NULL CHECK (state IN ('queued', 'running', 'review', 'completed', 'cancelled', 'failed', 'superseded')),
    invocation_id    TEXT,
    actor_id         TEXT NOT NULL,
    proposal_ids_json TEXT NOT NULL,
    PRIMARY KEY (site_id, run_id)
);
CREATE TABLE proposals (
    site_id                TEXT NOT NULL,
    proposal_id             TEXT NOT NULL,
    run_id                  TEXT NOT NULL,
    kind                    TEXT NOT NULL CHECK (kind IN ('entry_create', 'entry_update', 'entry_archive', 'glossary')),
    state                   TEXT NOT NULL CHECK (state IN ('pending', 'approved', 'applying', 'applied', 'rejected', 'conflicted')),
    payload_hash            TEXT NOT NULL,
    expected_entry_version  INTEGER,
    approved_by             TEXT,
    approval_revision       INTEGER NOT NULL CHECK (approval_revision >= 0), payload_json BLOB, rejection_reason TEXT,
    PRIMARY KEY (site_id, proposal_id)
);
CREATE TABLE proposal_commands (
    site_id              TEXT NOT NULL,
    command_id            TEXT NOT NULL,
    proposal_id           TEXT NOT NULL,
    payload_hash          TEXT NOT NULL,
    state                 TEXT NOT NULL CHECK (state IN ('pending', 'leased', 'applying', 'applied', 'rejected', 'failed')),
    localization_receipt TEXT, command_json BLOB, lease_id TEXT NOT NULL DEFAULT '', lease_until TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (site_id, command_id),
    UNIQUE (site_id, proposal_id)
);
CREATE TABLE assistant_conversations (
    site_id         TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    actor_id        TEXT NOT NULL,
    version         INTEGER NOT NULL CHECK (version >= 0),
    expires_at      TEXT NOT NULL,
    PRIMARY KEY (site_id, conversation_id)
);
CREATE TABLE assistant_messages (
    site_id         TEXT NOT NULL,
    message_id      TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    ordinal         INTEGER NOT NULL CHECK (ordinal >= 0),
    role            TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    body            TEXT NOT NULL,
    run_id          TEXT,
    created_at      TEXT NOT NULL,
    PRIMARY KEY (site_id, message_id),
    UNIQUE (site_id, conversation_id, ordinal)
);
CREATE TABLE runtime_state (
    id               INTEGER PRIMARY KEY CHECK (id = 1),
    schema_version   INTEGER NOT NULL CHECK (schema_version = 1),
    memory_generation INTEGER NOT NULL CHECK (memory_generation >= 0),
    data_epoch       TEXT NOT NULL,
    mapping_revision INTEGER NOT NULL CHECK (mapping_revision >= 0),
    updated_at       TEXT NOT NULL
);
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
CREATE TABLE memory_receipts (
 site_id TEXT NOT NULL, actor_id TEXT NOT NULL, function_id TEXT NOT NULL,
 key_hash TEXT NOT NULL, data_epoch TEXT NOT NULL, input_hash TEXT NOT NULL,
 result_json BLOB NOT NULL, expires_at TEXT NOT NULL,
 PRIMARY KEY(site_id,actor_id,function_id,key_hash,data_epoch)
);
CREATE TABLE entry_commits (
 site_id TEXT NOT NULL, entry_id TEXT NOT NULL, version INTEGER NOT NULL,
 file_ref TEXT NOT NULL, body_hash TEXT NOT NULL, previous_version INTEGER,
 PRIMARY KEY(site_id,entry_id,version)
);
CREATE TABLE assistant_inputs (
 run_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,operation_json BLOB NOT NULL,
 snapshot_json BLOB NOT NULL,input_hash TEXT NOT NULL,data_epoch TEXT NOT NULL,
 job_id TEXT,dispatch_state TEXT NOT NULL CHECK(dispatch_state IN ('pending','submitted','cancelled'))
, cancel_delivered INTEGER NOT NULL DEFAULT 0 CHECK(cancel_delivered IN(0,1)));
CREATE INDEX idx_memory_entries_current
    ON memory_entries (site_id, entry_id, state, version DESC);
CREATE INDEX idx_memory_entries_epoch
    ON memory_entries (site_id, updated_at, entry_id, version);
CREATE INDEX idx_task_mappings_order
    ON task_mappings (site_id, task_id, language, enabled, order_value, mapping_id);
CREATE UNIQUE INDEX idx_task_mappings_identity ON task_mappings(site_id,task_id,entry_id,language) WHERE deleted=0;
