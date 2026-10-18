-- 私有业务辅助记录；正文与完整元数据保存在权威 Markdown 对象，SQL 只保存提交指针和派生清单。
CREATE TABLE memory_receipts (
 site_id TEXT NOT NULL, actor_id TEXT NOT NULL, function_id TEXT NOT NULL,
 key_hash TEXT NOT NULL, data_epoch TEXT NOT NULL, input_hash TEXT NOT NULL,
 result_json BLOB NOT NULL, expires_at TEXT NOT NULL,
 PRIMARY KEY(site_id,actor_id,function_id,key_hash,data_epoch)
);
ALTER TABLE task_mappings ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0 CHECK(deleted IN (0,1));
ALTER TABLE context_snapshots ADD COLUMN manifest_json BLOB;
ALTER TABLE proposals ADD COLUMN payload_json BLOB;
ALTER TABLE proposals ADD COLUMN rejection_reason TEXT;
CREATE TABLE entry_commits (
 site_id TEXT NOT NULL, entry_id TEXT NOT NULL, version INTEGER NOT NULL,
 file_ref TEXT NOT NULL, body_hash TEXT NOT NULL, previous_version INTEGER,
 PRIMARY KEY(site_id,entry_id,version)
);
DROP INDEX idx_task_mappings_identity;
CREATE UNIQUE INDEX idx_task_mappings_identity ON task_mappings(site_id,task_id,entry_id,language) WHERE deleted=0;
CREATE TABLE assistant_inputs (
 run_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,operation_json BLOB NOT NULL,
 snapshot_json BLOB NOT NULL,input_hash TEXT NOT NULL,data_epoch TEXT NOT NULL,
 job_id TEXT,dispatch_state TEXT NOT NULL CHECK(dispatch_state IN ('pending','submitted','cancelled'))
);
ALTER TABLE assistant_inputs ADD COLUMN cancel_delivered INTEGER NOT NULL DEFAULT 0 CHECK(cancel_delivered IN(0,1));
ALTER TABLE proposal_commands ADD COLUMN command_json BLOB;
ALTER TABLE proposal_commands ADD COLUMN lease_id TEXT NOT NULL DEFAULT '';
ALTER TABLE proposal_commands ADD COLUMN lease_until TEXT NOT NULL DEFAULT '';
