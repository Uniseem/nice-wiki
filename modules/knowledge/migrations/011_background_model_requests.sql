-- 只背景原任务持久完整去凭据Create DTO，供owner只读原key查找；QA正文/permit不进入本表字段。
ALTER TABLE model_steps ADD COLUMN request_json BLOB;
ALTER TABLE model_steps ADD COLUMN request_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE model_steps ADD COLUMN observation_json BLOB;
ALTER TABLE model_steps ADD COLUMN observation_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE model_steps ADD COLUMN not_accepted_json BLOB;
ALTER TABLE model_steps ADD COLUMN not_accepted_hash TEXT NOT NULL DEFAULT '';
