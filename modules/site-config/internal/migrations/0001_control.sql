-- site-config 运行状态控制库的初始权威表。
-- control.db 不在 active/ 下，恢复换目录不得覆盖本库。
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);

CREATE TABLE runtime_state (
    singleton          INTEGER PRIMARY KEY CHECK (singleton = 1),
    data_epoch         TEXT NOT NULL,
    target_data_epoch  TEXT,
    phase              TEXT NOT NULL CHECK (phase IN (
        'running', 'backup_barrier', 'restore_staging',
        'restore_switching', 'restore_rolling_back', 'restore_blocked'
    )),
    maintenance        INTEGER NOT NULL CHECK (maintenance IN (0, 1)),
    restore_id         TEXT,
    barrier_id         TEXT,
    revision           INTEGER NOT NULL CHECK (revision >= 1),
    required_modules   TEXT NOT NULL,
    updated_at         TEXT NOT NULL
);

CREATE TABLE activation_receipts (
    restore_id        TEXT NOT NULL,
    module            TEXT NOT NULL,
    stage_id          TEXT NOT NULL,
    target_data_epoch TEXT NOT NULL,
    state             TEXT NOT NULL CHECK (state IN ('switched', 'rolled-back')),
    manifest_hash     TEXT NOT NULL,
    recorded_at       TEXT NOT NULL,
    signature         TEXT NOT NULL,
    PRIMARY KEY (restore_id, module)
);

CREATE TABLE runtime_operation_receipts (
    operation_id TEXT PRIMARY KEY,
    kind         TEXT NOT NULL,
    input_hash   TEXT NOT NULL,
    result_json  TEXT NOT NULL,
    created_at   TEXT NOT NULL
);

CREATE INDEX idx_activation_receipts_restore
    ON activation_receipts (restore_id);
