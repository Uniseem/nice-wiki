-- 全量估价只保存真实源/模型/记忆/价格快照，批准后原集合任务与回执同事务。
CREATE TABLE rebuild_plans (
 plan_id TEXT PRIMARY KEY,site_id TEXT NOT NULL,data_epoch TEXT NOT NULL,actor_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,input_hash TEXT NOT NULL,snapshot_json BLOB NOT NULL,snapshot_hash TEXT NOT NULL,
 result_json BLOB NOT NULL,expires_at TEXT NOT NULL,
 approved_by TEXT,approved_at TEXT,approved_root TEXT,approved_result_json BLOB,build_ids_json BLOB,
 UNIQUE(site_id,data_epoch,actor_id,operation_id)
);
