-- 免费恢复原已付费完整输出，独立保留原失败任务、执行事实和MG费用引用。
CREATE TABLE extraction_recovered_results (
 extraction_id TEXT PRIMARY KEY REFERENCES extractions(extraction_id),
 artifact_json BLOB NOT NULL,artifact_hash TEXT NOT NULL,
 model_step_key TEXT NOT NULL,model_result_hash TEXT NOT NULL,created_at TEXT NOT NULL
);
