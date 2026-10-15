-- 认证代次由首次签发事务初始化，恢复换入后必须更新；不为旧会话补发凭据。
CREATE TABLE auth_state (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    auth_epoch TEXT NOT NULL
);

-- 一个设备会话对应一个refresh族，轮换保持session_id不变。
-- 仅保存敏感令牌的摘要；绝对期限继续由sessions权威列保存。
CREATE TABLE session_families (
    family_id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL UNIQUE REFERENCES sessions (session_id),
    auth_epoch TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    refresh_digest TEXT NOT NULL UNIQUE,
    access_digest TEXT NOT NULL,
    csrf_digest TEXT NOT NULL,
    revoked_at TEXT,
    revision INTEGER NOT NULL CHECK (revision >= 1)
);

-- 已消费摘要保留到该设备会话失效，命中即撤销全族，不能再返回新secret。
CREATE TABLE consumed_refresh_tokens (
    refresh_digest TEXT PRIMARY KEY,
    family_id TEXT NOT NULL REFERENCES session_families (family_id),
    consumed_at TEXT NOT NULL
);
CREATE INDEX idx_consumed_refresh_family ON consumed_refresh_tokens (family_id);
