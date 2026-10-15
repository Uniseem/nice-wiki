-- 已发布迁移不改写；终局状态、安全错误码及回执与任务在同一事务提交。
-- 历史根操作未知时留空，不能猜测operation_id就是付费根。
ALTER TABLE jobs ADD COLUMN root_operation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN last_error_code TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN cancel_requested INTEGER NOT NULL DEFAULT 0 CHECK (cancel_requested IN (0, 1));
UPDATE jobs SET cancel_requested=1 WHERE state='cancel-requested';

ALTER TABLE lease_operations RENAME TO lease_operations_previous;
CREATE TABLE lease_operations (
    owner TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('claim', 'heartbeat', 'complete', 'fail', 'cancel')),
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    PRIMARY KEY (owner, data_epoch, operation_kind, operation_id)
);
INSERT INTO lease_operations SELECT * FROM lease_operations_previous;
DROP TABLE lease_operations_previous;

-- 即使HTTP响应丢失、换用新操作键，也只能重放同一job/fence/worker的原终局。
CREATE TABLE job_outcomes (
    job_id TEXT NOT NULL REFERENCES jobs(job_id),
    fencing_token INTEGER NOT NULL,
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('complete', 'fail')),
    worker_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    PRIMARY KEY (job_id, fencing_token)
);
