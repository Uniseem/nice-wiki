-- 费用策略必须由当前超管明确初始化；不合成会话美元默认或探测结果。
CREATE TABLE fee_budget_policy (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    site_id TEXT NOT NULL CHECK (length(site_id) > 0),
    version INTEGER NOT NULL CHECK (version BETWEEN 1 AND 9007199254740991),
    hourly_limits_enabled INTEGER NOT NULL CHECK (hourly_limits_enabled IN (0, 1)),
    session_hourly_limit_micro_usd INTEGER NOT NULL CHECK (session_hourly_limit_micro_usd BETWEEN 0 AND 9007199254740991),
    network_multiplier INTEGER NOT NULL CHECK (network_multiplier BETWEEN 1 AND 100),
    daily_qa_limit_micro_usd INTEGER NOT NULL CHECK (daily_qa_limit_micro_usd BETWEEN 0 AND 9007199254740991),
    data_epoch TEXT NOT NULL CHECK (length(data_epoch) > 0),
    updated_at TEXT NOT NULL
);

-- 仅保存操作引用、策略版本及日额度变化标记；不复制金额、凭据或请求正文。
CREATE TABLE fee_policy_audit (
    site_id TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    policy_version INTEGER NOT NULL CHECK (policy_version BETWEEN 1 AND 9007199254740991),
    actor_id TEXT NOT NULL,
    root_operation_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    daily_limit_changed INTEGER NOT NULL CHECK (daily_limit_changed IN (0, 1)),
    PRIMARY KEY (site_id, data_epoch, policy_version)
);
