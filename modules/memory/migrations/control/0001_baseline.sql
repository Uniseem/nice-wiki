-- 基线迁移：由历史迁移重放得到的最终结构（0001_control.sql 至 0001_control.sql）。
CREATE TABLE control_state (
    id            INTEGER PRIMARY KEY CHECK (id = 1),
    barrier_state TEXT NOT NULL CHECK (barrier_state IN ('released', 'draining', 'safe', 'failed')),
    data_epoch    TEXT NOT NULL,
    safe_point    INTEGER NOT NULL CHECK (safe_point IN (0, 1)),
    updated_at    TEXT NOT NULL
);
CREATE TABLE restore_receipts (
    restore_id  TEXT PRIMARY KEY,
    snapshot_id TEXT NOT NULL,
    checksum    TEXT NOT NULL,
    state       TEXT NOT NULL,
    created_at  TEXT NOT NULL
);
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
INSERT INTO "control_state"("id","barrier_state","data_epoch","safe_point","updated_at") VALUES(1,'released','',1,'');
