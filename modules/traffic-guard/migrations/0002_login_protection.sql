-- 只保存账号/网络HMAC及安全操作引用；login/bootstrap共享作用域。
CREATE TABLE login_attempts (
    attempt_id TEXT PRIMARY KEY,
    data_epoch TEXT NOT NULL,
    caller TEXT NOT NULL CHECK (caller = 'identity'),
    request_id TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    account_key TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('login', 'bootstrap')),
    created_at_ns INTEGER NOT NULL,
    expires_at_ns INTEGER NOT NULL CHECK (expires_at_ns > created_at_ns),
    creation_revision INTEGER NOT NULL CHECK (creation_revision = 1),
    outcome TEXT NOT NULL CHECK (outcome IN ('pending', 'authenticated', 'rejected', 'aborted')),
    finished_at_ns INTEGER,
    UNIQUE (data_epoch, caller, request_id),
    UNIQUE (data_epoch, attempt_id),
    CHECK ((outcome = 'pending' AND finished_at_ns IS NULL) OR
           (outcome <> 'pending' AND finished_at_ns IS NOT NULL))
);
CREATE INDEX login_account_pending ON login_attempts (data_epoch, account_key, outcome, expires_at_ns);

CREATE TABLE login_attempt_networks (
    data_epoch TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    network_key TEXT NOT NULL,
    PRIMARY KEY (data_epoch, attempt_id, network_key),
    FOREIGN KEY (data_epoch, attempt_id) REFERENCES login_attempts (data_epoch, attempt_id)
);
CREATE INDEX login_network_pending ON login_attempt_networks (data_epoch, network_key, attempt_id);

-- 每次rejected对账号和每个网络键各写一行；查询按服务时间滚动15分钟。
CREATE TABLE login_failures (
    data_epoch TEXT NOT NULL,
    scope_kind TEXT NOT NULL CHECK (scope_kind IN ('account', 'network')),
    scope_key TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    failed_at_ns INTEGER NOT NULL,
    PRIMARY KEY (data_epoch, scope_kind, scope_key, attempt_id),
    FOREIGN KEY (data_epoch, attempt_id) REFERENCES login_attempts (data_epoch, attempt_id)
);
CREATE INDEX login_failure_window ON login_failures (data_epoch, scope_kind, scope_key, failed_at_ns);

CREATE TABLE login_locks (
    data_epoch TEXT NOT NULL,
    scope_kind TEXT NOT NULL CHECK (scope_kind IN ('account', 'network')),
    scope_key TEXT NOT NULL,
    locked_until_ns INTEGER NOT NULL,
    PRIMARY KEY (data_epoch, scope_kind, scope_key)
);

-- Finish与Begin为不同operation命名空间；换key仍由原attempt终局去重。
CREATE TABLE login_finish_receipts (
    data_epoch TEXT NOT NULL,
    caller TEXT NOT NULL CHECK (caller = 'identity'),
    operation_id TEXT NOT NULL,
    attempt_id TEXT NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('authenticated', 'rejected', 'aborted')),
    created_at_ns INTEGER NOT NULL,
    PRIMARY KEY (data_epoch, caller, operation_id),
    FOREIGN KEY (data_epoch, attempt_id) REFERENCES login_attempts (data_epoch, attempt_id)
);
