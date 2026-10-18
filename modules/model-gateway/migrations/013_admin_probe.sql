-- 用户已明确批准独立probe日限默认0不限；不改QA与后台原账。
ALTER TABLE fee_budget_policy ADD COLUMN daily_probe_limit_micro_usd INTEGER NOT NULL DEFAULT 0 CHECK(daily_probe_limit_micro_usd BETWEEN 0 AND 9007199254740991);
-- 在接受invocation前同事务登记；延迟外键确保事务提交时必须有真实调用。
CREATE TABLE connection_probes(invocation_id TEXT PRIMARY KEY REFERENCES invocations(invocation_id) DEFERRABLE INITIALLY DEFERRED,provider_id TEXT NOT NULL,model_id TEXT NOT NULL,data_epoch TEXT NOT NULL,created_ns INTEGER NOT NULL);
CREATE INDEX connection_probe_latest ON connection_probes(provider_id,data_epoch,created_ns DESC,invocation_id DESC);
