-- 撤销结果与业务写入同事务保存，重放返回原计数，不重新撤销新会话。
-- 既有回执结果保持NULL；读取其完整结果时必须明确拒绝缺失，不能猜测。
ALTER TABLE operation_receipts ADD COLUMN result_json TEXT;

-- 自身会话列表按签发时间与会话ID进行稳定键集分页。
CREATE INDEX idx_sessions_self_page ON sessions (account_id, site_id, issued_at, session_id);
