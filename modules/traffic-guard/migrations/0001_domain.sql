-- traffic-guard 域初始权威表。
-- nw_outbox 由 libs/platform/outbox 在启动时按公共机制创建，不复制到本迁移。
CREATE TABLE guard_policy (
    site_id     TEXT PRIMARY KEY,
    revision    INTEGER NOT NULL CHECK (revision >= 0),
    policy_json TEXT NOT NULL CHECK (json_valid(policy_json)),
    data_epoch  TEXT NOT NULL
);

-- UpdatePolicy 的 operationId 幂等回执；只保存输入摘要和结果修订，不保存正文。
CREATE TABLE policy_operation_receipts (
    site_id       TEXT NOT NULL,
    operation_id  TEXT NOT NULL,
    input_hash    TEXT NOT NULL,
    revision      INTEGER NOT NULL CHECK (revision >= 0),
    data_epoch    TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    PRIMARY KEY (site_id, operation_id)
);

CREATE INDEX idx_policy_receipts_epoch
    ON policy_operation_receipts (data_epoch, created_at);

-- 迁移机制自身的已应用记录；由启动装配层按版本登记。
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
