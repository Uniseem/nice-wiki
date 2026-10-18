-- 提取、原任务拥有者和实际租约镜像仅本域私有，不复制jobs队列或MG费用表。
ALTER TABLE builds ADD COLUMN workflow_version INTEGER NOT NULL DEFAULT 0;
CREATE TABLE extractions (
 extraction_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
 cache_key TEXT NOT NULL,resource_key TEXT NOT NULL,owner_build_id TEXT NOT NULL,source_json BLOB NOT NULL,plan_json BLOB NOT NULL,
 workflow_version INTEGER NOT NULL DEFAULT 1,state TEXT NOT NULL,result_json BLOB,result_hash TEXT NOT NULL DEFAULT '',
 lease_id TEXT NOT NULL DEFAULT '',lease_until TEXT NOT NULL DEFAULT '',
 cancelled INTEGER NOT NULL DEFAULT 0,failure_code TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL
);
CREATE TABLE extraction_cache_intents (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,cache_key TEXT NOT NULL,extraction_id TEXT NOT NULL,
 PRIMARY KEY(site_id,data_epoch,cache_key)
);
CREATE TABLE extraction_origins (
 extraction_id TEXT NOT NULL,build_id TEXT NOT NULL,source_hash TEXT NOT NULL,source_json BLOB NOT NULL,
 PRIMARY KEY(extraction_id,build_id,source_hash)
);
CREATE TABLE extraction_dispatches (
 extraction_id TEXT PRIMARY KEY REFERENCES extractions(extraction_id),
 input_json BLOB NOT NULL,input_ref_json BLOB NOT NULL,operation_json BLOB NOT NULL,
 dedupe_key TEXT NOT NULL,job_id TEXT,jobs_operation_id TEXT NOT NULL DEFAULT '',
 dispatch_state TEXT NOT NULL DEFAULT 'pending',cancel_requested INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,updated_at TEXT NOT NULL
);
CREATE TABLE execution_records (
 job_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,
 job_type TEXT NOT NULL,object_id TEXT NOT NULL,input_ref_json BLOB NOT NULL,input_json BLOB NOT NULL,
 operation_json BLOB NOT NULL,worker_id TEXT NOT NULL,fence INTEGER NOT NULL,attempt INTEGER NOT NULL,
 lease_until TEXT NOT NULL,cancel_requested INTEGER NOT NULL DEFAULT 0,state TEXT NOT NULL,
 result_json BLOB,result_ref_json BLOB,paid_refs_json BLOB,
 UNIQUE(site_id,data_epoch,job_type,object_id)
);
CREATE TABLE build_execution_plans (
 build_id TEXT NOT NULL,phase TEXT NOT NULL,plan_json BLOB NOT NULL,plan_hash TEXT NOT NULL,
 PRIMARY KEY(build_id,phase)
);
