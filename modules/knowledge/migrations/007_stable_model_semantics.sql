-- 本域语义阶段固定原MG key；配置/记忆漂移不能换hash自动再付费。
ALTER TABLE model_steps ADD COLUMN semantic_key TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX model_step_semantic ON model_steps(site_id,data_epoch,owner_id,semantic_key) WHERE semantic_key<>'';
CREATE TABLE build_context_plans (
 build_id TEXT PRIMARY KEY,plan_json BLOB NOT NULL,plan_hash TEXT NOT NULL,created_at TEXT NOT NULL
);
