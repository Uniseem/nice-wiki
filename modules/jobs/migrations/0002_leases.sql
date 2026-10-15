-- fence计数独立于可释放的租约行，防止重试或回收后重复使用旧执行权。
ALTER TABLE jobs ADD COLUMN fencing_counter INTEGER NOT NULL DEFAULT 0 CHECK (fencing_counter >= 0);
UPDATE jobs SET fencing_counter = COALESCE((SELECT fencing_token FROM leases WHERE leases.job_id = jobs.job_id), 0);

-- 回执与领取/续租在同一事务提交，恢复代次、owner和操作种类共同限定幂等范围。
CREATE TABLE lease_operations (
    owner TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('claim', 'heartbeat')),
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_json BLOB NOT NULL,
    PRIMARY KEY (owner, data_epoch, operation_kind, operation_id)
);

-- 进度回执与任务同库，避免心跳重试重复分配seq；旧记录保留空键且不参与新唯一索引。
ALTER TABLE progress ADD COLUMN checkpoint_key TEXT NOT NULL DEFAULT '';
ALTER TABLE progress ADD COLUMN input_hash TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX idx_progress_checkpoint ON progress(job_id, checkpoint_key) WHERE checkpoint_key <> '';
