-- 普通设置的候选配置仅供未完成操作恢复，不能替代settings权威文件。
CREATE TABLE config_operation_materials (
    operation_id TEXT PRIMARY KEY REFERENCES config_operations(operation_id),
    kind         TEXT NOT NULL,
    config_json  TEXT NOT NULL,
    config_hash  TEXT NOT NULL,
    config_revision INTEGER NOT NULL CHECK (config_revision >= 1)
);

CREATE UNIQUE INDEX idx_config_revision_object ON config_revision_index(object_id);
CREATE UNIQUE INDEX idx_config_revision_number ON config_revision_index(site_id,revision);
