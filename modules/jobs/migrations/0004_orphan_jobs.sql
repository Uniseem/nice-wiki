-- 延后关联检查直到同名父表重建完成；子表仍引用jobs，不能先重命名原父表。
PRAGMA defer_foreign_keys=ON;
CREATE TABLE jobs_rebuilt (
    job_id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL,
    root_operation_id TEXT NOT NULL,
    owner TEXT NOT NULL,
    type TEXT NOT NULL,
    immutable_input_ref BLOB NOT NULL,
    input_hash TEXT NOT NULL,
    dedupe_key TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued', 'leased', 'running', 'retry-wait', 'dead-letter', 'cancel-requested', 'cancelled', 'succeeded', 'orphan')),
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    not_before TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    fencing_counter INTEGER NOT NULL DEFAULT 0 CHECK (fencing_counter >= 0),
    last_error_code TEXT NOT NULL DEFAULT '',
    cancel_requested INTEGER NOT NULL DEFAULT 0 CHECK (cancel_requested IN (0, 1))
);
INSERT INTO jobs_rebuilt (job_id,operation_id,root_operation_id,owner,type,immutable_input_ref,input_hash,dedupe_key,state,attempt,not_before,data_epoch,revision,created_at,updated_at,fencing_counter,last_error_code,cancel_requested)
SELECT job_id,operation_id,root_operation_id,owner,type,immutable_input_ref,input_hash,dedupe_key,state,attempt,not_before,data_epoch,revision,created_at,updated_at,fencing_counter,last_error_code,cancel_requested FROM jobs;
DROP TABLE jobs;
ALTER TABLE jobs_rebuilt RENAME TO jobs;
CREATE UNIQUE INDEX idx_jobs_owner_epoch_dedupe ON jobs(owner,data_epoch,dedupe_key);
CREATE INDEX idx_jobs_claimable ON jobs(state,not_before,created_at,job_id);

CREATE TABLE job_orphans (
    job_id TEXT PRIMARY KEY REFERENCES jobs(job_id),
    expired_fence INTEGER NOT NULL,
    worker_id TEXT NOT NULL,
    next_check_at TEXT NOT NULL
);
