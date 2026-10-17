-- 只持久化既有人工批准的执行决定；不从任务或年龄产生删除许可。
CREATE TABLE manual_cleanup_executions (
    plan_id TEXT PRIMARY KEY REFERENCES manual_cleanup_approvals(plan_id),
    approval_hash TEXT NOT NULL,
    job_id TEXT NOT NULL UNIQUE,
    execution_json TEXT NOT NULL CHECK(json_valid(execution_json)),
    execution_hash TEXT NOT NULL,
    result_json TEXT CHECK(result_json IS NULL OR json_valid(result_json)),
    result_hash TEXT,
    created_at TEXT NOT NULL,
    CHECK((result_json IS NULL)=(result_hash IS NULL))
);
-- 决定内固定原资产、原批准精确marks及共享bytes归属；完成结果另存，不改写决定。
CREATE TABLE manual_cleanup_deletions (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id),
    plan_id TEXT NOT NULL REFERENCES manual_cleanup_executions(plan_id),
    decision_json TEXT NOT NULL CHECK(json_valid(decision_json)),
    decision_hash TEXT NOT NULL,
    UNIQUE(plan_id,asset_id)
);
CREATE INDEX idx_manual_cleanup_deletions_plan ON manual_cleanup_deletions(plan_id,asset_id);
-- 下载入口须在同一asset锁下取得租约，流严格止于expires_at_ns；旧域尚无bytes入口。
CREATE TABLE asset_reader_leases (
    reader_id TEXT PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id),
    data_epoch TEXT NOT NULL,
    expires_at_ns INTEGER NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_asset_reader_leases_asset ON asset_reader_leases(asset_id,expires_at_ns);
