-- identity 域初始权威表（docs/data.md：私有 accounts/credential/session/
-- permission/receipt 业务库）。nw_outbox/nw_inbox 两表由 libs/platform
-- EnsureSchema 创建（outbox 工作包接入时），不写入本域迁移。
-- 账号名创建后不可修改；删除账户保留墓碑行供署名与审计。
CREATE TABLE accounts (
    account_id           TEXT PRIMARY KEY,
    site_id              TEXT NOT NULL,
    account_name         TEXT NOT NULL UNIQUE,
    is_superadmin        INTEGER NOT NULL CHECK (is_superadmin IN (0, 1)),
    must_change_password INTEGER NOT NULL CHECK (must_change_password IN (0, 1)),
    status               TEXT NOT NULL CHECK (status IN ('active', 'disabled', 'deleted')),
    revision             INTEGER NOT NULL,
    permission_revision  INTEGER NOT NULL,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);

-- 全站唯一超管由部分唯一索引保证。
CREATE UNIQUE INDEX idx_accounts_single_superadmin
    ON accounts (is_superadmin) WHERE is_superadmin = 1;

CREATE TABLE credentials (
    account_id          TEXT PRIMARY KEY REFERENCES accounts (account_id),
    password_hash       TEXT NOT NULL,
    credential_revision INTEGER NOT NULL
);

-- 已确认10项功能勾选的权威记录；枚举由本域函数按 FunctionPermission 校验。
CREATE TABLE account_permissions (
    account_id TEXT NOT NULL REFERENCES accounts (account_id),
    permission TEXT NOT NULL,
    PRIMARY KEY (account_id, permission)
);

CREATE TABLE sessions (
    session_id                  TEXT PRIMARY KEY,
    account_id                  TEXT NOT NULL REFERENCES accounts (account_id),
    site_id                     TEXT NOT NULL,
    credential_revision         INTEGER NOT NULL,
    permission_revision         INTEGER NOT NULL,
    issued_at                   TEXT NOT NULL,
    expires_at                  TEXT NOT NULL,
    absolute_expires_at         TEXT NOT NULL,
    device_label                TEXT NOT NULL DEFAULT '',
    restricted_to_password_change INTEGER NOT NULL CHECK (restricted_to_password_change IN (0, 1)),
    revoked_at                  TEXT,
    revision                    INTEGER NOT NULL
);

CREATE INDEX idx_sessions_account ON sessions (account_id);
CREATE INDEX idx_sessions_open ON sessions (account_id) WHERE revoked_at IS NULL;

CREATE TABLE preferences (
    account_id    TEXT PRIMARY KEY REFERENCES accounts (account_id),
    theme         TEXT NOT NULL,
    locale        TEXT NOT NULL,
    default_route TEXT NOT NULL,
    revision      INTEGER NOT NULL
);

-- 幂等回执：site/actor/operation 唯一；敏感输入只存salted digest。
CREATE TABLE operation_receipts (
    operation_id  TEXT NOT NULL,
    site_id       TEXT NOT NULL,
    actor_id      TEXT NOT NULL,
    input_digest  TEXT NOT NULL,
    result_hash   TEXT NOT NULL,
    state         TEXT NOT NULL DEFAULT 'committed',
    created_at    TEXT NOT NULL,
    PRIMARY KEY (site_id, actor_id, operation_id)
);

-- 迁移机制自身的已应用记录；首个迁移自举本表。
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
