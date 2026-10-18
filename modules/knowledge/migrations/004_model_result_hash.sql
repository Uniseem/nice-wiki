-- 本域模型步骤保留真实付费引用和完整原结果摘要；不复制MG费用表。
ALTER TABLE model_steps ADD COLUMN result_hash TEXT NOT NULL DEFAULT '';
