-- 只追加定时任务的类型、输入引用、代次、操作回执和到期索引。
-- 不重建schedules，不删除已有行。旧行新增列为空，当前代次筛选自然排除。
ALTER TABLE schedules ADD COLUMN job_type TEXT NOT NULL DEFAULT '';
ALTER TABLE schedules ADD COLUMN input_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE schedules ADD COLUMN data_epoch TEXT NOT NULL DEFAULT '';

CREATE TABLE schedule_operations (
    caller TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (caller, data_epoch, operation_id)
);

CREATE INDEX idx_schedules_epoch_enabled ON schedules (data_epoch, enabled, schedule_id);
