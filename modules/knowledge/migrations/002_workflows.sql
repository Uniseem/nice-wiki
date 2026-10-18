-- 配置、调度意图与原操作记录独立持久；不保存余额或授权凭证。
CREATE TABLE index_config (
 site_id TEXT PRIMARY KEY,data_epoch TEXT NOT NULL,config_id TEXT NOT NULL,
 version INTEGER NOT NULL,config_json BLOB NOT NULL
);
CREATE TABLE operation_receipts (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,actor_id TEXT NOT NULL,
 function_id TEXT NOT NULL,idempotency_key TEXT NOT NULL,input_hash TEXT NOT NULL,
 result_json BLOB NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(site_id,data_epoch,actor_id,function_id,idempotency_key)
);
CREATE TABLE build_dispatches (
 build_id TEXT PRIMARY KEY REFERENCES builds(build_id),input_json BLOB NOT NULL,
 input_ref_json BLOB NOT NULL,operation_json BLOB NOT NULL,dedupe_key TEXT NOT NULL,
 job_id TEXT,dispatch_state TEXT NOT NULL DEFAULT 'pending',cancel_requested INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,updated_at TEXT NOT NULL
);
ALTER TABLE builds ADD COLUMN config_json BLOB;
ALTER TABLE builds ADD COLUMN created_at TEXT NOT NULL DEFAULT '';
ALTER TABLE builds ADD COLUMN retired_at_ns INTEGER NOT NULL DEFAULT 0;
CREATE INDEX builds_source ON builds(site_id,data_epoch,document_id,space_id,generation);
CREATE TABLE document_watermarks (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,document_id TEXT NOT NULL,
 visibility_revision INTEGER NOT NULL,withdrawn INTEGER NOT NULL,
 PRIMARY KEY(site_id,data_epoch,document_id)
);
CREATE TABLE model_steps (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,owner_id TEXT NOT NULL,step_key TEXT NOT NULL,
 input_hash TEXT NOT NULL,operation_id TEXT NOT NULL,invocation_id TEXT,
 state TEXT NOT NULL,result_json BLOB,created_at TEXT NOT NULL,
 PRIMARY KEY(site_id,data_epoch,owner_id,step_key)
);

CREATE TABLE generation_watermarks (
 site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,document_id TEXT NOT NULL,space_id TEXT NOT NULL,
 generation INTEGER NOT NULL,PRIMARY KEY(site_id,data_epoch,document_id,space_id)
);
