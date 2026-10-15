-- 初始化决定是独立持久记录；准备配置仅是恢复材料，不能替代settings权威文件。
CREATE TABLE initialization_decisions (
    operation_id         TEXT PRIMARY KEY,
    site_id              TEXT NOT NULL,
    decision             TEXT NOT NULL CHECK (decision IN ('commit', 'abort')),
    request_hash         TEXT NOT NULL,
    decision_state_json  TEXT NOT NULL,
    prepared_config_json TEXT,
    prepared_config_hash TEXT,
    created_at           TEXT NOT NULL,
    CHECK ((decision = 'commit' AND prepared_config_json IS NOT NULL AND prepared_config_hash IS NOT NULL)
        OR (decision = 'abort' AND prepared_config_json IS NULL AND prepared_config_hash IS NULL))
);

CREATE TABLE initialization_state (
    singleton    INTEGER PRIMARY KEY CHECK (singleton = 1),
    operation_id TEXT NOT NULL UNIQUE REFERENCES initialization_decisions(operation_id),
    revision     INTEGER NOT NULL CHECK (revision >= 1),
    state_json   TEXT NOT NULL
);
