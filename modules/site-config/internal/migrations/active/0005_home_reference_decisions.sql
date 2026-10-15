-- 资源pin不是临时调用：固定来源材料和每个外部收敛阶段均需恢复。
-- config_operations保持普通业务回执；终止决定由本表保存，COMMITTED文件始终优先。
CREATE TABLE home_reference_decisions (
    operation_id TEXT PRIMARY KEY REFERENCES config_operations(operation_id),
    root_operation_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    source_object_id TEXT NOT NULL,
    source_revision_id TEXT NOT NULL,
    source_version INTEGER NOT NULL CHECK (source_version >= 1),
    reference_digest TEXT NOT NULL,
    references_json TEXT NOT NULL,
    decision TEXT NOT NULL CHECK (decision IN ('pending','committed','aborted','rejected')),
    phase TEXT NOT NULL CHECK (phase IN ('acquire_pending','pin_acquired','confirm_pending','confirmed','release_pending','released')),
    pin_json TEXT,
    pin_hash TEXT,
    failure_code TEXT,
    created_at TEXT NOT NULL,
    CHECK ((pin_json IS NULL) = (pin_hash IS NULL)),
    CHECK ((decision='pending' AND phase IN ('acquire_pending','pin_acquired')) OR
           (decision='committed' AND phase IN ('confirm_pending','confirmed')) OR
           (decision IN ('aborted','rejected') AND phase IN ('release_pending','released')))
);
CREATE INDEX idx_home_reference_pending ON home_reference_decisions(data_epoch,phase,source_version);
