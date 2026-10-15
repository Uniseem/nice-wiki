-- 配置回执不是可重建索引；文件COMMITTED决定包含同一原始请求身份。
CREATE TABLE config_operations (
    operation_id  TEXT PRIMARY KEY,
    actor_id      TEXT NOT NULL,
    site_id       TEXT NOT NULL,
    data_epoch    TEXT NOT NULL,
    request_hash  TEXT NOT NULL,
    state         TEXT NOT NULL CHECK (state IN ('in_progress', 'committed')),
    revision_id   TEXT,
    result_revision INTEGER,
    result_state_json TEXT
);
