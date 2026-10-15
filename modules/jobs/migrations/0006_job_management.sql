-- 人工retry独立回执；不扩写既有lease/event种类，不修改原任务/关联表。
-- request_hash绑定可信actor/site和jobId，不保存任务输入正文。
CREATE TABLE job_management_operations (
    caller TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (caller, data_epoch, operation_id)
);
