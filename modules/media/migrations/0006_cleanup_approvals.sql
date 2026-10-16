-- 原批准、固定任务输入与回执独立于可取消计划；未知任务提交不重新批准。
CREATE TABLE manual_cleanup_approvals (
    plan_id TEXT PRIMARY KEY REFERENCES manual_cleanup_plans(plan_id),
    operation_id TEXT NOT NULL UNIQUE,
    request_hash TEXT NOT NULL,
    approval_json TEXT NOT NULL CHECK(json_valid(approval_json)),
    approval_hash TEXT NOT NULL,
    job_input_json TEXT NOT NULL CHECK(json_valid(job_input_json)),
    job_input_hash TEXT NOT NULL,
    job_receipt_json TEXT CHECK(job_receipt_json IS NULL OR json_valid(job_receipt_json)),
    job_receipt_hash TEXT,
    result_json TEXT CHECK(result_json IS NULL OR json_valid(result_json)),
    result_hash TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK((job_receipt_json IS NULL)=(job_receipt_hash IS NULL)),
    CHECK((result_json IS NULL)=(result_hash IS NULL)),
    CHECK(result_json IS NULL OR job_receipt_json IS NOT NULL)
);
