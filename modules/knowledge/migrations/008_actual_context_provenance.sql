-- 背景缓存只复用MG真正winning模型快照证明；旧缺证明结果保留但不猜版本复用。
ALTER TABLE chunk_contexts ADD COLUMN model_snapshot_hash TEXT NOT NULL DEFAULT '';
